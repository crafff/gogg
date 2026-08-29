package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/crafff/gogg/apps/worker/internal/tft/analytics"
	"github.com/crafff/gogg/apps/worker/internal/tft/config"
	"github.com/crafff/gogg/apps/worker/internal/tft/ingest"
	"github.com/crafff/gogg/apps/worker/internal/tft/rawarchive"
	"github.com/crafff/gogg/apps/worker/internal/tft/runtime"
	"github.com/crafff/gogg/apps/worker/internal/tft/sampling"
	"github.com/crafff/gogg/apps/worker/internal/tft/staticdata"
	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const parserVersion = "tft-match-v1"

type Activities struct{ rt *runtime.Runtime }

func New(rt *runtime.Runtime) *Activities { return &Activities{rt: rt} }

type ResolvePlayerInput struct {
	JobID, Platform, GameName, TagLine string
}

type ResolvePlayerResult struct{ PUUID string }

func (a *Activities) ResolvePlayer(ctx context.Context, in ResolvePlayerInput) (ResolvePlayerResult, error) {
	platform := strings.ToUpper(strings.TrimSpace(in.Platform))
	if err := a.rt.Queries.MarkTFTPlayerLookupJobRunning(ctx, "RESOLVE_ACCOUNT", nil, in.JobID); err != nil {
		return ResolvePlayerResult{}, err
	}
	client, err := a.rt.Client(platform)
	if err != nil {
		return ResolvePlayerResult{}, temporal.NewNonRetryableApplicationError(err.Error(), "INVALID_PLATFORM", err)
	}
	release, err := a.rt.Gate.Acquire(ctx, platform, config.RoutingRegion(platform))
	if err != nil {
		return ResolvePlayerResult{}, err
	}
	account, callErr := client.GetAccountByRiotID(ctx, in.GameName, in.TagLine)
	release()
	if callErr != nil {
		if riotapi.IsNotFound(callErr) {
			return ResolvePlayerResult{}, temporal.NewNonRetryableApplicationError("Riot account was not found", "NOT_FOUND", callErr)
		}
		return ResolvePlayerResult{}, collectionError(callErr)
	}
	if account == nil || strings.TrimSpace(account.Puuid) == "" {
		return ResolvePlayerResult{}, temporal.NewNonRetryableApplicationError("Riot account response has no PUUID", "PROFILE_NOT_READY", nil)
	}
	release, err = a.rt.Gate.Acquire(ctx, platform, "")
	if err != nil {
		return ResolvePlayerResult{}, err
	}
	profile, profileErr := client.GetTFTSummonerByPUUID(ctx, account.Puuid)
	release()
	if profileErr != nil {
		if riotapi.IsNotFound(profileErr) {
			return ResolvePlayerResult{}, temporal.NewNonRetryableApplicationError("Riot account has no TFT profile on the selected platform", "NOT_FOUND_IN_REGION", profileErr)
		}
		return ResolvePlayerResult{}, collectionError(profileErr)
	}
	if profile == nil || strings.TrimSpace(profile.Puuid) == "" || profile.Puuid != account.Puuid {
		return ResolvePlayerResult{}, temporal.NewNonRetryableApplicationError("Riot TFT profile is not ready", "PROFILE_NOT_READY", nil)
	}
	now := timestamp(time.Now())
	if err := a.replacePlayerIdentity(ctx, sqlcgen.UpsertTFTPlayerIdentityParams{
		Platform: platform, Puuid: account.Puuid, GameName: account.GameName, TagLine: account.TagLine, IdentityRefreshedAt: now,
	}); err != nil {
		return ResolvePlayerResult{}, err
	}
	if err := a.rt.Queries.MarkTFTPlayerLookupJobRunning(ctx, "FETCH_MATCH_IDS", &account.Puuid, in.JobID); err != nil {
		return ResolvePlayerResult{}, err
	}
	_ = a.rt.Queries.MarkLatestTFTRawCaptureParsed(ctx, sqlcgen.MarkLatestTFTRawCaptureParsedParams{
		ParseStatus: "parsed", ParserVersion: ptr("tft-account-v1"), PlatformFilter: platform, KindFilter: "riot-account", ResourceFilter: in.GameName + "#" + in.TagLine,
	})
	_ = a.rt.Queries.MarkLatestTFTRawCaptureParsed(ctx, sqlcgen.MarkLatestTFTRawCaptureParsedParams{
		ParseStatus: "parsed", ParserVersion: ptr("tft-summoner-v1"), PlatformFilter: platform, KindFilter: "tft-summoner", ResourceFilter: account.Puuid,
	})
	return ResolvePlayerResult{PUUID: account.Puuid}, nil
}

// replacePlayerIdentity keeps the Riot ID and PUUID uniqueness changes atomic.
// PostgreSQL evaluates a data-modifying CTE under one statement snapshot, so a
// DELETE CTE followed by INSERT can still hit the expression unique index.
func (a *Activities) replacePlayerIdentity(ctx context.Context, params sqlcgen.UpsertTFTPlayerIdentityParams) (err error) {
	tx, err := a.rt.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin TFT player identity transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) && err == nil {
			err = fmt.Errorf("rollback TFT player identity transaction: %w", rollbackErr)
		}
	}()
	queries := a.rt.Queries.WithTx(tx)
	if err := queries.DeleteStaleTFTPlayerIdentityForRiotID(ctx, sqlcgen.DeleteStaleTFTPlayerIdentityForRiotIDParams{
		Platform: params.Platform, GameName: params.GameName, TagLine: params.TagLine, Puuid: params.Puuid,
	}); err != nil {
		return fmt.Errorf("delete stale TFT player identity: %w", err)
	}
	if _, err := queries.UpsertTFTPlayerIdentity(ctx, params); err != nil {
		return fmt.Errorf("upsert TFT player identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit TFT player identity transaction: %w", err)
	}
	return nil
}

type FetchPlayerHistoryInput struct {
	JobID, Platform, PUUID string
	Count                  int
}

type FetchPlayerHistoryResult struct{ Scanned, Fetched, Failed int }

type FetchPlayerHistoryHeartbeat struct {
	Scanned, Fetched, Failed int
	MatchIDs                 []string
	CompletedMatchIDs        []string
}

type playerHistoryHeartbeater struct {
	ctx        context.Context
	mu         sync.Mutex
	state      FetchPlayerHistoryHeartbeat
	stop, done chan struct{}
}

func startPlayerHistoryHeartbeater(ctx context.Context, state FetchPlayerHistoryHeartbeat) *playerHistoryHeartbeater {
	h := &playerHistoryHeartbeater{ctx: ctx, state: clonePlayerHeartbeat(state), stop: make(chan struct{}), done: make(chan struct{})}
	h.record()
	go func() {
		defer close(h.done)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.record()
			case <-ctx.Done():
				return
			case <-h.stop:
				return
			}
		}
	}()
	return h
}

func (h *playerHistoryHeartbeater) update(state FetchPlayerHistoryHeartbeat) {
	h.mu.Lock()
	h.state = clonePlayerHeartbeat(state)
	h.mu.Unlock()
	h.record()
}

func (h *playerHistoryHeartbeater) record() {
	h.mu.Lock()
	state := clonePlayerHeartbeat(h.state)
	h.mu.Unlock()
	activity.RecordHeartbeat(h.ctx, state)
}

func (h *playerHistoryHeartbeater) close() {
	close(h.stop)
	<-h.done
}

func clonePlayerHeartbeat(state FetchPlayerHistoryHeartbeat) FetchPlayerHistoryHeartbeat {
	state.MatchIDs = append([]string(nil), state.MatchIDs...)
	state.CompletedMatchIDs = append([]string(nil), state.CompletedMatchIDs...)
	return state
}

func (a *Activities) FetchPlayerHistory(ctx context.Context, in FetchPlayerHistoryInput) (FetchPlayerHistoryResult, error) {
	platform := strings.ToUpper(strings.TrimSpace(in.Platform))
	if in.Count < 1 || in.Count > 100 {
		in.Count = 20
	}
	client, err := a.rt.Client(platform)
	if err != nil {
		return FetchPlayerHistoryResult{}, temporal.NewNonRetryableApplicationError(err.Error(), "INVALID_PLATFORM", err)
	}
	resume := FetchPlayerHistoryHeartbeat{}
	if activity.HasHeartbeatDetails(ctx) {
		if err := activity.GetHeartbeatDetails(ctx, &resume); err != nil {
			return FetchPlayerHistoryResult{}, fmt.Errorf("decode TFT player history heartbeat: %w", err)
		}
	}
	heartbeats := startPlayerHistoryHeartbeater(ctx, resume)
	defer heartbeats.close()
	route := config.RoutingRegion(platform)
	filtered := append([]string(nil), resume.MatchIDs...)
	if len(filtered) == 0 {
		release, err := a.rt.Gate.Acquire(ctx, platform, route)
		if err != nil {
			return FetchPlayerHistoryResult{}, err
		}
		ids, callErr := client.GetTFTMatchIDsByPUUID(ctx, in.PUUID, 0, 0, 0, in.Count)
		release()
		if callErr != nil {
			return FetchPlayerHistoryResult{}, collectionError(callErr)
		}
		filtered = make([]string, 0, len(ids))
		prefix := platform + "_"
		for _, id := range ids {
			if strings.HasPrefix(strings.ToUpper(id), prefix) {
				filtered = append(filtered, id)
			}
		}
		resume.MatchIDs = append([]string(nil), filtered...)
	}
	result := FetchPlayerHistoryResult{Scanned: len(filtered), Fetched: resume.Fetched, Failed: resume.Failed}
	completed := make(map[string]bool, len(resume.CompletedMatchIDs))
	for _, matchID := range resume.CompletedMatchIDs {
		completed[matchID] = true
	}
	resume.Scanned = result.Scanned
	heartbeats.update(resume)
	if err := a.updatePlayerProgress(ctx, in.JobID, result); err != nil {
		return result, err
	}
	for _, matchID := range filtered {
		if completed[matchID] {
			continue
		}
		heartbeats.update(resume)
		release, err := a.rt.Gate.Acquire(ctx, platform, route)
		if err != nil {
			return result, err
		}
		dto, fetchErr := client.GetTFTMatchDetail(ctx, matchID)
		release()
		if fetchErr != nil {
			if riotapi.IsNotFound(fetchErr) {
				result.Failed++
				resume.Failed = result.Failed
				resume.CompletedMatchIDs = append(resume.CompletedMatchIDs, matchID)
				if err := a.updatePlayerProgress(ctx, in.JobID, result); err != nil {
					return result, err
				}
				heartbeats.update(resume)
				continue
			}
			return result, collectionError(fetchErr)
		}
		if dto == nil || dto.Metadata.MatchID != matchID {
			return result, fmt.Errorf("TFT player match identity mismatch requested=%s", matchID)
		}
		if _, err := a.rt.Ingest.Match(ctx, dto, platform, route, nil, nil, ""); err != nil {
			return result, err
		}
		if err := a.rt.Queries.MarkLatestTFTRawCaptureParsed(ctx, sqlcgen.MarkLatestTFTRawCaptureParsedParams{ParseStatus: "parsed", ParserVersion: ptr(parserVersion), PlatformFilter: platform, KindFilter: "tft-match-detail", ResourceFilter: matchID}); err != nil {
			return result, err
		}
		result.Fetched++
		resume.Fetched = result.Fetched
		resume.CompletedMatchIDs = append(resume.CompletedMatchIDs, matchID)
		if err := a.updatePlayerProgress(ctx, in.JobID, result); err != nil {
			return result, err
		}
		heartbeats.update(resume)
	}
	if err := a.rt.Queries.MarkTFTPlayerMatchesRefreshed(ctx, timestamp(time.Now()), platform, in.PUUID); err != nil {
		return result, err
	}
	return result, nil
}

func (a *Activities) updatePlayerProgress(ctx context.Context, jobID string, result FetchPlayerHistoryResult) error {
	return a.rt.Queries.UpdateTFTPlayerLookupJobProgress(ctx, sqlcgen.UpdateTFTPlayerLookupJobProgressParams{
		Stage: "FETCH_MATCHES", ScannedCount: int32(result.Scanned), FetchedCount: int32(result.Fetched), FailedCount: int32(result.Failed), ID: jobID,
	})
}

type FinishPlayerLookupInput struct {
	JobID, Status, ErrorCode, ErrorMessage string
}

func (a *Activities) FinishPlayerLookup(ctx context.Context, in FinishPlayerLookupInput) error {
	return a.rt.Queries.FinishTFTPlayerLookupJob(ctx, sqlcgen.FinishTFTPlayerLookupJobParams{
		ID: in.JobID, Status: in.Status, ErrorCode: optionalString(in.ErrorCode), ErrorMessage: optionalString(in.ErrorMessage),
	})
}

type StartRunInput struct {
	WorkflowID, WorkflowRunID, ScheduleID, ProfileName, Patch, Set string
	WindowStart, WindowEnd                                         time.Time
}

func (a *Activities) StartRun(ctx context.Context, in StartRunInput) (int64, error) {
	if existing, err := a.rt.Queries.GetTFTRunByWorkflowRunID(ctx, in.WorkflowRunID); err == nil {
		return existing.ID, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if _, err := a.rt.Queries.FailStaleTFTRuns(ctx, sqlcgen.FailStaleTFTRunsParams{
		ScheduleID: in.ScheduleID, ProfileName: in.ProfileName,
		Platform: "GLOBAL", WorkflowRunID: in.WorkflowRunID,
	}); err != nil {
		return 0, fmt.Errorf("reconcile stale TFT crawl runs: %w", err)
	}
	cfg, _ := json.Marshal(map[string]any{
		"platforms": a.rt.Cfg.TFT.Platforms, "queue_type": "RANKED_TFT", "queue_id": 1100,
		"master_limit": a.rt.Cfg.TFT.MasterLimit, "diamond_per_division": a.rt.Cfg.TFT.DiamondPerDivision,
		"match_count_per_seed": a.rt.Cfg.TFT.MatchCountPerSeed,
	})
	run, err := a.rt.Queries.CreateTFTRun(ctx, sqlcgen.CreateTFTRunParams{
		WorkflowID: in.WorkflowID, WorkflowRunID: in.WorkflowRunID, ScheduleID: in.ScheduleID, ProfileName: in.ProfileName,
		Platform: "GLOBAL", RoutingRegion: "GLOBAL", QueueType: "RANKED_TFT", QueueID: 1100,
		Status: "running", TargetPatch: optionalString(in.Patch), TargetSet: optionalString(in.Set),
		WindowStart: timestamp(in.WindowStart), WindowEnd: timestamp(in.WindowEnd), Config: cfg,
	})
	if err != nil {
		return 0, fmt.Errorf("create TFT run: %w", err)
	}
	return run.ID, nil
}

func (a *Activities) ResolveTargetPatch(ctx context.Context, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	var (
		snapshot sqlcgen.TftStaticSnapshot
		err      error
	)
	if requested == "" {
		snapshot, err = a.rt.Queries.GetLatestPublishedTFTStaticSnapshotAnyPatch(ctx, "cdragon", "en_us")
	} else {
		requested = ingest.NormalizePatch(requested)
		if requested == "" {
			return "", temporal.NewNonRetryableApplicationError("invalid TFT target patch", "INVALID_PATCH", nil)
		}
		snapshot, err = a.rt.Queries.GetLatestPublishedTFTStaticSnapshot(ctx, "cdragon", requested, "en_us")
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", temporal.NewNonRetryableApplicationError("published TFT static snapshot is required before crawling", "MISSING_STATIC", err)
		}
		return "", err
	}
	if _, _, err := a.loadCatalog(ctx, snapshot.Patch); err != nil {
		return "", temporal.NewNonRetryableApplicationError("published TFT unit catalog is not usable", "INVALID_STATIC", err)
	}
	return snapshot.Patch, nil
}

func (a *Activities) RequeueTargetPatch(ctx context.Context, targetPatch string) (int64, error) {
	rows, err := a.rt.Queries.RequeueTFTMatchesForPatch(ctx, targetPatch)
	if err != nil {
		return 0, fmt.Errorf("requeue archived TFT matches for patch %s: %w", targetPatch, err)
	}
	return rows, nil
}

type RunStateInput struct {
	RunID                                  int64
	Status, DesiredState, Stage, LastError string
}

func (a *Activities) SetRunState(ctx context.Context, in RunStateInput) error {
	return a.rt.Queries.UpdateTFTRunState(ctx, sqlcgen.UpdateTFTRunStateParams{
		ID: in.RunID, Status: in.Status, DesiredState: in.DesiredState, Stage: in.Stage, LastError: optionalString(in.LastError),
	})
}

type SeedTierInput struct {
	RunID          int64
	Platform, Tier string
}

func (a *Activities) FetchTopTier(ctx context.Context, in SeedTierInput) (int, error) {
	client, err := a.rt.Client(in.Platform)
	if err != nil {
		return 0, err
	}
	release, err := a.rt.Gate.Acquire(ctx, in.Platform, "")
	if err != nil {
		return 0, err
	}
	defer release()
	ctx = rawarchive.WithRunID(ctx, in.RunID)
	var list *riotapi.TFTLeagueListDTO
	switch strings.ToUpper(in.Tier) {
	case "CHALLENGER":
		list, err = client.GetTFTChallengerLeague(ctx, "RANKED_TFT")
	case "GRANDMASTER":
		list, err = client.GetTFTGrandmasterLeague(ctx, "RANKED_TFT")
	case "MASTER":
		list, err = client.GetTFTMasterLeague(ctx, "RANKED_TFT")
	default:
		return 0, fmt.Errorf("unsupported TFT top tier %q", in.Tier)
	}
	if err != nil {
		return 0, collectionError(err)
	}
	for _, entry := range list.Entries {
		entry.Tier = strings.ToUpper(in.Tier)
		if err := a.persistSeed(ctx, in.RunID, in.Platform, entry); err != nil {
			return 0, err
		}
	}
	if err := a.saveCheckpoint(ctx, in.RunID, "seed_tier", strings.ToUpper(in.Platform)+":"+strings.ToUpper(in.Tier), map[string]any{"complete": true}, int64(len(list.Entries))); err != nil {
		return 0, err
	}
	if err := a.rt.Queries.RefreshTFTRunCounts(ctx, in.RunID); err != nil {
		return 0, err
	}
	return len(list.Entries), nil
}

type DiamondPageInput struct {
	RunID              int64
	Platform, Division string
	Page               int
}
type PageResult struct {
	Count   int
	HasMore bool
}

func (a *Activities) FetchDiamondPage(ctx context.Context, in DiamondPageInput) (PageResult, error) {
	client, err := a.rt.Client(in.Platform)
	if err != nil {
		return PageResult{}, err
	}
	release, err := a.rt.Gate.Acquire(ctx, in.Platform, "")
	if err != nil {
		return PageResult{}, err
	}
	defer release()
	ctx = rawarchive.WithRunID(ctx, in.RunID)
	entries, err := client.GetTFTLeagueEntries(ctx, "RANKED_TFT", "DIAMOND", in.Division, in.Page)
	if err != nil {
		return PageResult{}, collectionError(err)
	}
	for _, entry := range entries {
		entry.Tier, entry.Rank = "DIAMOND", strings.ToUpper(in.Division)
		if err := a.persistSeed(ctx, in.RunID, in.Platform, entry); err != nil {
			return PageResult{}, err
		}
	}
	result := PageResult{Count: len(entries), HasMore: len(entries) > 0}
	if err := a.saveCheckpoint(ctx, in.RunID, "seed_diamond", strings.ToUpper(in.Platform)+":"+strings.ToUpper(in.Division), map[string]any{"page": in.Page, "has_more": result.HasMore}, int64(len(entries))); err != nil {
		return PageResult{}, err
	}
	if err := a.rt.Queries.RefreshTFTRunCounts(ctx, in.RunID); err != nil {
		return PageResult{}, err
	}
	return result, nil
}

func (a *Activities) persistSeed(ctx context.Context, runID int64, platform string, entry riotapi.TFTLeagueEntryDTO) error {
	if strings.TrimSpace(entry.Puuid) == "" {
		return nil
	}
	cohort := "MASTER_PLUS"
	if strings.EqualFold(entry.Tier, "DIAMOND") {
		cohort = "DIAMOND"
	}
	division := entry.Rank
	if division == "" {
		division = "I"
	}
	return a.rt.Queries.UpsertTFTSeedSnapshot(ctx, sqlcgen.UpsertTFTSeedSnapshotParams{
		RunID: runID, Platform: strings.ToUpper(platform), QueueType: "RANKED_TFT", Cohort: cohort,
		Tier: strings.ToUpper(entry.Tier), Division: optionalString(division), Puuid: entry.Puuid,
		LeagueID: optionalString(entry.LeagueID), LeaguePoints: optionalInt32(entry.LeaguePoints),
		Wins: optionalInt32(entry.Wins), Losses: optionalInt32(entry.Losses), Selected: false,
	})
}

type ApplySamplingInput struct {
	RunID          int64
	Platform, Salt string
	Patch          string
}

func (a *Activities) ApplySampling(ctx context.Context, in ApplySamplingInput) (int, error) {
	rows, err := a.rt.Queries.ListTFTSeedsForSampling(ctx, in.RunID)
	if err != nil {
		return 0, err
	}
	seeds := make([]sampling.Seed, 0, len(rows))
	for _, row := range rows {
		if row.Platform != strings.ToUpper(in.Platform) {
			continue
		}
		seeds = append(seeds, sampling.Seed{ID: row.ID, Puuid: row.Puuid, Tier: row.Tier, Division: value(row.Division), LeaguePoints: int(value32(row.LeaguePoints))})
	}
	patch, patchAge := in.Patch, time.Duration(0)
	if snapshot, snapshotErr := a.rt.Queries.GetLatestTFTStaticSnapshotAnyStatus(ctx, "cdragon", "en_us"); snapshotErr == nil {
		if patch == "" {
			patch = snapshot.Patch
		}
		patchAge = time.Since(snapshot.FetchedAt.Time)
	}
	var observations int64
	if patch != "" {
		observations, _ = a.rt.Queries.CountTFTEligibleObservations(ctx, strings.ToUpper(in.Platform), patch)
	}
	limits := sampling.EffectiveLimits(patchAge, int(observations),
		sampling.Limits{Master: a.rt.Cfg.TFT.MasterLimit, DiamondPerDivision: a.rt.Cfg.TFT.DiamondPerDivision},
		sampling.Limits{Master: a.rt.Cfg.TFT.ScaleMasterLimit, DiamondPerDivision: a.rt.Cfg.TFT.ScaleDiamondPerDivision},
		a.rt.Cfg.TFT.ScaleAfter, a.rt.Cfg.TFT.ScaleBelowObservations)
	salt := in.Salt
	if salt == "" {
		salt = in.Platform + "|" + patch
	}
	selected := sampling.Select(seeds, salt, limits)
	for _, seed := range seeds {
		if err := a.rt.Queries.SetTFTSeedSelected(ctx, selected[seed.ID], seed.ID); err != nil {
			return 0, err
		}
	}
	return len(selected), nil
}

type DiscoverInput struct {
	RunID                  int64
	Platform               string
	Offset, Limit          int
	WindowStart, WindowEnd time.Time
}
type BatchResult struct {
	Processed, Created int
	Remaining          int64
	NextEligibleAt     time.Time
	HasMore            bool
}

func (a *Activities) DiscoverMatches(ctx context.Context, in DiscoverInput) (BatchResult, error) {
	seeds, err := a.rt.Queries.ListSelectedTFTSeedsPage(ctx, sqlcgen.ListSelectedTFTSeedsPageParams{RunID: in.RunID, Platform: strings.ToUpper(in.Platform), RowOffset: int32(in.Offset), RowLimit: int32(in.Limit)})
	if err != nil {
		return BatchResult{}, err
	}
	client, err := a.rt.Client(in.Platform)
	if err != nil {
		return BatchResult{}, err
	}
	created := 0
	for _, seed := range seeds {
		windowStart := in.WindowStart
		previous, syncErr := a.rt.Queries.GetTFTPlayerMatchSync(ctx, seed.Platform, seed.Puuid, "RANKED_TFT")
		if syncErr == nil && previous.WindowEnd.Valid {
			windowStart = effectiveWindowStart(windowStart, previous.WindowEnd.Time, a.rt.Cfg.TFT.Overlap)
		} else if syncErr != nil && !errors.Is(syncErr, pgx.ErrNoRows) {
			return BatchResult{}, syncErr
		}
		release, err := a.rt.Gate.Acquire(ctx, in.Platform, "")
		if err != nil {
			return BatchResult{}, err
		}
		callCtx := rawarchive.WithRunID(ctx, in.RunID)
		ids, callErr := client.GetTFTMatchIDsByPUUID(callCtx, seed.Puuid, windowStart.Unix(), in.WindowEnd.Unix(), 0, a.rt.Cfg.TFT.MatchCountPerSeed)
		release()
		if callErr != nil {
			return BatchResult{}, collectionError(callErr)
		}
		for _, matchID := range ids {
			n, err := a.rt.Queries.InsertTFTMatchDiscovery(ctx, sqlcgen.InsertTFTMatchDiscoveryParams{RunID: in.RunID, Platform: seed.Platform, RoutingRegion: config.RoutingRegion(seed.Platform), SeedPuuid: seed.Puuid, MatchID: matchID, Cohort: seed.Cohort})
			if err != nil {
				return BatchResult{}, err
			}
			created += int(n)
			if _, err := a.rt.Queries.EnqueueTFTMatchJob(ctx, config.RoutingRegion(seed.Platform), matchID, seed.Platform); err != nil {
				return BatchResult{}, err
			}
		}
		var last *string
		if len(ids) > 0 {
			last = &ids[0]
		}
		if err := a.rt.Queries.UpsertTFTPlayerMatchSync(ctx, sqlcgen.UpsertTFTPlayerMatchSyncParams{
			Platform: seed.Platform, Puuid: seed.Puuid, QueueType: "RANKED_TFT", WindowStart: timestamp(windowStart),
			WindowEnd: timestamp(in.WindowEnd), LastSyncedAt: timestamp(time.Now()), LastMatchID: last,
		}); err != nil {
			return BatchResult{}, err
		}
	}
	if err := a.saveCheckpoint(ctx, in.RunID, "match_discovery", strings.ToUpper(in.Platform), map[string]any{"next_offset": in.Offset + len(seeds)}, int64(in.Offset+len(seeds))); err != nil {
		return BatchResult{}, err
	}
	if err := a.rt.Queries.RefreshTFTRunCounts(ctx, in.RunID); err != nil {
		return BatchResult{}, err
	}
	return BatchResult{Processed: len(seeds), Created: created, HasMore: len(seeds) == in.Limit}, nil
}

type DispatchInput struct {
	RunID         int64
	RoutingRegion string
	TargetPatch   string
	Limit         int
}

func (a *Activities) DispatchMatches(ctx context.Context, in DispatchInput) (BatchResult, error) {
	if strings.TrimSpace(in.TargetPatch) == "" {
		return BatchResult{}, temporal.NewNonRetryableApplicationError("TFT target patch is empty", "INVALID_PATCH", nil)
	}
	owner := activityLeaseOwner(ctx)
	ownerRef := &owner
	route := strings.ToUpper(in.RoutingRegion)
	jobs, err := a.rt.Queries.ClaimTFTMatchJobs(ctx, sqlcgen.ClaimTFTMatchJobsParams{LeaseOwner: ownerRef, LeaseSeconds: 360, RouteFilter: route, RowLimit: int32(in.Limit)})
	if err != nil {
		return BatchResult{}, err
	}
	defer func() { _ = a.rt.Queries.ReleaseTFTMatchLeases(context.WithoutCancel(ctx), route, ownerRef) }()
	completed := 0
	for _, job := range jobs {
		client, err := a.rt.Client(job.Platform)
		if err != nil {
			if retryErr := a.retryJob(ctx, job, ownerRef, err, time.Minute, true, nil); retryErr != nil {
				return BatchResult{}, retryErr
			}
			continue
		}
		release, err := a.rt.Gate.Acquire(ctx, job.Platform, job.RoutingRegion)
		if err != nil {
			if retryErr := a.retryJob(ctx, job, ownerRef, err, time.Minute, true, nil); retryErr != nil {
				return BatchResult{}, errors.Join(err, retryErr)
			}
			return BatchResult{}, err
		}
		callCtx := rawarchive.WithRunID(ctx, in.RunID)
		dto, callErr := client.GetTFTMatchDetail(callCtx, job.MatchID)
		release()
		if callErr != nil {
			if riotapi.IsNotFound(callErr) {
				code := int32(404)
				message := callErr.Error()
				if err := a.completeJob(ctx, job, ownerRef, "terminal", &code, &message); err != nil {
					return BatchResult{}, err
				}
				continue
			}
			var apiErr *riotapi.APIError
			if riotapi.IsUnauthorized(callErr) {
				if retryErr := a.retryJob(ctx, job, ownerRef, callErr, 5*time.Minute, false, nil); retryErr != nil {
					return BatchResult{}, errors.Join(callErr, retryErr)
				}
				return BatchResult{}, collectionError(callErr)
			}
			if errors.As(callErr, &apiErr) && !apiErr.Retryable {
				message := callErr.Error()
				code := int32(apiErr.StatusCode)
				if err := a.completeJob(ctx, job, ownerRef, "terminal", &code, &message); err != nil {
					return BatchResult{}, err
				}
				continue
			}
			if errors.As(callErr, &apiErr) && apiErr.Kind == riotapi.ErrorRateLimit {
				a.rt.Gate.ObserveRateLimit(job.RoutingRegion, time.Now())
			}
			next := time.Now().Add(time.Minute)
			if apiErr != nil && apiErr.RetryAfter > 0 {
				next = time.Now().Add(apiErr.RetryAfter)
			}
			var code *int32
			if apiErr != nil && apiErr.StatusCode > 0 {
				v := int32(apiErr.StatusCode)
				code = &v
			}
			if err := a.retryJob(ctx, job, ownerRef, callErr, time.Until(next), true, code); err != nil {
				return BatchResult{}, err
			}
			continue
		}
		if dto.Metadata.MatchID != job.MatchID {
			err := fmt.Errorf("TFT match identity mismatch requested=%s got=%s", job.MatchID, dto.Metadata.MatchID)
			if retryErr := a.retryJob(ctx, job, ownerRef, err, time.Hour, true, nil); retryErr != nil {
				return BatchResult{}, retryErr
			}
			continue
		}
		patch := ingest.NormalizePatch(dto.Info.GameVersion)
		var catalog ingest.Catalog
		var revision *int64
		if patch == in.TargetPatch && dto.Info.QueueID == ingest.StandardRankedQueueID {
			catalog, revision, err = a.loadCatalog(ctx, patch)
			if err != nil {
				if retryErr := a.retryJob(ctx, job, ownerRef, err, 30*time.Minute, false, nil); retryErr != nil {
					return BatchResult{}, retryErr
				}
				continue
			}
		}
		if _, err := a.rt.Ingest.Match(ctx, dto, job.Platform, job.RoutingRegion, catalog, revision, in.TargetPatch); err != nil {
			if retryErr := a.retryJob(ctx, job, ownerRef, err, time.Minute, true, nil); retryErr != nil {
				return BatchResult{}, errors.Join(err, retryErr)
			}
			continue
		}
		if err := a.rt.Queries.MarkLatestTFTRawCaptureParsed(ctx, sqlcgen.MarkLatestTFTRawCaptureParsedParams{ParseStatus: "parsed", ParserVersion: ptr(parserVersion), PlatformFilter: job.Platform, KindFilter: "tft-match-detail", ResourceFilter: job.MatchID}); err != nil {
			return BatchResult{}, err
		}
		if err := a.completeJob(ctx, job, ownerRef, "completed", nil, nil); err != nil {
			return BatchResult{}, err
		}
		a.rt.Gate.ObserveSuccess(job.RoutingRegion, time.Now())
		completed++
	}
	if err := a.saveCheckpoint(ctx, in.RunID, "match_detail", strings.ToUpper(in.RoutingRegion), map[string]any{"last_batch_size": len(jobs)}, int64(len(jobs))); err != nil {
		return BatchResult{}, err
	}
	if err := a.rt.Queries.RefreshTFTRunCounts(ctx, in.RunID); err != nil {
		return BatchResult{}, err
	}
	queue, err := a.rt.Queries.GetTFTMatchQueueState(ctx, route)
	if err != nil {
		return BatchResult{}, err
	}
	result := BatchResult{Processed: len(jobs), Created: completed, Remaining: queue.Remaining, HasMore: queue.Remaining > 0}
	if queue.NextEligibleAt.Valid {
		result.NextEligibleAt = queue.NextEligibleAt.Time
	}
	return result, nil
}

func (a *Activities) saveCheckpoint(ctx context.Context, runID int64, stage, scope string, cursor any, processed int64) error {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return fmt.Errorf("encode TFT checkpoint: %w", err)
	}
	return a.rt.Queries.UpsertTFTCheckpoint(ctx, sqlcgen.UpsertTFTCheckpointParams{
		RunID: runID, Stage: stage, ScopeKey: scope, Cursor: payload, Processed: processed,
		NextEligibleAt: timestamp(time.Now()), Attempt: 0,
	})
}

func collectionError(err error) error {
	if riotapi.IsUnauthorized(err) {
		return temporal.NewNonRetryableApplicationError("Riot API key rejected during TFT collection", "RIOT_AUTH", err)
	}
	return err
}

func effectiveWindowStart(base, previousEnd time.Time, overlap time.Duration) time.Time {
	candidate := previousEnd.Add(-overlap)
	if candidate.After(base) {
		return candidate
	}
	return base
}

func (a *Activities) retryJob(ctx context.Context, job sqlcgen.TftMatchJob, owner *string, cause error, after time.Duration, exhaustible bool, statusCode *int32) error {
	message := cause.Error()
	if exhaustible && job.Attempt >= 12 {
		return a.completeJob(ctx, job, owner, "terminal", statusCode, &message)
	}
	rows, err := a.rt.Queries.RetryTFTMatchJob(ctx, sqlcgen.RetryTFTMatchJobParams{NextEligibleAt: timestamp(time.Now().Add(max(after, 0))), LastStatusCode: statusCode, LastError: &message, RoutingRegion: job.RoutingRegion, MatchID: job.MatchID, LeaseOwner: owner})
	return requireLeaseUpdate("retry TFT match job", rows, err)
}

func (a *Activities) completeJob(ctx context.Context, job sqlcgen.TftMatchJob, owner *string, status string, statusCode *int32, lastError *string) error {
	rows, err := a.rt.Queries.CompleteTFTMatchJob(ctx, sqlcgen.CompleteTFTMatchJobParams{Status: status, LastStatusCode: statusCode, LastError: lastError, RoutingRegion: job.RoutingRegion, MatchID: job.MatchID, LeaseOwner: owner})
	return requireLeaseUpdate("complete TFT match job", rows, err)
}

func requireLeaseUpdate(action string, rows int64, err error) error {
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%s: lease was lost", action)
	}
	return nil
}

func activityLeaseOwner(ctx context.Context) string {
	info := activity.GetInfo(ctx)
	return fmt.Sprintf("%s/%s/%d", info.WorkflowExecution.RunID, info.ActivityID, info.Attempt)
}

func (a *Activities) loadCatalog(ctx context.Context, patch string) (ingest.Catalog, *int64, error) {
	if patch == "" {
		return nil, nil, errors.New("TFT match patch is invalid")
	}
	catalog := ingest.Catalog{}
	snapshot, err := a.rt.Queries.GetLatestPublishedTFTStaticSnapshot(ctx, "cdragon", patch, "en_us")
	if err != nil {
		return nil, nil, fmt.Errorf("load published TFT static snapshot for patch %s: %w", patch, err)
	}
	rows, err := a.rt.Queries.ListTFTPurchasableUnits(ctx, snapshot.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("load TFT unit catalog for patch %s: %w", patch, err)
	}
	for _, row := range rows {
		catalog[row.ObjectID] = ingest.UnitDefinition{Purchasable: true, Cost: int(value32(row.Cost))}
	}
	if len(catalog) == 0 {
		return nil, nil, fmt.Errorf("published TFT unit catalog for patch %s is empty", patch)
	}
	return catalog, &snapshot.ID, nil
}

func (a *Activities) SyncStatic(ctx context.Context) (staticdata.SyncResult, error) {
	return staticdata.Sync(ctx, a.rt.Queries, staticdata.Options{
		Root: a.rt.Cfg.TFT.StaticRoot, Locales: a.rt.Cfg.TFT.StaticLocales,
		Client: &http.Client{Timeout: 90 * time.Second},
	})
}

func (a *Activities) SyncStaticSource(ctx context.Context, input staticdata.SourceSyncInput) (staticdata.SyncResult, error) {
	return staticdata.SyncSource(ctx, a.rt.Queries, staticdata.Options{
		Root: a.rt.Cfg.TFT.StaticRoot, Locales: a.rt.Cfg.TFT.StaticLocales,
		Client: &http.Client{Timeout: 90 * time.Second},
	}, input)
}

func (a *Activities) DownloadStaticAssets(ctx context.Context, limit int) (staticdata.DownloadResult, error) {
	return staticdata.DownloadAssets(ctx, a.rt.Queries, a.rt.Cfg.TFT.StaticRoot,
		&http.Client{Timeout: 60 * time.Second}, activityLeaseOwner(ctx), limit)
}

func (a *Activities) DownloadStaticAssetsForSnapshots(ctx context.Context, snapshotIDs []int64) (staticdata.DownloadResult, error) {
	result, err := staticdata.DownloadAssetsForSnapshots(ctx, a.rt.Queries, a.rt.Cfg.TFT.StaticRoot,
		&http.Client{Timeout: 60 * time.Second}, activityLeaseOwner(ctx), staticdata.DownloadInput{
			SnapshotIDs: snapshotIDs,
			BatchSize:   a.rt.Cfg.TFT.StaticDownloadBatch,
			Concurrency: a.rt.Cfg.TFT.StaticDownloadWorkers,
		})
	var exhausted *staticdata.ExhaustedAssetsError
	if errors.As(err, &exhausted) {
		return staticdata.DownloadResult{}, temporal.NewNonRetryableApplicationError(err.Error(), "STATIC_ASSET_EXHAUSTED", err)
	}
	return result, err
}

func (a *Activities) PublishStaticSnapshots(ctx context.Context, snapshotIDs []int64) error {
	for _, snapshotID := range snapshotIDs {
		rows, err := a.rt.Queries.PublishTFTStaticSnapshot(ctx, snapshotID)
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("TFT static snapshot %d still has incomplete assets", snapshotID)
		}
	}
	return nil
}

type AnalysisTarget struct {
	Platform, Patch           string
	SetNumber, QueueID        int
	FirstMatchAt, LastMatchAt time.Time
}

func (a *Activities) ListAnalysisTargets(ctx context.Context, notBefore time.Time) ([]AnalysisTarget, error) {
	rows, err := a.rt.Queries.ListTFTAnalysisTargets(ctx, timestamp(notBefore))
	if err != nil {
		return nil, err
	}
	out := make([]AnalysisTarget, 0, len(rows))
	for _, row := range rows {
		if row.SetNumber == nil || !row.FirstMatchAt.Valid || !row.LastMatchAt.Valid {
			continue
		}
		out = append(out, AnalysisTarget{Platform: row.Platform, Patch: row.Patch, SetNumber: int(*row.SetNumber), QueueID: int(row.QueueID), FirstMatchAt: row.FirstMatchAt.Time, LastMatchAt: row.LastMatchAt.Time})
	}
	return out, nil
}

type PublishAnalysisInput struct {
	Target                 AnalysisTarget
	Cohort, WindowKind     string
	WindowStart, WindowEnd time.Time
}

type PublishAnalysisResult struct {
	Observations int64
	Families     int
	Published    bool
}

func (a *Activities) PublishAnalysis(ctx context.Context, in PublishAnalysisInput) (PublishAnalysisResult, error) {
	observations, families, err := analytics.New(a.rt.Pool).Publish(ctx, analytics.Filter{
		Platform: in.Target.Platform, Patch: in.Target.Patch, SetNumber: in.Target.SetNumber, QueueID: in.Target.QueueID,
		Cohort: in.Cohort, WindowKind: in.WindowKind, WindowStart: in.WindowStart, WindowEnd: in.WindowEnd, MinFamilySamples: 200,
	})
	if errors.Is(err, analytics.ErrInsufficientCoverage) {
		return PublishAnalysisResult{Observations: observations}, nil
	}
	return PublishAnalysisResult{Observations: observations, Families: families, Published: err == nil}, err
}

func timestamp(v time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: v.UTC(), Valid: true} }
func optionalString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}
func optionalInt32(v int) *int32 { n := int32(v); return &n }
func ptr[T any](v T) *T          { return &v }
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func value32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}
