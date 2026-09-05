package tft

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const (
	DefaultHistoryPageSize = 20
	MaxHistoryPageSize     = 20
	MaxHistoryItems        = 100
)

type FullQuerier interface {
	Querier
	GetLatestCompletedTFTObservedRun(context.Context, string) (sqlcgen.TftCrawlRun, error)
	GetTFTRunByID(context.Context, int64) (sqlcgen.TftCrawlRun, error)
	GetTFTObservedLineupPreview(context.Context, int64, string) (sqlcgen.GetTFTObservedLineupPreviewRow, error)
	GetTFTObservedLineupDetails(context.Context, sqlcgen.GetTFTObservedLineupDetailsParams) (interface{}, error)
	ListLatestTFTLocalizedStaticObjects(context.Context, int64, []string, string) ([]sqlcgen.ListLatestTFTLocalizedStaticObjectsRow, error)
	ListTFTTeamPlannerUnits(context.Context, int64, string) ([]sqlcgen.ListTFTTeamPlannerUnitsRow, error)
	GetTFTPlayerIdentity(context.Context, string, string, string) (sqlcgen.TftPlayerIdentity, error)
	GetTFTPlayerLookupJob(context.Context, string) (sqlcgen.TftPlayerLookupJob, error)
	FindActiveTFTPlayerLookupJob(context.Context, string, string, string) (sqlcgen.TftPlayerLookupJob, error)
	FindLatestTFTPlayerLookupJob(context.Context, string, string, string) (sqlcgen.TftPlayerLookupJob, error)
	CreateTFTPlayerLookupJob(context.Context, sqlcgen.CreateTFTPlayerLookupJobParams) (sqlcgen.TftPlayerLookupJob, error)
	FinishTFTPlayerLookupJob(context.Context, sqlcgen.FinishTFTPlayerLookupJobParams) error
	DeleteExpiredTFTPlayerLookupJobs(context.Context) (int64, error)
	ListTFTPlayerMatches(context.Context, sqlcgen.ListTFTPlayerMatchesParams) ([]sqlcgen.ListTFTPlayerMatchesRow, error)
	ListTFTMatchParticipantsForHistory(context.Context, []string) ([]sqlcgen.ListTFTMatchParticipantsForHistoryRow, error)
	ListTFTMatchAugmentsForHistory(context.Context, []string) ([]sqlcgen.TftMatchAugment, error)
	ListTFTMatchTraitsForHistory(context.Context, []string) ([]sqlcgen.TftMatchTrait, error)
	ListTFTMatchUnitsForHistory(context.Context, []string) ([]sqlcgen.TftMatchUnit, error)
	ListTFTMatchUnitItemsForHistory(context.Context, []string) ([]sqlcgen.TftMatchUnitItem, error)
}

type FixedWindowLimiter interface {
	AllowFixedWindows(context.Context, []string, []int, []time.Duration) (int, time.Duration, error)
}

type RuntimeConfig struct {
	Freshness time.Duration
	IPLimit   int
	IPWindow  time.Duration
}

type WorkflowInput struct {
	JobID, Platform, GameName, TagLine string
}

type WorkflowStarter interface {
	StartTFTPlayerWorkflow(context.Context, string, WorkflowInput) error
}

type Identity struct {
	Platform, GameName, TagLine string
}

type HistoryFilter struct {
	Identity
	After, Locale  string
	First, QueueID int
}

type PlayerProfile struct {
	Platform, GameName, TagLine string
	LastRefreshedAt             *time.Time
	IsStale                     bool
}

type HistoryTrait struct {
	Entity                                  Entity
	NumUnits, Style, TierCurrent, TierTotal int
}

type HistoryUnit struct {
	Entity       Entity
	Rarity, Tier int
	Items        []Entity
}

type HistoryParticipant struct {
	PUUID, GameName, TagLine, CompanionContentID, CompanionSpecies string
	IsCurrentPlayer, Abnormal                                      bool
	Placement, Level, GoldLeft, LastRound, PlayersEliminated       int
	TotalDamageToPlayers, CompanionItemID, CompanionSkinID         int
	TimeEliminatedSeconds                                          float64
	Augments                                                       []Entity
	Traits                                                         []HistoryTrait
	Units                                                          []HistoryUnit
}

type HistoryMatch struct {
	MatchID, Platform, RoutingRegion, GameVersion, Patch       string
	TFTGameType, SetCoreName, EndOfGameResult, ExclusionReason string
	QueueID, MapID, SetNumber, ParticipantCount                int
	GameDatetime                                               time.Time
	GameLengthSeconds                                          float64
	Eligible                                                   bool
	Participant                                                HistoryParticipant
	Participants                                               []HistoryParticipant
}

type PageInfo struct {
	EndCursor   *string
	HasNextPage bool
	Returned    int
}

type HistoryResult struct {
	Profile  PlayerProfile
	Matches  []HistoryMatch
	PageInfo PageInfo
}

type Job struct {
	ID, Platform, GameName, TagLine, Status, Stage string
	ScannedCount, FetchedCount, FailedCount        int
	ErrorCode, ErrorMessage                        *string
	CreatedAt, UpdatedAt                           time.Time
	CompletedAt                                    *time.Time
}

type RefreshResult struct {
	Fresh, Reused bool
	Job           *Job
}

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Message) }

type RateLimitError struct {
	RetryAfter time.Duration
	Scope      string
}

func (e *RateLimitError) Error() string { return "TFT player refresh rate limited" }

var ErrRefreshUnavailable = errors.New("TFT player refresh unavailable")

func (s *Service) History(ctx context.Context, filter HistoryFilter) (*HistoryResult, error) {
	filter, err := normalizeHistoryFilter(filter)
	if err != nil {
		return nil, err
	}
	identity, err := s.q.GetTFTPlayerIdentity(ctx, filter.Platform, filter.GameName, filter.TagLine)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get TFT player identity: %w", err)
	}
	fingerprint := historyFingerprint(filter, identity.Puuid)
	cursor, err := decodeHistoryCursor(filter.After, fingerprint)
	if err != nil {
		return nil, &ValidationError{Field: "after", Message: "invalid cursor"}
	}
	profile := mapPlayerProfile(identity, s.now(), s.cfg.Freshness)
	if cursor.Seen >= MaxHistoryItems {
		return &HistoryResult{Profile: profile, Matches: []HistoryMatch{}, PageInfo: PageInfo{Returned: cursor.Seen}}, nil
	}
	limit := min(filter.First, MaxHistoryItems-cursor.Seen)
	rows, err := s.q.ListTFTPlayerMatches(ctx, sqlcgen.ListTFTPlayerMatchesParams{
		Puuid: identity.Puuid, Platform: filter.Platform, QueueID: int32(filter.QueueID),
		BeforeTime: cursor.pgTime(), BeforeMatchID: cursor.MatchID, RowLimit: int32(limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list TFT player matches: %w", err)
	}
	hasNext := len(rows) > limit && cursor.Seen+limit < MaxHistoryItems
	if len(rows) > limit {
		rows = rows[:limit]
	}
	matches := make([]HistoryMatch, 0, len(rows))
	matchIndex := make(map[string]int, len(rows))
	matchIDs := make([]string, 0, len(rows))
	patches := map[string]bool{}
	for _, row := range rows {
		matchIndex[row.MatchID] = len(matches)
		matchIDs = append(matchIDs, row.MatchID)
		patches[row.Patch] = true
		matches = append(matches, mapHistoryMatch(row))
	}
	if len(matchIDs) > 0 {
		if err := s.assembleHistory(ctx, matches, matchIndex, matchIDs, patches, identity.Puuid, filter.Locale); err != nil {
			return nil, err
		}
	}
	seen := cursor.Seen + len(matches)
	var endCursor *string
	if len(rows) > 0 && seen < MaxHistoryItems {
		value := encodeHistoryCursor(historyCursor{Time: rows[len(rows)-1].GameDatetime.Time, MatchID: rows[len(rows)-1].MatchID, Seen: seen, Fingerprint: fingerprint})
		endCursor = &value
	}
	return &HistoryResult{Profile: profile, Matches: matches, PageInfo: PageInfo{EndCursor: endCursor, HasNextPage: hasNext, Returned: seen}}, nil
}

func (s *Service) assembleHistory(ctx context.Context, matches []HistoryMatch, indexes map[string]int, matchIDs []string, patches map[string]bool, currentPUUID, locale string) error {
	participants, err := s.q.ListTFTMatchParticipantsForHistory(ctx, matchIDs)
	if err != nil {
		return fmt.Errorf("list TFT participants: %w", err)
	}
	augments, err := s.q.ListTFTMatchAugmentsForHistory(ctx, matchIDs)
	if err != nil {
		return fmt.Errorf("list TFT augments: %w", err)
	}
	traits, err := s.q.ListTFTMatchTraitsForHistory(ctx, matchIDs)
	if err != nil {
		return fmt.Errorf("list TFT traits: %w", err)
	}
	units, err := s.q.ListTFTMatchUnitsForHistory(ctx, matchIDs)
	if err != nil {
		return fmt.Errorf("list TFT units: %w", err)
	}
	items, err := s.q.ListTFTMatchUnitItemsForHistory(ctx, matchIDs)
	if err != nil {
		return fmt.Errorf("list TFT unit items: %w", err)
	}
	entitiesByPatch := map[string]map[string]Entity{}
	for patch := range patches {
		rows, loadErr := s.q.ListTFTLocalizedStaticObjects(ctx, []string{"unit", "item", "augment", "trait"}, patch, locale)
		if loadErr != nil && !errors.Is(loadErr, pgx.ErrNoRows) {
			return fmt.Errorf("localize TFT history: %w", loadErr)
		}
		entitiesByPatch[patch] = indexLocalizedEntities(rows)
	}
	entityFor := func(matchIndex int, id string) Entity {
		return localizedEntity(entitiesByPatch[matches[matchIndex].Patch], id)
	}
	type participantKey struct{ MatchID, PUUID string }
	type participantLocation struct{ Match, Participant int }
	locations := map[participantKey]participantLocation{}
	for _, row := range participants {
		index, ok := indexes[row.MatchID]
		if !ok {
			continue
		}
		participant := mapHistoryParticipant(row, row.Puuid == currentPUUID)
		matches[index].Participants = append(matches[index].Participants, participant)
		locations[participantKey{row.MatchID, row.Puuid}] = participantLocation{Match: index, Participant: len(matches[index].Participants) - 1}
	}
	for _, row := range augments {
		if location, ok := locations[participantKey{row.MatchID, row.Puuid}]; ok {
			p := &matches[location.Match].Participants[location.Participant]
			p.Augments = append(p.Augments, entityFor(location.Match, row.AugmentID))
		}
	}
	for _, row := range traits {
		if location, ok := locations[participantKey{row.MatchID, row.Puuid}]; ok {
			p := &matches[location.Match].Participants[location.Participant]
			p.Traits = append(p.Traits, HistoryTrait{Entity: entityFor(location.Match, row.TraitID), NumUnits: int(value16(row.NumUnits)), Style: int(value16(row.Style)), TierCurrent: int(value16(row.TierCurrent)), TierTotal: int(value16(row.TierTotal))})
		}
	}
	type unitKey struct {
		MatchID, PUUID string
		Index          int16
	}
	type unitLocation struct{ Match, Participant, Unit int }
	unitLocations := map[unitKey]unitLocation{}
	for _, row := range units {
		if location, ok := locations[participantKey{row.MatchID, row.Puuid}]; ok {
			p := &matches[location.Match].Participants[location.Participant]
			unit := HistoryUnit{Entity: entityFor(location.Match, row.CharacterID), Rarity: int(value16(row.Rarity)), Tier: int(value16(row.Tier)), Items: []Entity{}}
			if unit.Entity.Name == "" && row.Name != nil {
				unit.Entity.Name = *row.Name
			}
			p.Units = append(p.Units, unit)
			unitLocations[unitKey{row.MatchID, row.Puuid, row.UnitIndex}] = unitLocation{Match: location.Match, Participant: location.Participant, Unit: len(p.Units) - 1}
		}
	}
	for _, row := range items {
		if location, ok := unitLocations[unitKey{row.MatchID, row.Puuid, row.UnitIndex}]; ok {
			unit := &matches[location.Match].Participants[location.Participant].Units[location.Unit]
			unit.Items = append(unit.Items, entityFor(location.Match, row.ItemID))
		}
	}
	for i := range matches {
		for _, participant := range matches[i].Participants {
			if participant.IsCurrentPlayer {
				matches[i].Participant = participant
				break
			}
		}
	}
	return nil
}

func (s *Service) Refresh(ctx context.Context, identity Identity, clientIP string) (RefreshResult, error) {
	identity, err := validateIdentity(identity)
	if err != nil {
		return RefreshResult{}, err
	}
	// Job rows are reconnectable for seven days; prune older terminal rows on
	// refresh traffic so the latest-identity query remains bounded.
	_, _ = s.q.DeleteExpiredTFTPlayerLookupJobs(ctx)
	if row, findErr := s.q.GetTFTPlayerIdentity(ctx, identity.Platform, identity.GameName, identity.TagLine); findErr == nil {
		if row.MatchesRefreshedAt.Valid && s.now().Sub(row.MatchesRefreshedAt.Time) <= s.cfg.Freshness {
			return RefreshResult{Fresh: true}, nil
		}
	} else if !errors.Is(findErr, pgx.ErrNoRows) {
		return RefreshResult{}, findErr
	}
	normName, normTag := strings.ToLower(identity.GameName), strings.ToUpper(identity.TagLine)
	if row, findErr := s.q.FindActiveTFTPlayerLookupJob(ctx, identity.Platform, normName, normTag); findErr == nil {
		job := mapJob(row)
		// Reissuing the stable workflow ID is safe while a run is active and
		// restarts it only if the prior run failed before persisting a terminal
		// job state (Temporal reuse policy is ALLOW_DUPLICATE_FAILED_ONLY).
		if err := s.startWorkflow(ctx, row.ID, identity); err != nil {
			return RefreshResult{}, err
		}
		return RefreshResult{Reused: true, Job: &job}, nil
	} else if !errors.Is(findErr, pgx.ErrNoRows) {
		return RefreshResult{}, findErr
	}
	if row, findErr := s.q.FindLatestTFTPlayerLookupJob(ctx, identity.Platform, normName, normTag); findErr == nil {
		age := s.now().Sub(row.CreatedAt.Time)
		if age >= 0 && age <= s.cfg.Freshness {
			job := mapJob(row)
			return RefreshResult{Reused: true, Job: &job}, nil
		}
	} else if !errors.Is(findErr, pgx.ErrNoRows) {
		return RefreshResult{}, findErr
	}
	if s.limiter == nil || s.starter == nil {
		return RefreshResult{}, ErrRefreshUnavailable
	}
	denied, retryAfter, limitErr := s.limiter.AllowFixedWindows(ctx,
		[]string{"tft-player-refresh:identity:" + hash(identity.Platform+"\x00"+normName+"\x00"+normTag), "tft-player-refresh:ip:" + hash(clientIP)},
		[]int{1, s.cfg.IPLimit}, []time.Duration{s.cfg.Freshness, s.cfg.IPWindow})
	if limitErr != nil {
		return RefreshResult{}, ErrRefreshUnavailable
	}
	if denied >= 0 {
		scope := "IDENTITY"
		if denied == 1 {
			scope = "IP"
		}
		return RefreshResult{}, &RateLimitError{RetryAfter: retryAfter, Scope: scope}
	}
	jobID := uuid.NewString()
	row, err := s.q.CreateTFTPlayerLookupJob(ctx, sqlcgen.CreateTFTPlayerLookupJobParams{ID: jobID, Platform: identity.Platform, GameName: identity.GameName, TagLine: identity.TagLine, GameNameNorm: normName, TagLineNorm: normTag})
	if err != nil {
		if active, findErr := s.q.FindActiveTFTPlayerLookupJob(ctx, identity.Platform, normName, normTag); findErr == nil {
			job := mapJob(active)
			return RefreshResult{Reused: true, Job: &job}, nil
		}
		return RefreshResult{}, err
	}
	if err := s.startWorkflow(ctx, jobID, identity); err != nil {
		code, message := "WORKFLOW_START_FAILED", "TFT refresh worker is unavailable"
		_ = s.q.FinishTFTPlayerLookupJob(ctx, sqlcgen.FinishTFTPlayerLookupJobParams{Status: "FAILED", ErrorCode: &code, ErrorMessage: &message, ID: jobID})
		return RefreshResult{}, err
	}
	job := mapJob(row)
	return RefreshResult{Job: &job}, nil
}

func (s *Service) startWorkflow(ctx context.Context, jobID string, identity Identity) error {
	if s.starter == nil {
		return ErrRefreshUnavailable
	}
	if err := s.starter.StartTFTPlayerWorkflow(ctx, "tft-player-lookup-"+jobID, WorkflowInput{JobID: jobID, Platform: identity.Platform, GameName: identity.GameName, TagLine: identity.TagLine}); err != nil {
		return ErrRefreshUnavailable
	}
	return nil
}

func (s *Service) GetJob(ctx context.Context, id string) (*Job, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, &ValidationError{Field: "id", Message: "invalid lookup job id"}
	}
	row, err := s.q.GetTFTPlayerLookupJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job := mapJob(row)
	return &job, nil
}

func normalizeHistoryFilter(filter HistoryFilter) (HistoryFilter, error) {
	identity, err := validateIdentity(filter.Identity)
	if err != nil {
		return HistoryFilter{}, err
	}
	filter.Identity = identity
	filter.Locale = strings.ToLower(strings.TrimSpace(filter.Locale))
	if filter.Locale == "" {
		filter.Locale = "en_us"
	}
	if filter.Locale != "en_us" && filter.Locale != "zh_cn" {
		return HistoryFilter{}, &ValidationError{Field: "locale", Message: "must be en_us or zh_cn"}
	}
	if filter.First == 0 {
		filter.First = DefaultHistoryPageSize
	}
	if filter.First < 1 || filter.First > MaxHistoryPageSize {
		return HistoryFilter{}, &ValidationError{Field: "first", Message: "must be between 1 and 20"}
	}
	if filter.QueueID < 0 || filter.QueueID > 10000 {
		return HistoryFilter{}, &ValidationError{Field: "queueId", Message: "must be zero or a positive queue id"}
	}
	return filter, nil
}

func validateIdentity(identity Identity) (Identity, error) {
	identity.Platform = strings.ToUpper(strings.TrimSpace(identity.Platform))
	identity.GameName = strings.TrimSpace(identity.GameName)
	identity.TagLine = strings.TrimSpace(identity.TagLine)
	if !supportedPlatforms[identity.Platform] {
		return Identity{}, &ValidationError{Field: "platform", Message: "unsupported TFT platform"}
	}
	if n := len([]rune(identity.GameName)); n < 1 || n > 32 {
		return Identity{}, &ValidationError{Field: "gameName", Message: "must contain 1 to 32 characters"}
	}
	if n := len([]rune(identity.TagLine)); n < 1 || n > 16 {
		return Identity{}, &ValidationError{Field: "tagLine", Message: "must contain 1 to 16 characters"}
	}
	return identity, nil
}

var supportedPlatforms = map[string]bool{"NA1": true, "BR1": true, "LA1": true, "LA2": true, "KR": true, "JP1": true, "EUN1": true, "EUW1": true, "TR1": true, "ME1": true, "RU": true, "OC1": true, "SG2": true, "TW2": true, "VN2": true}

type historyCursor struct {
	Time        time.Time `json:"t"`
	MatchID     string    `json:"m"`
	Seen        int       `json:"s"`
	Fingerprint string    `json:"f"`
}

func (c historyCursor) pgTime() pgtype.Timestamptz {
	if c.Time.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: c.Time, Valid: true}
}
func encodeHistoryCursor(c historyCursor) string {
	body, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(body)
}
func decodeHistoryCursor(raw, fingerprint string) (historyCursor, error) {
	if raw == "" {
		return historyCursor{}, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return historyCursor{}, err
	}
	var c historyCursor
	if json.Unmarshal(body, &c) != nil || c.Time.IsZero() || c.MatchID == "" || c.Seen < 0 || c.Seen > MaxHistoryItems || c.Fingerprint != fingerprint {
		return historyCursor{}, errors.New("invalid cursor")
	}
	return c, nil
}

func historyFingerprint(filter HistoryFilter, puuid string) string {
	return hash(strings.Join([]string{filter.Platform, puuid, fmt.Sprint(filter.QueueID), filter.Locale}, "\x00"))
}

func mapPlayerProfile(row sqlcgen.TftPlayerIdentity, now time.Time, freshness time.Duration) PlayerProfile {
	var refreshed *time.Time
	if row.MatchesRefreshedAt.Valid {
		value := row.MatchesRefreshedAt.Time
		refreshed = &value
	}
	return PlayerProfile{Platform: row.Platform, GameName: row.GameName, TagLine: row.TagLine, LastRefreshedAt: refreshed, IsStale: refreshed == nil || now.Sub(*refreshed) > freshness}
}
func mapHistoryMatch(row sqlcgen.ListTFTPlayerMatchesRow) HistoryMatch {
	return HistoryMatch{MatchID: row.MatchID, Platform: row.Platform, RoutingRegion: row.RoutingRegion, QueueID: int(row.QueueID), GameVersion: row.GameVersion, Patch: row.Patch, GameDatetime: row.GameDatetime.Time, GameLengthSeconds: valueFloat(row.GameLengthSeconds), MapID: int(value32(row.MapID)), TFTGameType: valueString(row.TftGameType), SetCoreName: valueString(row.SetCoreName), SetNumber: int(value32(row.SetNumber)), EndOfGameResult: valueString(row.EndOfGameResult), ParticipantCount: int(row.ParticipantCount), Eligible: row.Eligible, ExclusionReason: valueString(row.ExclusionReason), Participants: []HistoryParticipant{}}
}
func mapHistoryParticipant(row sqlcgen.ListTFTMatchParticipantsForHistoryRow, current bool) HistoryParticipant {
	return HistoryParticipant{PUUID: row.Puuid, GameName: valueString(row.GameName), TagLine: valueString(row.TagLine), IsCurrentPlayer: current, Placement: int(row.Placement), Level: int(value16(row.Level)), GoldLeft: int(value32(row.GoldLeft)), LastRound: int(value32(row.LastRound)), PlayersEliminated: int(value32(row.PlayersEliminated)), TimeEliminatedSeconds: valueFloat(row.TimeEliminatedSeconds), TotalDamageToPlayers: int(value32(row.TotalDamageToPlayers)), CompanionContentID: valueString(row.CompanionContentID), CompanionItemID: int(value32(row.CompanionItemID)), CompanionSkinID: int(value32(row.CompanionSkinID)), CompanionSpecies: valueString(row.CompanionSpecies), Abnormal: row.Abnormal, Augments: []Entity{}, Traits: []HistoryTrait{}, Units: []HistoryUnit{}}
}
func mapJob(row sqlcgen.TftPlayerLookupJob) Job {
	job := Job{ID: row.ID, Platform: row.Platform, GameName: row.RequestedGameName, TagLine: row.RequestedTagLine, Status: row.Status, Stage: row.Stage, ScannedCount: int(row.ScannedCount), FetchedCount: int(row.FetchedCount), FailedCount: int(row.FailedCount), ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
	if row.CompletedAt.Valid {
		value := row.CompletedAt.Time
		job.CompletedAt = &value
	}
	return job
}
func valueString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func value16(value *int16) int16 {
	if value == nil {
		return 0
	}
	return *value
}
func value32(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}
func valueFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
func hash(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }
