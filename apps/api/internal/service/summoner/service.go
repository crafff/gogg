package summoner

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 20
	MaxHistoryItems = 100
)

var supportedQueueIDs = map[QueueFilter][]int32{
	QueueAll:        {400, 420, 440, 480},
	QueueDraft:      {400},
	QueueRankedSolo: {420},
	QueueRankedFlex: {440},
	QueueSwiftplay:  {480},
}

type QueueFilter string

const (
	QueueAll        QueueFilter = "ALL"
	QueueDraft      QueueFilter = "DRAFT_PICK"
	QueueRankedSolo QueueFilter = "RANKED_SOLO"
	QueueRankedFlex QueueFilter = "RANKED_FLEX"
	QueueSwiftplay  QueueFilter = "SWIFTPLAY"
)

type Identity struct {
	Region   string
	GameName string
	TagLine  string
}

type PageRequest struct {
	Queue QueueFilter
	First int
	After string
}

type Profile struct {
	Region          string
	GameName        string
	TagLine         string
	ProfileIconID   int
	SummonerLevel   int64
	LastRefreshedAt *time.Time
	IsStale         bool
}

type Rank struct {
	QueueType    string
	Tier         string
	Division     string
	LeaguePoints int
	Wins         int
	Losses       int
	WinRate      float64
}

type Match struct {
	MatchID           string
	Queue             QueueFilter
	QueueID           int
	GameStartTime     time.Time
	DurationSeconds   int
	Version           string
	EndOfGameResult   string
	Position          string
	Win               bool
	ChampionID        int
	ChampionName      string
	ChampionLevel     int
	Kills             int
	Deaths            int
	Assists           int
	KDA               float64
	MinionsKilled     int
	CSPerMinute       float64
	GoldEarned        int
	DamageToChampions int
	VisionScore       int
	ItemIDs           []int
	SummonerSpellIDs  []int
	PrimaryStyleID    int
	SecondaryStyleID  int
	PerkIDs           []int
	StatShardIDs      []int
	EarlySurrender    bool
	Surrender         bool
	AverageTier       *string
	AverageDivision   *string
	TierCoverage      int
	Participants      []Participant
}

type Participant struct {
	ParticipantID     int
	TeamID            int
	IsCurrentPlayer   bool
	GameName          string
	TagLine           string
	Position          string
	Win               bool
	ChampionID        int
	ChampionName      string
	ChampionLevel     int
	Kills             int
	Deaths            int
	Assists           int
	KDA               float64
	MinionsKilled     int
	GoldEarned        int
	DamageToChampions int
	VisionScore       int
	ItemIDs           []int
	SummonerSpellIDs  []int
	PrimaryStyleID    int
	SecondaryStyleID  int
	PerkIDs           []int
	Rank              *ParticipantRank
}

type ParticipantRank struct {
	Tier               string
	Division           *string
	LeaguePoints       *int
	SnapshotDeltaHours *int
}

type PageInfo struct {
	EndCursor   *string
	HasNextPage bool
	Returned    int
}

type Result struct {
	Profile  Profile
	Ranks    []Rank
	Matches  []Match
	PageInfo PageInfo
}

type Job struct {
	ID             string
	Region         string
	GameName       string
	TagLine        string
	Status         string
	Stage          string
	ScannedCount   int
	SupportedCount int
	FetchedCount   int
	FailedCount    int
	ErrorCode      *string
	ErrorMessage   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

type RefreshResult struct {
	Fresh  bool
	Reused bool
	Job    *Job
}

type FixedWindowLimiter interface {
	AllowFixedWindows(context.Context, []string, []int, []time.Duration) (int, time.Duration, error)
}

type WorkflowInput struct {
	JobID    string `json:"job_id"`
	Region   string `json:"region"`
	GameName string `json:"game_name"`
	TagLine  string `json:"tag_line"`
}

type WorkflowStarter interface {
	StartSummonerWorkflow(context.Context, string, string, WorkflowInput) error
}

type Config struct {
	Freshness        time.Duration
	IPLimit          int
	IPWindow         time.Duration
	RegionTaskQueues map[string]string
}

type Service struct {
	queries *sqlcgen.Queries
	limiter FixedWindowLimiter
	starter WorkflowStarter
	cfg     Config
	now     func() time.Time
}

func New(queries *sqlcgen.Queries, limiter FixedWindowLimiter, starter WorkflowStarter, cfg Config) *Service {
	return &Service{queries: queries, limiter: limiter, starter: starter, cfg: cfg, now: time.Now}
}

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Message) }

type RateLimitScope string

const (
	RateLimitIdentity RateLimitScope = "IDENTITY"
	RateLimitIP       RateLimitScope = "IP"
)

type RateLimitError struct {
	RetryAfter time.Duration
	Scope      RateLimitScope
}

func (e *RateLimitError) Error() string { return "summoner refresh rate limited" }

var ErrRefreshUnavailable = errors.New("summoner refresh unavailable")

func (s *Service) Get(ctx context.Context, identity Identity, page PageRequest) (*Result, error) {
	identity, err := validateIdentity(identity)
	if err != nil {
		return nil, err
	}
	if page.Queue == "" {
		page.Queue = QueueAll
	}
	queueIDs, ok := supportedQueueIDs[page.Queue]
	if !ok {
		return nil, &ValidationError{Field: "queue", Message: "unsupported queue filter"}
	}
	if page.First == 0 {
		page.First = DefaultPageSize
	}
	if page.First < 1 || page.First > MaxPageSize {
		return nil, &ValidationError{Field: "first", Message: "must be between 1 and 20"}
	}

	profileRow, err := s.queries.GetSummonerByIdentity(ctx, identity.Region, identity.GameName, identity.TagLine)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	cursor, err := decodeCursor(page.After)
	if err != nil {
		return nil, &ValidationError{Field: "after", Message: "invalid cursor"}
	}
	ranks, err := s.queries.ListSummonerCurrentRanks(ctx, identity.Region, profileRow.Puuid)
	if err != nil {
		return nil, err
	}
	if cursor.Seen >= MaxHistoryItems {
		return &Result{Profile: s.mapProfile(profileRow), Ranks: mapRanks(ranks), Matches: []Match{}, PageInfo: PageInfo{Returned: cursor.Seen}}, nil
	}
	limit := min(page.First, MaxHistoryItems-cursor.Seen)
	rows, err := s.queries.ListSummonerMatches(ctx, sqlcgen.ListSummonerMatchesParams{
		Puuid: profileRow.Puuid, Region: identity.Region, QueueIds: queueIDs,
		BeforeTime: cursor.pgTime(), BeforeMatchID: cursor.MatchID, RowLimit: int32(limit + 1),
	})
	if err != nil {
		return nil, err
	}
	hasNext := len(rows) > limit && cursor.Seen+limit < MaxHistoryItems
	if len(rows) > limit {
		rows = rows[:limit]
	}
	matches := make([]Match, 0, len(rows))
	matchIndexes := make(map[string]int, len(rows))
	matchIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		matchIndexes[row.MatchID] = len(matches)
		matchIDs = append(matchIDs, row.MatchID)
		matches = append(matches, mapMatch(row))
	}
	if len(matchIDs) > 0 {
		participantRows, err := s.queries.ListSummonerMatchParticipants(ctx, matchIDs)
		if err != nil {
			return nil, err
		}
		for _, row := range participantRows {
			if index, ok := matchIndexes[row.MatchID]; ok {
				matches[index].Participants = append(matches[index].Participants, mapParticipant(row, profileRow.Puuid))
			}
		}
	}
	seen := cursor.Seen + len(matches)
	var endCursor *string
	if len(rows) > 0 && seen < MaxHistoryItems {
		encoded := encodeCursor(historyCursor{Time: rows[len(rows)-1].GameStartTs.Time, MatchID: rows[len(rows)-1].MatchID, Seen: seen})
		endCursor = &encoded
	}
	return &Result{
		Profile: s.mapProfile(profileRow), Ranks: mapRanks(ranks), Matches: matches,
		PageInfo: PageInfo{EndCursor: endCursor, HasNextPage: hasNext, Returned: seen},
	}, nil
}

func (s *Service) Refresh(ctx context.Context, identity Identity, clientIP string) (RefreshResult, error) {
	identity, err := validateIdentity(identity)
	if err != nil {
		return RefreshResult{}, err
	}
	if row, err := s.queries.GetSummonerByIdentity(ctx, identity.Region, identity.GameName, identity.TagLine); err == nil {
		if row.MatchesRefreshedAt.Valid && s.now().Sub(row.MatchesRefreshedAt.Time) <= s.cfg.Freshness {
			return RefreshResult{Fresh: true}, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RefreshResult{}, err
	}

	normName, normTag := strings.ToLower(identity.GameName), strings.ToUpper(identity.TagLine)
	if row, err := s.queries.FindActiveSummonerLookupJob(ctx, identity.Region, normName, normTag); err == nil {
		job := mapJob(row)
		// A process can stop after committing the job but before starting the
		// workflow. ExecuteWorkflow is idempotent by workflow ID, so retry queued
		// jobs whenever another request encounters one.
		if row.Status == "QUEUED" {
			if s.starter == nil {
				return RefreshResult{}, ErrRefreshUnavailable
			}
			taskQueue := s.cfg.RegionTaskQueues[identity.Region]
			if taskQueue == "" {
				return RefreshResult{}, ErrRefreshUnavailable
			}
			if err := s.starter.StartSummonerWorkflow(ctx, "summoner-lookup-"+row.ID, taskQueue, WorkflowInput{
				JobID: row.ID, Region: identity.Region, GameName: row.RequestedGameName, TagLine: row.RequestedTagLine,
			}); err != nil {
				return RefreshResult{}, ErrRefreshUnavailable
			}
		}
		return RefreshResult{Reused: true, Job: &job}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RefreshResult{}, err
	}
	// A refresh can fail quickly (for example when the selected platform does
	// not contain the globally resolved Riot account). Repeated navigation or a
	// browser reload during the same refresh window should reconnect to that
	// terminal result instead of consuming the limiter and returning an
	// unrelated RATE_LIMITED error.
	if row, err := s.queries.FindLatestSummonerLookupJob(ctx, identity.Region, normName, normTag); err == nil {
		if withinRefreshWindow(s.now(), row.CreatedAt.Time, s.cfg.Freshness) {
			job := mapJob(row)
			return RefreshResult{Reused: true, Job: &job}, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RefreshResult{}, err
	}
	if s.limiter == nil || s.starter == nil {
		return RefreshResult{}, ErrRefreshUnavailable
	}

	identityKey := "summoner-refresh:identity:" + hashKey(identity.Region+"\x00"+normName+"\x00"+normTag)
	ipKey := "summoner-refresh:ip:" + hashKey(clientIP)
	deniedIndex, retryAfter, err := s.limiter.AllowFixedWindows(
		ctx,
		[]string{identityKey, ipKey},
		[]int{1, s.cfg.IPLimit},
		[]time.Duration{s.cfg.Freshness, s.cfg.IPWindow},
	)
	if err != nil {
		return RefreshResult{}, ErrRefreshUnavailable
	}
	if deniedIndex >= 0 {
		scope := RateLimitIdentity
		if deniedIndex == 1 {
			scope = RateLimitIP
		}
		return RefreshResult{}, &RateLimitError{RetryAfter: retryAfter, Scope: scope}
	}

	jobID := uuid.NewString()
	row, err := s.queries.CreateSummonerLookupJob(ctx, sqlcgen.CreateSummonerLookupJobParams{
		ID: jobID, Region: identity.Region, GameName: identity.GameName, TagLine: identity.TagLine,
		GameNameNorm: normName, TagLineNorm: normTag,
	})
	if err != nil {
		if active, findErr := s.queries.FindActiveSummonerLookupJob(ctx, identity.Region, normName, normTag); findErr == nil {
			job := mapJob(active)
			return RefreshResult{Reused: true, Job: &job}, nil
		}
		return RefreshResult{}, err
	}
	taskQueue := s.cfg.RegionTaskQueues[identity.Region]
	if taskQueue == "" {
		return RefreshResult{}, ErrRefreshUnavailable
	}
	if err := s.starter.StartSummonerWorkflow(ctx, "summoner-lookup-"+jobID, taskQueue, WorkflowInput{
		JobID: jobID, Region: identity.Region, GameName: identity.GameName, TagLine: identity.TagLine,
	}); err != nil {
		code, message := "WORKFLOW_START_FAILED", "refresh worker is unavailable"
		_ = s.queries.FinishSummonerLookupJob(ctx, sqlcgen.FinishSummonerLookupJobParams{
			Status: "FAILED", ErrorCode: &code, ErrorMessage: &message, ID: jobID,
		})
		return RefreshResult{}, ErrRefreshUnavailable
	}
	job := mapJob(row)
	return RefreshResult{Job: &job}, nil
}

func withinRefreshWindow(now, createdAt time.Time, window time.Duration) bool {
	age := now.Sub(createdAt)
	return age >= 0 && age <= window
}

func (s *Service) GetJob(ctx context.Context, id string) (*Job, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, &ValidationError{Field: "id", Message: "invalid lookup job id"}
	}
	row, err := s.queries.GetSummonerLookupJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job := mapJob(row)
	return &job, nil
}

func validateIdentity(in Identity) (Identity, error) {
	in.Region = strings.ToUpper(strings.TrimSpace(in.Region))
	in.GameName = strings.TrimSpace(in.GameName)
	in.TagLine = strings.TrimSpace(in.TagLine)
	if in.Region != "KR" && in.Region != "NA1" {
		return Identity{}, &ValidationError{Field: "region", Message: "must be KR or NA1"}
	}
	if len([]rune(in.GameName)) < 1 || len([]rune(in.GameName)) > 32 {
		return Identity{}, &ValidationError{Field: "gameName", Message: "must contain 1 to 32 characters"}
	}
	if len([]rune(in.TagLine)) < 1 || len([]rune(in.TagLine)) > 16 {
		return Identity{}, &ValidationError{Field: "tagLine", Message: "must contain 1 to 16 characters"}
	}
	return in, nil
}

func (s *Service) mapProfile(row sqlcgen.GetSummonerByIdentityRow) Profile {
	var refreshed *time.Time
	if row.MatchesRefreshedAt.Valid {
		t := row.MatchesRefreshedAt.Time
		refreshed = &t
	}
	return Profile{
		Region: row.Region, GameName: deref(row.GameName), TagLine: deref(row.TagLine),
		ProfileIconID: int(deref(row.ProfileIconID)), SummonerLevel: deref(row.SummonerLevel),
		LastRefreshedAt: refreshed,
		IsStale:         refreshed == nil || s.now().Sub(*refreshed) > s.cfg.Freshness,
	}
}

func mapRanks(rows []sqlcgen.ListSummonerCurrentRanksRow) []Rank {
	out := make([]Rank, 0, len(rows))
	for _, row := range rows {
		games := int(row.Wins + row.Losses)
		winRate := 0.0
		if games > 0 {
			winRate = float64(row.Wins) * 100 / float64(games)
		}
		out = append(out, Rank{QueueType: row.QueueType, Tier: row.Tier, Division: deref(row.Division),
			LeaguePoints: int(row.LeaguePoints), Wins: int(row.Wins), Losses: int(row.Losses), WinRate: winRate})
	}
	return out
}

func mapMatch(row sqlcgen.ListSummonerMatchesRow) Match {
	duration := int(deref(row.GameDuration))
	cs := int(deref(row.TotalMinionsKilled) + deref(row.NeutralMinionsKilled))
	minutes := float64(duration) / 60
	csPerMinute := 0.0
	if minutes > 0 {
		csPerMinute = float64(cs) / minutes
	}
	kills, deaths, assists := int(deref(row.Kills)), int(deref(row.Deaths)), int(deref(row.Assists))
	return Match{
		MatchID: row.MatchID, Queue: queueFromID(int(row.QueueID)), QueueID: int(row.QueueID),
		GameStartTime: row.GameStartTs.Time, DurationSeconds: duration, Version: row.Version,
		EndOfGameResult: deref(row.EndOfGameResult), Position: firstNonEmpty(deref(row.TeamPosition), deref(row.IndividualPosition)),
		Win: deref(row.Win), ChampionID: int(deref(row.ChampionID)), ChampionName: deref(row.ChampionName),
		ChampionLevel: int(deref(row.ChampLevel)), Kills: kills, Deaths: deaths, Assists: assists,
		KDA: calculateKDA(kills, deaths, assists), MinionsKilled: cs, CSPerMinute: math.Round(csPerMinute*10) / 10,
		GoldEarned: int(deref(row.GoldEarned)), DamageToChampions: int(deref(row.TotalDamageDealtToChampions)), VisionScore: int(deref(row.VisionScore)),
		ItemIDs:          []int{int(deref(row.Item0)), int(deref(row.Item1)), int(deref(row.Item2)), int(deref(row.Item3)), int(deref(row.Item4)), int(deref(row.Item5)), int(deref(row.Item6))},
		SummonerSpellIDs: []int{int(deref(row.Summoner1ID)), int(deref(row.Summoner2ID))},
		PrimaryStyleID:   int(deref(row.Style0)), SecondaryStyleID: int(deref(row.Style1)),
		PerkIDs:        []int{int(deref(row.Perk0)), int(deref(row.Perk1)), int(deref(row.Perk2)), int(deref(row.Perk3)), int(deref(row.Perk4)), int(deref(row.Perk5))},
		StatShardIDs:   []int{int(deref(row.StatOffense)), int(deref(row.StatFlex)), int(deref(row.StatDefense))},
		EarlySurrender: deref(row.GameEndedInEarlySurrender), Surrender: deref(row.GameEndedInSurrender),
		AverageTier: nonEmptyStringPointer(row.AvgTier), AverageDivision: nonEmptyStringPointer(row.AvgDivision),
		TierCoverage: int(deref(row.TierCoverage)),
		Participants: []Participant{},
	}
}

func mapParticipant(row sqlcgen.ListSummonerMatchParticipantsRow, currentPUUID string) Participant {
	kills, deaths, assists := int(deref(row.Kills)), int(deref(row.Deaths)), int(deref(row.Assists))
	var rank *ParticipantRank
	if tier := nonEmptyStringPointer(row.TierAtMatch); tier != nil {
		rank = &ParticipantRank{
			Tier: *tier, Division: nonEmptyStringPointer(row.DivisionAtMatch),
			LeaguePoints: int32Pointer(row.LpAtMatch), SnapshotDeltaHours: int32Pointer(row.TierSnapshotDeltaH),
		}
	}
	return Participant{
		ParticipantID: int(deref(row.ParticipantID)), TeamID: int(deref(row.TeamID)),
		IsCurrentPlayer: row.Puuid != nil && *row.Puuid == currentPUUID,
		GameName:        deref(row.GameName), TagLine: deref(row.TagLine),
		Position: firstNonEmpty(deref(row.TeamPosition), deref(row.IndividualPosition)), Win: deref(row.Win),
		ChampionID: int(deref(row.ChampionID)), ChampionName: deref(row.ChampionName), ChampionLevel: int(deref(row.ChampLevel)),
		Kills: kills, Deaths: deaths, Assists: assists, KDA: calculateKDA(kills, deaths, assists),
		MinionsKilled: int(deref(row.TotalMinionsKilled) + deref(row.NeutralMinionsKilled)),
		GoldEarned:    int(deref(row.GoldEarned)), DamageToChampions: int(deref(row.TotalDamageDealtToChampions)), VisionScore: int(deref(row.VisionScore)),
		ItemIDs:          []int{int(deref(row.Item0)), int(deref(row.Item1)), int(deref(row.Item2)), int(deref(row.Item3)), int(deref(row.Item4)), int(deref(row.Item5)), int(deref(row.Item6))},
		SummonerSpellIDs: []int{int(deref(row.Summoner1ID)), int(deref(row.Summoner2ID))},
		PrimaryStyleID:   int(deref(row.Style0)), SecondaryStyleID: int(deref(row.Style1)),
		PerkIDs: []int{int(deref(row.Perk0)), int(deref(row.Perk1)), int(deref(row.Perk2)), int(deref(row.Perk3)), int(deref(row.Perk4)), int(deref(row.Perk5))},
		Rank:    rank,
	}
}

func calculateKDA(kills, deaths, assists int) float64 {
	kda := float64(kills + assists)
	if deaths > 0 {
		kda /= float64(deaths)
	}
	return math.Round(kda*100) / 100
}

func queueFromID(id int) QueueFilter {
	switch id {
	case 400:
		return QueueDraft
	case 420:
		return QueueRankedSolo
	case 440:
		return QueueRankedFlex
	case 480:
		return QueueSwiftplay
	default:
		return QueueAll
	}
}

type historyCursor struct {
	Time    time.Time `json:"t"`
	MatchID string    `json:"m"`
	Seen    int       `json:"s"`
}

func (c historyCursor) pgTime() pgtype.Timestamptz {
	if c.Time.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: c.Time, Valid: true}
}

func encodeCursor(c historyCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(raw string) (historyCursor, error) {
	if raw == "" {
		return historyCursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return historyCursor{}, err
	}
	var c historyCursor
	if err := json.Unmarshal(b, &c); err != nil || c.Time.IsZero() || c.MatchID == "" || c.Seen < 0 || c.Seen > MaxHistoryItems {
		return historyCursor{}, errors.New("invalid cursor")
	}
	return c, nil
}

func mapJob(row sqlcgen.SummonerLookupJob) Job {
	job := Job{
		ID: row.ID, Region: row.Region, GameName: row.RequestedGameName, TagLine: row.RequestedTagLine,
		Status: row.Status, Stage: row.Stage, ScannedCount: int(row.ScannedCount), SupportedCount: int(row.SupportedCount),
		FetchedCount: int(row.FetchedCount), FailedCount: int(row.FailedCount), ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		job.CompletedAt = &t
	}
	return job
}

func hashKey(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nonEmptyStringPointer(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	copy := *value
	return &copy
}

func int32Pointer(value *int32) *int {
	if value == nil {
		return nil
	}
	copy := int(*value)
	return &copy
}

func deref[T any](v *T) (zero T) {
	if v != nil {
		return *v
	}
	return zero
}
