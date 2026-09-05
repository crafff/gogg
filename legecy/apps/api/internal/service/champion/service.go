package champion

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	"github.com/jackc/pgx/v5"
)

type Filter struct {
	QueueID   int
	Version   string
	Region    string
	TierGroup string
	Position  string
}

type Build struct {
	IDs           []int
	Games         int
	EligibleGames int
	PickRate      float64
	WinRate       float64
}

type RuneBuild struct {
	PrimaryStyleID   int
	SecondaryStyleID int
	PerkIDs          []int
	StatShardIDs     []int
	Build
}

type ItemStage struct {
	Stage  int
	Builds []Build
}

type Result struct {
	ChampionID          int
	ChampionName        string
	Games               int
	ResolvedVersion     string
	RuneBuilds          []RuneBuild
	SummonerSpellBuilds []Build
	StarterBuilds       []Build
	BootsBuilds         []Build
	ItemBuilds          []ItemStage
}

type Querier interface {
	GetChampionIdentity(context.Context, int32) (sqlcgen.GetChampionIdentityRow, error)
	ListChampionDetailBuilds(context.Context, sqlcgen.ListChampionDetailBuildsParams) ([]sqlcgen.ListChampionDetailBuildsRow, error)
}

type VersionResolver interface {
	GetLatestVersion(context.Context) (string, error)
}

type Service struct {
	q        Querier
	versions VersionResolver
}

func New(q Querier, versions VersionResolver) *Service { return &Service{q: q, versions: versions} }

func (s *Service) Get(ctx context.Context, championID int, f Filter) (*Result, error) {
	if championID <= 0 {
		return nil, fmt.Errorf("champion id must be positive")
	}
	if f.QueueID == 0 {
		f.QueueID = 420
	}
	if f.QueueID != 420 {
		return nil, fmt.Errorf("queue id must be 420")
	}
	f.Position = strings.ToUpper(strings.TrimSpace(f.Position))
	if f.Position != "" && !validPosition(f.Position) {
		return nil, fmt.Errorf("invalid position %q", f.Position)
	}
	identity, err := s.q.GetChampionIdentity(ctx, int32(championID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get champion identity: %w", err)
	}
	version := strings.TrimSpace(f.Version)
	resolved := ""
	if version == "latest" {
		version, err = s.versions.GetLatestVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve latest version: %w", err)
		}
		resolved = version
	}
	rows, err := s.q.ListChampionDetailBuilds(ctx, sqlcgen.ListChampionDetailBuildsParams{
		ChampionID: int32(championID), QueueID: int32(f.QueueID), VersionFilter: version,
		RegionFilter: strings.ToUpper(strings.TrimSpace(f.Region)), AvgTiers: tierGroup(f.TierGroup),
		PositionFilter: f.Position, RowLimit: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("list champion detail builds: %w", err)
	}
	out := &Result{ChampionID: int(identity.ChampionID), ChampionName: identity.ChampionName, ResolvedVersion: resolved,
		RuneBuilds: []RuneBuild{}, SummonerSpellBuilds: []Build{}, StarterBuilds: []Build{}, BootsBuilds: []Build{},
		ItemBuilds: []ItemStage{{Stage: 3, Builds: []Build{}}, {Stage: 4, Builds: []Build{}}, {Stage: 5, Builds: []Build{}}, {Stage: 6, Builds: []Build{}}}}
	for _, row := range rows {
		if int(row.EligibleGames) > out.Games {
			out.Games = int(row.EligibleGames)
		}
		b := build(row)
		switch row.Category {
		case "RUNES":
			if len(b.IDs) == 11 {
				out.RuneBuilds = append(out.RuneBuilds, RuneBuild{PrimaryStyleID: b.IDs[0], SecondaryStyleID: b.IDs[1], PerkIDs: append([]int(nil), b.IDs[2:8]...), StatShardIDs: append([]int(nil), b.IDs[8:11]...), Build: b})
			}
		case "SPELLS":
			out.SummonerSpellBuilds = append(out.SummonerSpellBuilds, b)
		case "STARTER":
			out.StarterBuilds = append(out.StarterBuilds, b)
		case "BOOTS":
			out.BootsBuilds = append(out.BootsBuilds, b)
		case "ITEMS":
			for i := range out.ItemBuilds {
				if out.ItemBuilds[i].Stage == int(row.Stage) {
					out.ItemBuilds[i].Builds = append(out.ItemBuilds[i].Builds, b)
				}
			}
		}
	}
	return out, nil
}

func build(row sqlcgen.ListChampionDetailBuildsRow) Build {
	ids := make([]int, len(row.Signature))
	for i, id := range row.Signature {
		ids[i] = int(id)
	}
	pick, win := 0.0, 0.0
	if row.EligibleGames > 0 {
		pick = float64(row.Games) * 100 / float64(row.EligibleGames)
	}
	if row.Games > 0 {
		win = float64(row.Wins) * 100 / float64(row.Games)
	}
	return Build{IDs: ids, Games: int(row.Games), EligibleGames: int(row.EligibleGames), PickRate: pick, WinRate: win}
}

func validPosition(v string) bool {
	switch v {
	case "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY":
		return true
	}
	return false
}
func tierGroup(v string) []string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "challenger":
		return []string{"CHALLENGER"}
	case "grandmaster":
		return []string{"GRANDMASTER"}
	case "grandmaster_plus":
		return []string{"GRANDMASTER", "CHALLENGER"}
	case "master":
		return []string{"MASTER"}
	case "master_plus":
		return []string{"MASTER", "GRANDMASTER", "CHALLENGER"}
	default:
		return []string{}
	}
}
