package tft

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type Querier interface {
	ListTFTAnalysisCatalog(context.Context) ([]sqlcgen.ListTFTAnalysisCatalogRow, error)
	ResolveLatestTFTAnalysisPatch(context.Context, sqlcgen.ResolveLatestTFTAnalysisPatchParams) (string, error)
	GetTFTLineupPublication(context.Context, sqlcgen.GetTFTLineupPublicationParams) (sqlcgen.TftLineupPublication, error)
	ListTFTLineupFamilies(context.Context, sqlcgen.ListTFTLineupFamiliesParams) ([]sqlcgen.ListTFTLineupFamiliesRow, error)
	ListTFTLocalizedStaticObjects(context.Context, []string, string, string) ([]sqlcgen.ListTFTLocalizedStaticObjectsRow, error)
}

type Service struct {
	q       FullQuerier
	limiter FixedWindowLimiter
	starter WorkflowStarter
	cfg     RuntimeConfig
	now     func() time.Time
}

func New(q FullQuerier, limiter FixedWindowLimiter, starter WorkflowStarter, cfg RuntimeConfig) *Service {
	if cfg.Freshness <= 0 {
		cfg.Freshness = 10 * time.Minute
	}
	if cfg.IPLimit <= 0 {
		cfg.IPLimit = 5
	}
	if cfg.IPWindow <= 0 {
		cfg.IPWindow = 10 * time.Minute
	}
	return &Service{q: q, limiter: limiter, starter: starter, cfg: cfg, now: time.Now}
}

type Filter struct {
	Platform, Patch, Cohort, Window, Locale string
	SetNumber, MinSamples, Limit            int
}

type CatalogEntry struct {
	Platform, Patch, Cohort, Window   string
	SetNumber, QueueID                int
	PublishedAt                       time.Time
	SourceMatches, SourceParticipants int64
	Coverage                          Coverage
}

type Coverage struct {
	WindowStart, WindowEnd time.Time
	ExactLineups           int
	FamilyThreshold        int
}

type Entity struct{ ID, Name, IconURL string }
type Count struct {
	Entity Entity
	Count  int
	Rate   float64
}
type Metrics struct {
	SampleSize, LobbyCount                                     int64
	PickRate, AvgPlacement, FirstRate, Top4Rate, ContestedRate float64
}
type Lineup struct {
	ID                                        string
	CoreUnits                                 []Entity
	CommonItems, CommonAugments, CommonTraits []Count
	Metrics                                   Metrics
}
type Result struct {
	Platform, Patch, Cohort, Window, Locale, AlgorithmVersion string
	SetNumber, QueueID                                        int
	PublishedAt                                               time.Time
	SourceMatches, SourceParticipants                         int64
	Coverage                                                  Coverage
	Items                                                     []Lineup
}

func (s *Service) Catalog(ctx context.Context) ([]CatalogEntry, error) {
	rows, err := s.q.ListTFTAnalysisCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("list TFT analysis catalog: %w", err)
	}
	out := make([]CatalogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, CatalogEntry{Platform: row.Platform, Patch: row.Patch, SetNumber: int(row.SetNumber), QueueID: int(row.QueueID), Cohort: row.Cohort, Window: row.WindowKind, PublishedAt: row.PublishedAt.Time, SourceMatches: row.SourceMatches, SourceParticipants: row.SourceParticipants, Coverage: parseCoverage(row.Coverage)})
	}
	return out, nil
}

func (s *Service) Lineups(ctx context.Context, f Filter) (Result, error) {
	f = Normalize(f)
	if err := Validate(f); err != nil {
		return Result{}, err
	}
	if f.Patch == "latest" {
		patch, err := s.q.ResolveLatestTFTAnalysisPatch(ctx, sqlcgen.ResolveLatestTFTAnalysisPatchParams{Platform: f.Platform, SetNumber: int32(f.SetNumber), QueueID: 1100, Cohort: f.Cohort, WindowKind: f.Window})
		if err != nil {
			return Result{}, fmt.Errorf("resolve latest TFT patch: %w", err)
		}
		f.Patch = patch
	}
	params := sqlcgen.GetTFTLineupPublicationParams{Platform: f.Platform, Patch: f.Patch, SetNumber: int32(f.SetNumber), QueueID: 1100, Cohort: f.Cohort, WindowKind: f.Window}
	publication, err := s.q.GetTFTLineupPublication(ctx, params)
	if err != nil {
		return Result{}, fmt.Errorf("get TFT lineup publication: %w", err)
	}
	rows, err := s.q.ListTFTLineupFamilies(ctx, sqlcgen.ListTFTLineupFamiliesParams{Platform: f.Platform, Patch: f.Patch, SetNumber: int32(f.SetNumber), QueueID: 1100, Cohort: f.Cohort, WindowKind: f.Window, MinSamples: int64(f.MinSamples), RowLimit: int32(f.Limit)})
	if err != nil {
		return Result{}, fmt.Errorf("list TFT lineups: %w", err)
	}
	localized, err := s.q.ListTFTLocalizedStaticObjects(ctx, []string{"unit", "item", "augment", "trait"}, f.Patch, f.Locale)
	if err != nil {
		return Result{}, fmt.Errorf("localize TFT lineups: %w", err)
	}
	entities := indexLocalizedEntities(localized)
	out := Result{Platform: f.Platform, Patch: f.Patch, SetNumber: f.SetNumber, QueueID: 1100, Cohort: f.Cohort, Window: f.Window, Locale: f.Locale, AlgorithmVersion: publication.AlgorithmVersion, PublishedAt: publication.PublishedAt.Time, SourceMatches: publication.SourceMatches, SourceParticipants: publication.SourceParticipants, Coverage: parseCoverage(publication.Coverage), Items: make([]Lineup, 0, len(rows))}
	for _, row := range rows {
		lineup := Lineup{ID: row.FamilyID, Metrics: Metrics{SampleSize: row.SampleSize, LobbyCount: row.LobbyCount, PickRate: row.PickRate, AvgPlacement: row.AvgPlacement, FirstRate: row.FirstRate, Top4Rate: row.Top4Rate, ContestedRate: row.ContestedRate}}
		for _, id := range row.CoreUnits {
			lineup.CoreUnits = append(lineup.CoreUnits, localizedEntity(entities, id))
		}
		lineup.CommonItems = parseCounts(row.CommonItems, entities)
		lineup.CommonAugments = parseCounts(row.CommonAugments, entities)
		lineup.CommonTraits = parseCounts(row.CommonTraits, entities)
		out.Items = append(out.Items, lineup)
	}
	return out, nil
}

func Normalize(f Filter) Filter {
	f.Platform = strings.ToUpper(strings.TrimSpace(f.Platform))
	f.Patch = strings.TrimSpace(f.Patch)
	if f.Patch == "" {
		f.Patch = "latest"
	}
	if f.Cohort == "" {
		f.Cohort = "MASTER_PLUS"
	}
	if f.Window == "" {
		f.Window = "PATCH"
	}
	f.Locale = strings.ToLower(f.Locale)
	if f.Locale == "" {
		f.Locale = "en_us"
	}
	if f.MinSamples == 0 {
		f.MinSamples = 200
	}
	if f.Limit == 0 {
		f.Limit = 50
	}
	return f
}

func Validate(f Filter) error {
	supported := map[string]bool{"NA1": true, "BR1": true, "LA1": true, "LA2": true, "KR": true, "JP1": true, "EUN1": true, "EUW1": true, "TR1": true, "ME1": true, "RU": true, "OC1": true, "SG2": true, "TW2": true, "VN2": true}
	if !supported[f.Platform] {
		return fmt.Errorf("unsupported TFT platform %q", f.Platform)
	}
	if f.SetNumber <= 0 {
		return fmt.Errorf("setNumber must be positive")
	}
	if f.Cohort != "MASTER_PLUS" && f.Cohort != "DIAMOND" {
		return fmt.Errorf("invalid TFT cohort %q", f.Cohort)
	}
	if f.Window != "THREE_DAYS" && f.Window != "PATCH" {
		return fmt.Errorf("invalid TFT window %q", f.Window)
	}
	if f.Locale != "en_us" && f.Locale != "zh_cn" {
		return fmt.Errorf("locale must be en_us or zh_cn")
	}
	if f.MinSamples < 1 || f.MinSamples > 100000 {
		return fmt.Errorf("minSamples must be 1..100000")
	}
	if f.Limit < 1 || f.Limit > 100 {
		return fmt.Errorf("limit must be 1..100")
	}
	return nil
}

type rawCount struct {
	ID    string  `json:"id"`
	Count int     `json:"count"`
	Rate  float64 `json:"rate"`
}

func parseCoverage(body []byte) Coverage {
	var raw struct {
		WindowStart     time.Time `json:"window_start"`
		WindowEnd       time.Time `json:"window_end"`
		ExactLineups    int       `json:"exact_lineups"`
		FamilyThreshold int       `json:"family_threshold"`
	}
	if json.Unmarshal(body, &raw) != nil {
		return Coverage{}
	}
	return Coverage{WindowStart: raw.WindowStart, WindowEnd: raw.WindowEnd, ExactLineups: raw.ExactLineups, FamilyThreshold: raw.FamilyThreshold}
}

func parseCounts(body []byte, entities map[string]Entity) []Count {
	var raw []rawCount
	if json.Unmarshal(body, &raw) != nil {
		return []Count{}
	}
	out := make([]Count, 0, len(raw))
	for _, item := range raw {
		out = append(out, Count{Entity: localizedEntity(entities, item.ID), Count: item.Count, Rate: item.Rate})
	}
	return out
}
