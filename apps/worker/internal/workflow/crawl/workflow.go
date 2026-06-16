// Package crawl holds the CrawlRegionWorkflow that replaces the legacy
// internal/crawler/runner.go. Chunk 2 wires Phase 0 + Phase 1; chunks
// 3-4 extend it with Phase 2/3/3.5/4 and Phase 5/5.5.
//
// The workflow is deterministic: no clock reads, no map iteration, no
// goroutines — all I/O is through Activities. The legacy runs row
// keeps audit semantics (CreateRun → PinRunVersion → CompleteRun /
// FailRun) so reporting queries that filter by runs.status stay
// meaningful.
package crawl

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	crawlact "github.com/crafff/gogg/apps/worker/internal/activity/crawl"
)

// CrawlRegionInput is the workflow's external interface. Either
// ProfileName (resolved server-side via the ResolveProfile activity)
// OR an inline Profile may be supplied. ProfileName wins if both are
// set — mirrors the legacy `--profile name` precedence.
type CrawlRegionInput struct {
	ProfileName string                   `json:"profile_name,omitempty"`
	Profile     crawlact.ProfileSnapshot `json:"profile"`
}

// CrawlRegionOutput summarises the run. Per-phase outputs are
// retained so dashboards / parity tests can diff workflow runs without
// scraping event history.
type CrawlRegionOutput struct {
	RunID           int                     `json:"run_id"`
	ResolvedVersion string                  `json:"resolved_version"`
	Phase0Output    crawlact.Phase0Output   `json:"phase0"`
	Phase1Output    crawlact.Phase1Output   `json:"phase1"`
	Phase2Outputs   []crawlact.Phase2Output `json:"phase2"`
	Phase3Output    crawlact.Phase3Output   `json:"phase3"`
	Phase35Output   crawlact.Phase35Output  `json:"phase35"`
	Phase4Output    crawlact.Phase4Output   `json:"phase4"`
	Phase5Output    crawlact.Phase5Output   `json:"phase5"`
	Phase55Output   crawlact.Phase55Output  `json:"phase55"`
	StartedAt       time.Time               `json:"started_at"`
}

// Bookkeeping activities (CreateRun, PinRunVersion, CompleteRun,
// FailRun) are short DB writes; phase activities are network-bound.
// Distinct option sets so the SLA per category is clear.
var (
	bookkeepingOpts = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	}
	phase0Opts = workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	// Phase 1 walks every tier in RankPrefetchTiers, pagination
	// included. A KR daily run takes minutes; the upper bound covers
	// a cold division-tier walk on a slow link.
	phase1Opts = workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	// Phase 2-4 share a generous timeout because the legacy phase
	// code doesn't heartbeat mid-loop yet. Chunk 4 will refactor the
	// inner loops to heartbeat per batch, at which point we'll tighten
	// HeartbeatTimeout. Today the only heartbeat fires on Activity
	// entry so HeartbeatTimeout MUST be unset (=disabled) — a finite
	// value would trip during the per-puuid pagination.
	phase2Opts = workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	phase3Opts = workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	phase35Opts = workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	phase4Opts = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	// Phase 5 hits Riot's timeline endpoint per match, which returns
	// multi-MB payloads — the heaviest network burden in the pipeline.
	// Generous timeout to cover a cold KR daily walk.
	phase5Opts = workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	// Phase 5.5 is DB-bound (no Riot per-match call); CDragon catalog
	// fetch is the only network hop and that's per-patch cached. 30
	// minutes is comfortable.
	phase55Opts = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
)

// CrawlRegionWorkflow orchestrates Phase 0 + Phase 1 for one region.
// Any phase error fails the workflow; FailRun is invoked via a
// disconnected context so the audit row stamps even on cancellation.
func CrawlRegionWorkflow(ctx workflow.Context, in CrawlRegionInput) (CrawlRegionOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("crawl_region_starting", "profile_name", in.ProfileName, "region", in.Profile.Region)

	profile, err := resolveProfile(ctx, in)
	if err != nil {
		return CrawlRegionOutput{}, err
	}
	if profile.Region == "" {
		return CrawlRegionOutput{}, fmt.Errorf("region must be set on profile or via name lookup")
	}

	startedAt := workflow.Now(ctx)
	runID, err := createRun(ctx, in.ProfileName, profile)
	if err != nil {
		return CrawlRegionOutput{}, err
	}
	logger.Info("crawl_region_run_created", "run_id", runID, "region", profile.Region)

	out, phaseErr := runPhases(ctx, runID, profile, startedAt)
	if phaseErr != nil {
		dctx, cancel := workflow.NewDisconnectedContext(ctx)
		defer cancel()
		dctx = workflow.WithActivityOptions(dctx, bookkeepingOpts)
		_ = workflow.ExecuteActivity(dctx, (*crawlact.Activities).FailRun, runID).Get(dctx, nil)
		return CrawlRegionOutput{}, phaseErr
	}

	logger.Info("crawl_region_completed",
		"run_id", runID,
		"resolved_version", out.ResolvedVersion,
		"phase1_tiers", out.Phase1Output.TierCounts,
	)
	return out, nil
}

// runPhases executes Phase 0 → optional version pin → Phase 1 → run
// complete. Returns the fully-formed CrawlRegionOutput on success.
func runPhases(ctx workflow.Context, runID int, profile crawlact.ProfileSnapshot, startedAt time.Time) (CrawlRegionOutput, error) {
	ctx0 := workflow.WithActivityOptions(ctx, phase0Opts)
	var p0 crawlact.Phase0Output
	if err := workflow.ExecuteActivity(ctx0, (*crawlact.Activities).Phase0VersionSync, crawlact.Phase0Input{
		Region:        profile.Region,
		PinnedVersion: profile.Version,
	}).Get(ctx0, &p0); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase0: %w", err)
	}

	ctxBK := workflow.WithActivityOptions(ctx, bookkeepingOpts)
	// Pin the resolved version on the run row when the profile didn't
	// already pin one — matches legacy phase0 mutating state.Profile.Version.
	if profile.Version == "" {
		if err := workflow.ExecuteActivity(ctxBK, (*crawlact.Activities).PinRunVersion, crawlact.PinRunVersionInput{
			RunID:   runID,
			Version: p0.ResolvedVersion,
		}).Get(ctxBK, nil); err != nil {
			return CrawlRegionOutput{}, fmt.Errorf("pin run version: %w", err)
		}
	}

	ctx1 := workflow.WithActivityOptions(ctx, phase1Opts)
	var p1 crawlact.Phase1Output
	if err := workflow.ExecuteActivity(ctx1, (*crawlact.Activities).Phase1RankSnapshot, crawlact.Phase1Input{
		RunID:             runID,
		Region:            profile.Region,
		Queue:             profile.Queue,
		RankPrefetchTiers: profile.RankPrefetchTiers,
	}).Get(ctx1, &p1); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase1: %w", err)
	}

	p2Outputs, err := runPhase2(ctx, runID, profile, p0.ResolvedVersion, startedAt)
	if err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase2: %w", err)
	}

	ctx3 := workflow.WithActivityOptions(ctx, phase3Opts)
	var p3 crawlact.Phase3Output
	if err := workflow.ExecuteActivity(ctx3, (*crawlact.Activities).Phase3MatchDetails, crawlact.Phase3Input{
		RunID:        runID,
		Region:       profile.Region,
		Version:      p0.ResolvedVersion,
		RunStartedAt: startedAt,
	}).Get(ctx3, &p3); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase3: %w", err)
	}

	ctx35 := workflow.WithActivityOptions(ctx, phase35Opts)
	var p35 crawlact.Phase35Output
	if err := workflow.ExecuteActivity(ctx35, (*crawlact.Activities).Phase35OnDemandRank, crawlact.Phase35Input{
		RunID:        runID,
		Region:       profile.Region,
		Queue:        profile.Queue,
		RunStartedAt: startedAt,
	}).Get(ctx35, &p35); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase35: %w", err)
	}

	ctx4 := workflow.WithActivityOptions(ctx, phase4Opts)
	var p4 crawlact.Phase4Output
	if err := workflow.ExecuteActivity(ctx4, (*crawlact.Activities).Phase4AvgTierCalc, crawlact.Phase4Input{
		RunID:        runID,
		Region:       profile.Region,
		RunStartedAt: startedAt,
	}).Get(ctx4, &p4); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase4: %w", err)
	}

	ctx5 := workflow.WithActivityOptions(ctx, phase5Opts)
	var p5 crawlact.Phase5Output
	if err := workflow.ExecuteActivity(ctx5, (*crawlact.Activities).Phase5Timeline, crawlact.Phase5Input{
		RunID:        runID,
		Region:       profile.Region,
		Version:      p0.ResolvedVersion,
		RunStartedAt: startedAt,
	}).Get(ctx5, &p5); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase5: %w", err)
	}

	ctx55 := workflow.WithActivityOptions(ctx, phase55Opts)
	var p55 crawlact.Phase55Output
	if err := workflow.ExecuteActivity(ctx55, (*crawlact.Activities).Phase55ItemClassify, crawlact.Phase55Input{
		RunID:        runID,
		Region:       profile.Region,
		Version:      p0.ResolvedVersion,
		RunStartedAt: startedAt,
	}).Get(ctx55, &p55); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("phase55: %w", err)
	}

	if err := workflow.ExecuteActivity(ctxBK, (*crawlact.Activities).CompleteRun, runID).Get(ctxBK, nil); err != nil {
		return CrawlRegionOutput{}, fmt.Errorf("complete run: %w", err)
	}

	return CrawlRegionOutput{
		RunID:           runID,
		ResolvedVersion: p0.ResolvedVersion,
		Phase0Output:    p0,
		Phase1Output:    p1,
		Phase2Outputs:   p2Outputs,
		Phase3Output:    p3,
		Phase35Output:   p35,
		Phase4Output:    p4,
		Phase5Output:    p5,
		Phase55Output:   p55,
		StartedAt:       startedAt,
	}, nil
}

// runPhase2 picks the dispatch shape:
//   - execution=sequential (or unset): one Activity with all TargetTiers
//     inline. Matches legacy SequentialStrategy.
//   - execution=pipeline: fan out one Activity per tier with
//     workflow.Future and await all. New in Phase C — legacy
//     PipelineStrategy was actually per-tier *serial*, so this is a
//     real speedup when tiers don't share rate-limit budget.
//
// Phase 3/3.5/4 stay sequential after the barrier because they operate
// on global pending tables — concurrent invocations would race.
func runPhase2(
	ctx workflow.Context,
	runID int,
	profile crawlact.ProfileSnapshot,
	resolvedVersion string,
	startedAt time.Time,
) ([]crawlact.Phase2Output, error) {
	if len(profile.TargetTiers) == 0 {
		return nil, fmt.Errorf("phase2: profile.target_tiers must be non-empty")
	}
	ctx2 := workflow.WithActivityOptions(ctx, phase2Opts)

	if profile.Execution != "pipeline" {
		var out crawlact.Phase2Output
		if err := workflow.ExecuteActivity(ctx2, (*crawlact.Activities).Phase2MatchIDCollection, crawlact.Phase2Input{
			RunID:        runID,
			Region:       profile.Region,
			Version:      resolvedVersion,
			Queue:        profile.Queue,
			Tiers:        profile.TargetTiers,
			RunStartedAt: startedAt,
		}).Get(ctx2, &out); err != nil {
			return nil, err
		}
		return []crawlact.Phase2Output{out}, nil
	}

	// Pipeline mode: fan out per-tier futures, gather in order.
	futures := make([]workflow.Future, len(profile.TargetTiers))
	for i, tier := range profile.TargetTiers {
		futures[i] = workflow.ExecuteActivity(ctx2, (*crawlact.Activities).Phase2MatchIDCollection, crawlact.Phase2Input{
			RunID:        runID,
			Region:       profile.Region,
			Version:      resolvedVersion,
			Queue:        profile.Queue,
			Tiers:        []string{tier},
			RunStartedAt: startedAt,
		})
	}
	outs := make([]crawlact.Phase2Output, len(futures))
	for i, f := range futures {
		if err := f.Get(ctx2, &outs[i]); err != nil {
			return nil, fmt.Errorf("phase2 tier %s: %w", profile.TargetTiers[i], err)
		}
	}
	return outs, nil
}

func resolveProfile(ctx workflow.Context, in CrawlRegionInput) (crawlact.ProfileSnapshot, error) {
	if in.ProfileName == "" {
		return in.Profile, nil
	}
	ctxBK := workflow.WithActivityOptions(ctx, bookkeepingOpts)
	var p crawlact.ProfileSnapshot
	if err := workflow.ExecuteActivity(ctxBK, (*crawlact.Activities).ResolveProfile, crawlact.ResolveProfileInput{
		ProfileName: in.ProfileName,
	}).Get(ctxBK, &p); err != nil {
		return crawlact.ProfileSnapshot{}, fmt.Errorf("resolve profile %q: %w", in.ProfileName, err)
	}
	return p, nil
}

func createRun(ctx workflow.Context, profileName string, p crawlact.ProfileSnapshot) (int, error) {
	ctxBK := workflow.WithActivityOptions(ctx, bookkeepingOpts)
	var res crawlact.CreateRunOutput
	if err := workflow.ExecuteActivity(ctxBK, (*crawlact.Activities).CreateRun, crawlact.CreateRunInput{
		ProfileName:       profileName,
		Region:            p.Region,
		Mode:              p.Mode,
		TargetTiers:       p.TargetTiers,
		RankPrefetchTiers: p.RankPrefetchTiers,
		Queue:             p.Queue,
		Execution:         p.Execution,
		Version:           p.Version,
	}).Get(ctxBK, &res); err != nil {
		return 0, fmt.Errorf("create run: %w", err)
	}
	return res.RunID, nil
}
