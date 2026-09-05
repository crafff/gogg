// Package championinsights serves revisioned, cohort-specific early-game
// statistics. It keeps observational evidence separate from champion build
// choices and from any future personalized recommendation workflow.
package championinsights

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type Filter struct {
	QueueID   int
	Version   string
	Region    string
	TierGroup string
	Position  string
}

type Bucket struct {
	Ordinal              int
	LowerBound           float64
	UpperBound           float64
	Games                int
	Wins                 int
	SamplePlayers        int
	ObservedWinRate      float64
	ObservedWinRateDelta float64
}

type Factor struct {
	MetricKey     string
	Kind          string
	StartMinute   int
	EndMinute     int
	Unit          string
	P50           float64
	P70           float64
	P90           float64
	EvidenceGrade string
	DisplayOrder  int
	Buckets       []Bucket
}

type Result struct {
	ChampionID        int
	ChampionName      string
	Position          string
	ResolvedVersion   string
	RegionScope       string
	TierGroup         string
	Revision          string
	Algorithm         string
	DataThrough       *time.Time
	PublishedAt       *time.Time
	Availability      string
	UnavailableReason *string
	CohortScope       string
	SampleGames       int
	SamplePlayers     int
	Factors           []Factor
}

type Querier interface {
	GetChampionIdentity(context.Context, int32) (sqlcgen.GetChampionIdentityRow, error)
	GetPublishedChampionInsightIdentity(context.Context, int32) (sqlcgen.GetPublishedChampionInsightIdentityRow, error)
	GetLatestChampionInsightVersion(context.Context) (string, error)
	GetPublishedChampionInsightCohort(context.Context, sqlcgen.GetPublishedChampionInsightCohortParams) (sqlcgen.GetPublishedChampionInsightCohortRow, error)
	ListPublishedChampionInsightFactors(context.Context, int64) ([]sqlcgen.ListPublishedChampionInsightFactorsRow, error)
}

type Service struct{ q Querier }

func New(q Querier) *Service { return &Service{q: q} }

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Message) }

func (s *Service) Get(ctx context.Context, championID int, filter Filter) (*Result, error) {
	if championID <= 0 {
		return nil, &ValidationError{Field: "id", Message: "must be positive"}
	}
	if filter.QueueID == 0 {
		filter.QueueID = 420
	}
	if filter.QueueID != 420 {
		return nil, &ValidationError{Field: "queueId", Message: "must be 420"}
	}
	position := strings.ToUpper(strings.TrimSpace(filter.Position))
	if position == "" {
		return nil, &ValidationError{Field: "position", Message: "is required"}
	}
	if !validPosition(position) {
		return nil, &ValidationError{Field: "position", Message: "is not supported"}
	}
	region, err := normalizeRegion(filter.Region)
	if err != nil {
		return nil, err
	}
	tier, err := normalizeTierGroup(filter.TierGroup)
	if err != nil {
		return nil, err
	}
	identity, err := s.getIdentity(ctx, int32(championID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get champion identity: %w", err)
	}
	version := strings.TrimSpace(filter.Version)
	if version == "" || version == "latest" {
		version, err = s.q.GetLatestChampionInsightVersion(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return unavailable(identity, position, version, region, tier, "NOT_PUBLISHED"), nil
		}
		if err != nil {
			return nil, fmt.Errorf("resolve latest champion insight version: %w", err)
		}
	} else if !validPatchVersion(version) {
		return nil, &ValidationError{Field: "version", Message: "must use major.minor format"}
	}
	if position != "JUNGLE" {
		return unavailable(identity, position, version, region, tier, "UNSUPPORTED_POSITION"), nil
	}
	cohort, err := s.q.GetPublishedChampionInsightCohort(ctx, sqlcgen.GetPublishedChampionInsightCohortParams{
		ChampionID: int32(championID), QueueID: int32(filter.QueueID), Version: version,
		RegionScope: region, TierGroup: tier, TeamPosition: position,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return unavailable(identity, position, version, region, tier, "NOT_PUBLISHED"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get published champion insight cohort: %w", err)
	}
	result := &Result{
		ChampionID: int(cohort.ChampionID), ChampionName: cohort.ChampionName,
		Position: cohort.TeamPosition, ResolvedVersion: version, RegionScope: region,
		TierGroup: tier, Revision: cohort.Revision, Algorithm: cohort.AlgorithmVersion,
		Availability: cohort.Availability, UnavailableReason: cohort.ReasonCode,
		CohortScope: cohort.CohortScope, SampleGames: int(cohort.SampleGames),
		SamplePlayers: int(cohort.SamplePlayers), Factors: []Factor{},
	}
	if cohort.DataThrough.Valid {
		value := cohort.DataThrough.Time
		result.DataThrough = &value
	}
	if cohort.PublishedAt.Valid {
		value := cohort.PublishedAt.Time
		result.PublishedAt = &value
	}
	if cohort.Availability != "AVAILABLE" {
		return result, nil
	}
	rows, err := s.q.ListPublishedChampionInsightFactors(ctx, cohort.CohortID)
	if err != nil {
		return nil, fmt.Errorf("list published champion insight factors: %w", err)
	}
	result.Factors = assembleFactors(rows)
	if len(result.Factors) == 0 {
		reason := "SPARSE_BUCKETS"
		result.Availability = "INSUFFICIENT_SAMPLE"
		result.UnavailableReason = &reason
	}
	return result, nil
}

func assembleFactors(rows []sqlcgen.ListPublishedChampionInsightFactorsRow) []Factor {
	out := make([]Factor, 0, 4)
	for _, row := range rows {
		if len(out) == 0 || out[len(out)-1].MetricKey != row.MetricKey {
			out = append(out, Factor{
				MetricKey: row.MetricKey, Kind: row.Kind, StartMinute: int(row.StartMinute),
				EndMinute: int(row.EndMinute), Unit: row.Unit,
				P50: row.P50, P70: row.P70, P90: row.P90, EvidenceGrade: row.EvidenceGrade,
				DisplayOrder: int(row.DisplayOrder), Buckets: []Bucket{},
			})
		}
		factor := &out[len(out)-1]
		factor.Buckets = append(factor.Buckets, Bucket{
			Ordinal: int(row.Ordinal), LowerBound: row.LowerBound, UpperBound: row.UpperBound,
			Games: int(row.Games), Wins: int(row.Wins), SamplePlayers: int(row.SamplePlayers),
		})
	}
	for factorIndex := range out {
		factor := &out[factorIndex]
		var totalGames, totalWins int
		for _, bucket := range factor.Buckets {
			totalGames += bucket.Games
			totalWins += bucket.Wins
		}
		baseline := rate(totalWins, totalGames)
		for bucketIndex := range factor.Buckets {
			bucket := &factor.Buckets[bucketIndex]
			bucket.ObservedWinRate = rate(bucket.Wins, bucket.Games)
			bucket.ObservedWinRateDelta = bucket.ObservedWinRate - baseline
		}
	}
	return out
}

type championIdentity struct {
	championID   int32
	championName string
}

func (s *Service) getIdentity(ctx context.Context, championID int32) (championIdentity, error) {
	published, err := s.q.GetPublishedChampionInsightIdentity(ctx, championID)
	if err == nil {
		return championIdentity{championID: published.ChampionID, championName: published.ChampionName}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return championIdentity{}, fmt.Errorf("get published champion insight identity: %w", err)
	}
	fallback, err := s.q.GetChampionIdentity(ctx, championID)
	if err != nil {
		return championIdentity{}, err
	}
	return championIdentity{championID: fallback.ChampionID, championName: fallback.ChampionName}, nil
}

func unavailable(identity championIdentity, position, version, region, tier, reason string) *Result {
	return &Result{
		ChampionID: int(identity.championID), ChampionName: identity.championName,
		Position: position, ResolvedVersion: version, RegionScope: region, TierGroup: tier,
		Availability: "UNAVAILABLE", UnavailableReason: &reason, Factors: []Factor{},
	}
}

func normalizeRegion(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "ALL", nil
	}
	if value != "KR" && value != "NA1" {
		return "", &ValidationError{Field: "region", Message: "must be KR or NA1"}
	}
	return value, nil
}

func normalizeTierGroup(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "ALL", nil
	}
	switch value {
	case "ALL", "MASTER", "MASTER_PLUS", "GRANDMASTER", "GRANDMASTER_PLUS", "CHALLENGER":
		return value, nil
	default:
		return "", &ValidationError{Field: "tierGroup", Message: "is not supported"}
	}
}

func validPosition(value string) bool {
	switch value {
	case "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY":
		return true
	}
	return false
}

func validPatchVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || strconv.Itoa(n) != part {
			return false
		}
	}
	return true
}

func rate(wins, games int) float64 {
	if games == 0 {
		return 0
	}
	return float64(wins) * 100 / float64(games)
}
