package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const AlgorithmVersion = "greedy-jaccard-0.60-core-0.60-v1"

var ErrInsufficientCoverage = errors.New("insufficient TFT analysis coverage")

type Filter struct {
	Platform, Patch, Cohort, WindowKind string
	SetNumber, QueueID                  int
	WindowStart, WindowEnd              time.Time
	MinFamilySamples                    int
}

type Observation struct {
	MatchID, Puuid, Signature      string
	Placement                      int
	Units, Items, Augments, Traits []string
}

type Exact struct {
	Signature                                    string
	Units                                        []string
	Sample, PlacementSum, First, Top4, Contested int
	Lobbies                                      map[string]bool
	LobbyCounts                                  map[string]int
	Items, Augments, Traits                      map[string]int
}

type Family struct {
	ID, Representative                           string
	Members                                      []*Exact
	CoreUnits                                    []string
	Sample, PlacementSum, First, Top4, Contested int
	Lobbies                                      map[string]bool
	Items, Augments, Traits                      map[string]int
}

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Publish(ctx context.Context, filter Filter) (int64, int, error) {
	if filter.MinFamilySamples <= 0 {
		filter.MinFamilySamples = 200
	}
	q := sqlcgen.New(s.pool)
	setNumber := int32(filter.SetNumber)
	rows, err := q.ListTFTLineupObservations(ctx, sqlcgen.ListTFTLineupObservationsParams{
		Platform: filter.Platform, Patch: filter.Patch, SetNumber: &setNumber, QueueID: int32(filter.QueueID), Cohort: filter.Cohort,
		WindowStart: timestamp(filter.WindowStart), WindowEnd: timestamp(filter.WindowEnd),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("list TFT lineup observations: %w", err)
	}
	observations := make([]Observation, 0, len(rows))
	matches := map[string]bool{}
	for _, row := range rows {
		if row.LineupSignature == nil {
			continue
		}
		observations = append(observations, Observation{MatchID: row.MatchID, Puuid: row.Puuid, Signature: *row.LineupSignature, Placement: int(row.Placement), Units: row.UnitIds, Items: row.ItemIds, Augments: row.AugmentIds, Traits: row.TraitIds})
		matches[row.MatchID] = true
	}
	exacts := BuildExact(observations)
	families := Cluster(exacts)
	hasPublishableFamily := false
	for _, family := range families {
		if family.Sample >= filter.MinFamilySamples {
			hasPublishableFamily = true
			break
		}
	}
	if len(observations) < filter.MinFamilySamples || !hasPublishableFamily {
		return int64(len(observations)), 0, ErrInsufficientCoverage
	}
	coverage, _ := json.Marshal(map[string]any{"window_start": filter.WindowStart.UTC(), "window_end": filter.WindowEnd.UTC(), "exact_lineups": len(exacts), "family_threshold": filter.MinFamilySamples})
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	txq := sqlcgen.New(tx)
	slice := sqlcgen.DeleteTFTExactRollupSliceParams{Platform: filter.Platform, Patch: filter.Patch, SetNumber: int32(filter.SetNumber), QueueID: int32(filter.QueueID), Cohort: filter.Cohort, WindowKind: filter.WindowKind}
	if err := txq.DeleteTFTExactRollupSlice(ctx, slice); err != nil {
		return 0, 0, err
	}
	for _, exact := range exacts {
		items, _ := json.Marshal(exact.Items)
		augments, _ := json.Marshal(exact.Augments)
		traits, _ := json.Marshal(exact.Traits)
		if err := txq.UpsertTFTExactRollup(ctx, sqlcgen.UpsertTFTExactRollupParams{
			Platform: filter.Platform, Patch: filter.Patch, SetNumber: int32(filter.SetNumber), QueueID: int32(filter.QueueID), Cohort: filter.Cohort, WindowKind: filter.WindowKind,
			Signature: exact.Signature, SampleSize: int64(exact.Sample), LobbyCount: int64(len(exact.Lobbies)), PlacementSum: int64(exact.PlacementSum),
			FirstCount: int64(exact.First), Top4Count: int64(exact.Top4), ContestedCount: int64(exact.Contested), UnitIds: exact.Units,
			ItemCounts: items, AugmentCounts: augments, TraitCounts: traits,
		}); err != nil {
			return 0, 0, err
		}
	}
	publication, err := txq.CreateTFTLineupPublication(ctx, sqlcgen.CreateTFTLineupPublicationParams{
		Platform: filter.Platform, Patch: filter.Patch, SetNumber: int32(filter.SetNumber), QueueID: int32(filter.QueueID), Cohort: filter.Cohort,
		WindowKind: filter.WindowKind, AlgorithmVersion: AlgorithmVersion, SourceMatches: int64(len(matches)), SourceParticipants: int64(len(observations)), Coverage: coverage,
	})
	if err != nil {
		return 0, 0, err
	}
	if err := txq.DeleteTFTLineupPublicationFamilies(ctx, publication.ID); err != nil {
		return 0, 0, err
	}
	publishedFamilies := 0
	for _, family := range families {
		if family.Sample < filter.MinFamilySamples {
			continue
		}
		items, _ := json.Marshal(topCounts(family.Items, family.Sample, 20))
		augments, _ := json.Marshal(topCounts(family.Augments, family.Sample, 20))
		traits, _ := json.Marshal(topCounts(family.Traits, family.Sample, 20))
		if err := txq.InsertTFTLineupFamily(ctx, sqlcgen.InsertTFTLineupFamilyParams{
			PublicationID: publication.ID, FamilyID: family.ID, SampleSize: int64(family.Sample), LobbyCount: int64(len(family.Lobbies)),
			PickRate: rate(family.Sample, len(observations)), AvgPlacement: float64(family.PlacementSum) / float64(family.Sample),
			FirstRate: rate(family.First, family.Sample), Top4Rate: rate(family.Top4, family.Sample), ContestedRate: rate(family.Contested, family.Sample),
			CoreUnits: family.CoreUnits, CommonItems: items, CommonAugments: augments, CommonTraits: traits,
		}); err != nil {
			return 0, 0, err
		}
		for _, member := range family.Members {
			if err := txq.InsertTFTLineupFamilyMember(ctx, sqlcgen.InsertTFTLineupFamilyMemberParams{PublicationID: publication.ID, FamilyID: family.ID, Signature: member.Signature, SampleSize: int64(member.Sample), Jaccard: jaccard(member.Units, family.Members[0].Units)}); err != nil {
				return 0, 0, err
			}
		}
		publishedFamilies++
	}
	if err := txq.PublishTFTLineupPublication(ctx, publication.ID); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return int64(len(observations)), publishedFamilies, nil
}

func BuildExact(observations []Observation) []*Exact {
	bySignature := map[string]*Exact{}
	lobbySignatureCounts := map[string]int{}
	for _, observation := range observations {
		lobbySignatureCounts[observation.MatchID+"\x00"+observation.Signature]++
	}
	for _, observation := range observations {
		exact := bySignature[observation.Signature]
		if exact == nil {
			exact = &Exact{Signature: observation.Signature, Units: append([]string(nil), observation.Units...), Lobbies: map[string]bool{}, LobbyCounts: map[string]int{}, Items: map[string]int{}, Augments: map[string]int{}, Traits: map[string]int{}}
			bySignature[observation.Signature] = exact
		}
		exact.Sample++
		exact.PlacementSum += observation.Placement
		exact.Lobbies[observation.MatchID] = true
		exact.LobbyCounts[observation.MatchID]++
		if observation.Placement == 1 {
			exact.First++
		}
		if observation.Placement <= 4 {
			exact.Top4++
		}
		if lobbySignatureCounts[observation.MatchID+"\x00"+observation.Signature] > 1 {
			exact.Contested++
		}
		addCounts(exact.Items, observation.Items)
		addCounts(exact.Augments, observation.Augments)
		addCounts(exact.Traits, observation.Traits)
	}
	out := make([]*Exact, 0, len(bySignature))
	for _, exact := range bySignature {
		out = append(out, exact)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sample != out[j].Sample {
			return out[i].Sample > out[j].Sample
		}
		return out[i].Signature < out[j].Signature
	})
	return out
}

func Cluster(exacts []*Exact) []*Family {
	families := []*Family{}
	for _, exact := range exacts {
		best := -1
		bestScore := 0.0
		for i, family := range families {
			shared := sharedCount(exact.Units, family.Members[0].Units)
			score := jaccard(exact.Units, family.Members[0].Units)
			if shared >= 4 && score >= 0.60 && score > bestScore {
				best, bestScore = i, score
			}
		}
		if best < 0 {
			sum := sha256.Sum256([]byte(exact.Signature))
			families = append(families, &Family{ID: hex.EncodeToString(sum[:8]), Representative: exact.Signature, Lobbies: map[string]bool{}, Items: map[string]int{}, Augments: map[string]int{}, Traits: map[string]int{}})
			best = len(families) - 1
		}
		family := families[best]
		family.Members = append(family.Members, exact)
		family.Sample += exact.Sample
		family.PlacementSum += exact.PlacementSum
		family.First += exact.First
		family.Top4 += exact.Top4
		for lobby := range exact.Lobbies {
			family.Lobbies[lobby] = true
		}
		mergeCounts(family.Items, exact.Items)
		mergeCounts(family.Augments, exact.Augments)
		mergeCounts(family.Traits, exact.Traits)
	}
	for _, family := range families {
		unitWeight := map[string]int{}
		lobbyFamily := map[string]int{}
		for _, member := range family.Members {
			for _, unit := range unique(member.Units) {
				unitWeight[unit] += member.Sample
			}
			for lobby, count := range member.LobbyCounts {
				lobbyFamily[lobby] += count
			}
		}
		for unit, count := range unitWeight {
			if float64(count)/float64(family.Sample) >= 0.60 {
				family.CoreUnits = append(family.CoreUnits, unit)
			}
		}
		sort.Strings(family.CoreUnits)
		for _, count := range lobbyFamily {
			if count > 1 {
				family.Contested += count
			}
		}
	}
	return families
}

type CountStat struct {
	ID    string  `json:"id"`
	Count int     `json:"count"`
	Rate  float64 `json:"rate"`
}

func topCounts(counts map[string]int, total, limit int) []CountStat {
	out := make([]CountStat, 0, len(counts))
	for id, count := range counts {
		out = append(out, CountStat{ID: id, Count: count, Rate: rate(count, total)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
func addCounts(target map[string]int, values []string) {
	for _, value := range values {
		if value != "" {
			target[value]++
		}
	}
}
func mergeCounts(target, source map[string]int) {
	for key, value := range source {
		target[key] += value
	}
}
func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func sharedCount(a, b []string) int {
	set := map[string]bool{}
	for _, v := range unique(a) {
		set[v] = true
	}
	n := 0
	for _, v := range unique(b) {
		if set[v] {
			n++
		}
	}
	return n
}
func jaccard(a, b []string) float64 {
	shared := sharedCount(a, b)
	union := len(unique(a)) + len(unique(b)) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}
func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
