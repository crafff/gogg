// Package rankings serves champion ranking aggregates. Two query
// modes — overall (champions ranked across all positions, each row
// carrying every position it plays above PositionThreshold %) and
// by-position (a single team_position filter, each row playing that
// one role) — both feed the same JSON shape on the wire.
//
// Business logic lives here, not in the SQL layer:
//
//   - tier_group strings ("master_plus") expand to lists of
//     matches.avg_tier values ([MASTER, GRANDMASTER, CHALLENGER]).
//   - "latest" version filter is resolved to the actual game version
//     via the catalog service before the SQL ever runs.
//   - Filter normalisation (trim, uppercase, clamp).
package rankings

import (
	"context"
	"fmt"
	"strings"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

// ChampionRanking is the per-champion ranking row. Mirrors the legacy
// ChampionRankingItem field-for-field so /api/v1/rankings/champions
// serialises byte-equal to /api/rankings/champions.
type ChampionRanking struct {
	ChampionID   int      `json:"championId"`
	ChampionName string   `json:"championName"`
	TeamPosition []string `json:"teamPosition"`
	Games        int      `json:"games"`
	Wins         int      `json:"wins"`
	Losses       int      `json:"losses"`
	WinRate      float64  `json:"winRate"`
	PickRate     float64  `json:"pickRate"`
	BanRate      float64  `json:"banRate"`
	KDA          float64  `json:"kda"`
}

// Filter carries every query knob the caller can flip. The transport
// layer is responsible for parsing+clamping query params into Filter;
// the service does no further input scrubbing.
type Filter struct {
	QueueID           int
	Version           string // exact match; "" = all
	Region            string // exact match; "" = all
	TierGroup         string // "master_plus", "challenger", … "" = all
	MinGames          int
	Limit             int // -1 = unlimited
	PositionThreshold float64
	Position          string // "" = call GetOverall; non-empty = call GetByPosition
}

// Result bundles the rows with the cross-row totalMatches the legacy
// response embeds in `meta`. Returning these in one shot avoids two
// trips through the connection pool.
type Result struct {
	Items        []ChampionRanking
	TotalMatches int
	// ResolvedVersion is the version actually queried, after
	// resolving Filter.Version="latest" to a concrete value. Empty
	// when the caller didn't ask for "latest". The transport layer
	// echoes this back in the response meta so the client knows what
	// it actually got.
	ResolvedVersion string
}

// Querier is the narrow sqlc-bound surface this service needs.
// Implemented by sqlcgen.Queries via duck-typing; tests use a
// hand-rolled fake.
type Querier interface {
	ListOverallRankings(ctx context.Context, arg sqlcgen.ListOverallRankingsParams) ([]sqlcgen.ListOverallRankingsRow, error)
	ListRankingsByPosition(ctx context.Context, arg sqlcgen.ListRankingsByPositionParams) ([]sqlcgen.ListRankingsByPositionRow, error)
}

// VersionResolver resolves the "latest" magic value into the concrete
// version stored in game_versions. Implemented by the catalog service
// in practice, but kept as a small interface so this package doesn't
// import catalog directly.
type VersionResolver interface {
	GetLatestVersion(ctx context.Context) (string, error)
}

// Service is the rankings use case. Construct with New().
type Service struct {
	q        Querier
	versions VersionResolver
}

// New returns a Service bound to the given dependencies.
func New(q Querier, versions VersionResolver) *Service {
	return &Service{q: q, versions: versions}
}

// GetOverall returns champion rankings aggregated across every
// position. Each row's TeamPosition lists the positions the champion
// played for at least Filter.PositionThreshold % of their games.
func (s *Service) GetOverall(ctx context.Context, f Filter) (Result, error) {
	version, err := s.resolveVersion(ctx, f.Version)
	if err != nil {
		return Result{}, err
	}
	rows, err := s.q.ListOverallRankings(ctx, sqlcgen.ListOverallRankingsParams{
		QueueID:           int32(f.QueueID),
		VersionFilter:     version,
		RegionFilter:      strings.ToUpper(f.Region),
		AvgTiers:          tierGroupToAvgTiers(f.TierGroup),
		PositionThreshold: f.PositionThreshold,
		MinGames:          int32(f.MinGames),
		RowLimit:          int32(f.Limit),
	})
	if err != nil {
		return Result{}, fmt.Errorf("list overall rankings: %w", err)
	}
	items := make([]ChampionRanking, 0, len(rows))
	var total int
	for _, r := range rows {
		items = append(items, ChampionRanking{
			ChampionID:   int(r.ChampionID),
			ChampionName: r.ChampionName,
			TeamPosition: r.TeamPosition,
			Games:        int(r.Games),
			Wins:         int(r.Wins),
			Losses:       int(r.Losses),
			WinRate:      r.WinRate,
			PickRate:     r.PickRate,
			BanRate:      r.BanRate,
			KDA:          r.Kda,
		})
		total = int(r.TotalMatches)
	}
	return Result{Items: items, TotalMatches: total, ResolvedVersion: maybeResolved(f.Version, version)}, nil
}

// GetByPosition returns champion rankings for a single position.
// Filter.Position must be non-empty; transport callers should route
// to this instead of GetOverall when the user picked a position
// filter.
func (s *Service) GetByPosition(ctx context.Context, f Filter) (Result, error) {
	if strings.TrimSpace(f.Position) == "" {
		return Result{}, fmt.Errorf("rankings: position is required for GetByPosition")
	}
	version, err := s.resolveVersion(ctx, f.Version)
	if err != nil {
		return Result{}, err
	}
	rows, err := s.q.ListRankingsByPosition(ctx, sqlcgen.ListRankingsByPositionParams{
		QueueID:        int32(f.QueueID),
		VersionFilter:  version,
		RegionFilter:   strings.ToUpper(f.Region),
		AvgTiers:       tierGroupToAvgTiers(f.TierGroup),
		PositionFilter: strings.ToUpper(f.Position),
		MinGames:       int32(f.MinGames),
		RowLimit:       int32(f.Limit),
	})
	if err != nil {
		return Result{}, fmt.Errorf("list by-position rankings: %w", err)
	}
	pos := strings.ToUpper(f.Position)
	items := make([]ChampionRanking, 0, len(rows))
	var total int
	for _, r := range rows {
		items = append(items, ChampionRanking{
			ChampionID:   int(r.ChampionID),
			ChampionName: r.ChampionName,
			TeamPosition: []string{pos},
			Games:        int(r.Games),
			Wins:         int(r.Wins),
			Losses:       int(r.Losses),
			WinRate:      r.WinRate,
			PickRate:     r.PickRate,
			BanRate:      r.BanRate,
			KDA:          r.Kda,
		})
		total = int(r.TotalMatches)
	}
	return Result{Items: items, TotalMatches: total, ResolvedVersion: maybeResolved(f.Version, version)}, nil
}

// resolveVersion turns "latest" into a concrete version via the
// catalog service. Any other value (including "") is passed through.
func (s *Service) resolveVersion(ctx context.Context, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "latest" {
		return requested, nil
	}
	v, err := s.versions.GetLatestVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve latest version: %w", err)
	}
	return v, nil
}

// maybeResolved returns the resolved value only when the caller asked
// for "latest"; for any other input ("" or an explicit version) it
// returns "" because there was nothing to resolve. Transport echoes
// this into the response meta so /api/v1 callers can see what
// "latest" mapped to without making a second call.
func maybeResolved(requested, resolved string) string {
	if strings.TrimSpace(requested) == "latest" {
		return resolved
	}
	return ""
}

// tierGroupToAvgTiers expands a tier_group string into the avg_tier
// values it covers. Mirrors legacy tierGroupToAvgTiers exactly so
// /api/v1 returns identical rows for any given tier filter.
//
//	challenger        → [CHALLENGER]
//	grandmaster_plus  → [GRANDMASTER, CHALLENGER]
//	grandmaster       → [GRANDMASTER]
//	master_plus       → [MASTER, GRANDMASTER, CHALLENGER]
//	master            → [MASTER]
//	""                → [] (no filter)
func tierGroupToAvgTiers(tg string) []string {
	switch tg {
	case "challenger":
		return []string{"CHALLENGER"}
	case "grandmaster_plus":
		return []string{"GRANDMASTER", "CHALLENGER"}
	case "grandmaster":
		return []string{"GRANDMASTER"}
	case "master_plus":
		return []string{"MASTER", "GRANDMASTER", "CHALLENGER"}
	case "master":
		return []string{"MASTER"}
	default:
		return []string{}
	}
}
