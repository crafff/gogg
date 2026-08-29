package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/apps/worker/internal/tft/activity"
	"github.com/crafff/gogg/packages/tftcontract"
)

const (
	activityBatchSize = 10
	maxDiamondPages   = 1000
)

func Crawl(ctx workflow.Context, in tftcontract.CrawlInput) error {
	state := tftcontract.CrawlStatus{State: "running", Stage: "starting"}
	if err := workflow.SetQueryHandler(ctx, tftcontract.CrawlStatusQueryName, func() (tftcontract.CrawlStatus, error) { return state, nil }); err != nil {
		return err
	}
	control := workflow.GetSignalChannel(ctx, tftcontract.CrawlSignalName)
	now := workflow.Now(ctx)
	if in.WindowLag <= 0 {
		in.WindowLag = 30 * time.Minute
	}
	if in.Window <= 0 {
		in.Window = 7 * 24 * time.Hour
	}
	if in.WindowEnd.IsZero() {
		in.WindowEnd = now.Add(-in.WindowLag)
	}
	if in.WindowStart.IsZero() {
		in.WindowStart = in.WindowEnd.Add(-in.Window)
	}
	if in.ProfileName == "" {
		in.ProfileName = "global_high_tier"
	}
	if len(in.Platforms) == 0 {
		return temporal.NewNonRetryableApplicationError("TFT platforms are empty", "INVALID_CONFIG", nil)
	}

	base := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: time.Minute, MaximumAttempts: 5},
	})
	var targetPatch string
	if err := workflow.ExecuteActivity(base, "Activities.ResolveTargetPatch", in.Patch).Get(ctx, &targetPatch); err != nil {
		return err
	}
	in.Patch = targetPatch
	info := workflow.GetInfo(ctx)
	var runID int64
	if err := workflow.ExecuteActivity(base, "Activities.StartRun", activity.StartRunInput{
		WorkflowID: info.WorkflowExecution.ID, WorkflowRunID: info.WorkflowExecution.RunID, ScheduleID: tftcontract.DefaultScheduleID,
		ProfileName: in.ProfileName, Patch: in.Patch, Set: in.Set, WindowStart: in.WindowStart, WindowEnd: in.WindowEnd,
		Platforms: append([]string(nil), in.Platforms...),
	}).Get(ctx, &runID); err != nil {
		return err
	}
	state.RunID = runID
	var requeued int64
	if err := workflow.ExecuteActivity(base, "Activities.RequeueTargetPatch", in.Patch).Get(ctx, &requeued); err != nil {
		return failRun(base, runID, "starting", err)
	}

	for {
		state.State, state.Stage, state.PausePending = "running", "seed_and_discover", false
		paused, cancelled, err := runPlatformStage(base, control, in, runID, &state)
		if cancelled {
			return setTerminal(base, runID, "cancelled", state.Stage, nil)
		}
		if err != nil {
			if resume, cancel, handled, waitErr := handleAuthError(base, control, runID, &state, err); handled {
				if waitErr != nil {
					return failRun(base, runID, state.Stage, waitErr)
				}
				if cancel {
					return setTerminal(base, runID, "cancelled", state.Stage, nil)
				}
				if resume {
					continue
				}
			}
			return failRun(base, runID, state.Stage, err)
		}
		if paused {
			resume, cancel, waitErr := waitPaused(base, control, runID, &state)
			if waitErr != nil {
				return failRun(base, runID, state.Stage, waitErr)
			}
			if cancel {
				return setTerminal(base, runID, "cancelled", state.Stage, nil)
			}
			if resume {
				continue
			}
		}

		state.Stage = "match_detail"
		paused, cancelled, err = runRouteStage(base, control, runID, in.Patch, &state)
		if cancelled {
			return setTerminal(base, runID, "cancelled", state.Stage, nil)
		}
		if err != nil {
			if resume, cancel, handled, waitErr := handleAuthError(base, control, runID, &state, err); handled {
				if waitErr != nil {
					return failRun(base, runID, state.Stage, waitErr)
				}
				if cancel {
					return setTerminal(base, runID, "cancelled", state.Stage, nil)
				}
				if resume {
					continue
				}
			}
			return failRun(base, runID, state.Stage, err)
		}
		if paused {
			resume, cancel, waitErr := waitPaused(base, control, runID, &state)
			if waitErr != nil {
				return failRun(base, runID, state.Stage, waitErr)
			}
			if cancel {
				return setTerminal(base, runID, "cancelled", state.Stage, nil)
			}
			if resume {
				continue
			}
		}
		state.Stage = "analysis"
		paused, cancelled, err = runAnalysisStage(base, control, in, runID, &state)
		if cancelled {
			return setTerminal(base, runID, "cancelled", state.Stage, nil)
		}
		if err != nil {
			return failRun(base, runID, state.Stage, err)
		}
		if paused {
			resume, cancel, waitErr := waitPaused(base, control, runID, &state)
			if waitErr != nil {
				return failRun(base, runID, state.Stage, waitErr)
			}
			if cancel {
				return setTerminal(base, runID, "cancelled", state.Stage, nil)
			}
			if resume {
				continue
			}
		}
		state.State, state.Stage = "completed", "completed"
		return setTerminal(base, runID, "completed", "completed", nil)
	}
}

func runAnalysisStage(ctx workflow.Context, control workflow.ReceiveChannel, in tftcontract.CrawlInput, runID int64, state *tftcontract.CrawlStatus) (bool, bool, error) {
	state.StageCompleted, state.StageTotal = 0, 0
	var targets []activity.AnalysisTarget
	if err := workflow.ExecuteActivity(ctx, "Activities.ListAnalysisTargets", in.WindowStart).Get(ctx, &targets); err != nil {
		return false, false, err
	}
	if len(targets) == 0 {
		return false, false, nil
	}
	state.StageCompleted, state.StageTotal = 0, len(targets)
	stageCtx, cancelActivities := workflow.WithCancel(ctx)
	results := workflow.NewBufferedChannel(ctx, len(targets))
	for _, targetValue := range targets {
		target := targetValue
		workflow.Go(stageCtx, func(gctx workflow.Context) {
			var stageErr error
			for _, cohort := range []string{"MASTER_PLUS", "DIAMOND"} {
				for _, windowKind := range []string{"THREE_DAYS", "PATCH"} {
					start := in.WindowStart
					if target.FirstMatchAt.After(start) {
						start = target.FirstMatchAt
					}
					if windowKind == "THREE_DAYS" && in.WindowEnd.Add(-72*time.Hour).After(start) {
						start = in.WindowEnd.Add(-72 * time.Hour)
					}
					var result activity.PublishAnalysisResult
					stageErr = workflow.ExecuteActivity(gctx, "Activities.PublishAnalysis", activity.PublishAnalysisInput{Target: target, Cohort: cohort, WindowKind: windowKind, WindowStart: start, WindowEnd: in.WindowEnd}).Get(gctx, &result)
					if stageErr != nil {
						break
					}
				}
				if stageErr != nil {
					break
				}
			}
			results.Send(gctx, stageResult{Key: target.Platform, Err: stageErr})
		})
	}
	return waitStage(ctx, control, results, len(targets), cancelActivities, state)
}

type stageResult struct {
	Key string
	Err error
}

func runPlatformStage(ctx workflow.Context, control workflow.ReceiveChannel, in tftcontract.CrawlInput, runID int64, state *tftcontract.CrawlStatus) (bool, bool, error) {
	state.StageCompleted, state.StageTotal = 0, len(in.Platforms)
	stageCtx, cancelActivities := workflow.WithCancel(ctx)
	results := workflow.NewBufferedChannel(ctx, len(in.Platforms))
	for _, platformValue := range in.Platforms {
		platform := strings.ToUpper(platformValue)
		workflow.Go(stageCtx, func(gctx workflow.Context) {
			childCtx := workflow.WithChildOptions(gctx, workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("tft-platform-%d-%s", runID, strings.ToLower(platform)),
				TaskQueue:  tftcontract.SeedTaskQueue, ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_TERMINATE,
			})
			err := workflow.ExecuteChildWorkflow(childCtx, PlatformSeed, tftcontract.PlatformInput{CrawlInput: in, RunID: runID, Platform: platform}).Get(gctx, nil)
			results.Send(gctx, stageResult{Key: platform, Err: err})
		})
	}
	return waitStage(ctx, control, results, len(in.Platforms), cancelActivities, state)
}

func PlatformSeed(ctx workflow.Context, in tftcontract.PlatformInput) error {
	return seedAndDiscover(withCrawlActivityOptions(ctx), in.CrawlInput, in.RunID, strings.ToUpper(in.Platform))
}

func seedAndDiscover(ctx workflow.Context, in tftcontract.CrawlInput, runID int64, platform string) error {
	for _, tier := range []string{"CHALLENGER", "GRANDMASTER", "MASTER"} {
		var count int
		if err := workflow.ExecuteActivity(ctx, "Activities.FetchTopTier", activity.SeedTierInput{RunID: runID, Platform: platform, Tier: tier}).Get(ctx, &count); err != nil {
			return fmt.Errorf("seed %s %s: %w", platform, tier, err)
		}
	}
	for _, division := range []string{"I", "II", "III", "IV"} {
		for page := 1; page <= maxDiamondPages; page++ {
			var result activity.PageResult
			if err := workflow.ExecuteActivity(ctx, "Activities.FetchDiamondPage", activity.DiamondPageInput{RunID: runID, Platform: platform, Division: division, Page: page}).Get(ctx, &result); err != nil {
				return fmt.Errorf("seed %s diamond %s page %d: %w", platform, division, page, err)
			}
			if !result.HasMore {
				break
			}
			if page == maxDiamondPages {
				return temporal.NewNonRetryableApplicationError("TFT Diamond pagination exceeded safety bound", "PAGINATION_INVARIANT", nil, platform, division)
			}
		}
	}
	var selected int
	if err := workflow.ExecuteActivity(ctx, "Activities.ApplySampling", activity.ApplySamplingInput{RunID: runID, Platform: platform, Patch: in.Patch}).Get(ctx, &selected); err != nil {
		return err
	}
	for offset := 0; ; offset += activityBatchSize {
		var result activity.BatchResult
		if err := workflow.ExecuteActivity(ctx, "Activities.DiscoverMatches", activity.DiscoverInput{RunID: runID, Platform: platform, Offset: offset, Limit: activityBatchSize, WindowStart: in.WindowStart, WindowEnd: in.WindowEnd}).Get(ctx, &result); err != nil {
			return err
		}
		if !result.HasMore {
			break
		}
	}
	return nil
}

func runRouteStage(ctx workflow.Context, control workflow.ReceiveChannel, runID int64, patch string, state *tftcontract.CrawlStatus) (bool, bool, error) {
	routes := []string{"AMERICAS", "ASIA", "EUROPE", "SEA"}
	state.StageCompleted, state.StageTotal = 0, len(routes)
	stageCtx, cancelActivities := workflow.WithCancel(ctx)
	results := workflow.NewBufferedChannel(ctx, len(routes))
	for _, routeValue := range routes {
		route := routeValue
		workflow.Go(stageCtx, func(gctx workflow.Context) {
			childCtx := workflow.WithChildOptions(gctx, workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("tft-route-%d-%s", runID, strings.ToLower(route)),
				TaskQueue:  tftcontract.SeedTaskQueue, ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_TERMINATE,
			})
			stageErr := workflow.ExecuteChildWorkflow(childCtx, RouteDispatch, tftcontract.RouteInput{RunID: runID, RoutingRegion: route, Patch: patch}).Get(gctx, nil)
			results.Send(gctx, stageResult{Key: route, Err: stageErr})
		})
	}
	return waitStage(ctx, control, results, len(routes), cancelActivities, state)
}

func RouteDispatch(ctx workflow.Context, in tftcontract.RouteInput) error {
	base := withCrawlActivityOptions(ctx)
	routeCtx := workflow.WithTaskQueue(base, tftcontract.MatchTaskQueue(strings.ToUpper(in.RoutingRegion)))
	for batches := 0; ; batches++ {
		var result activity.BatchResult
		if err := workflow.ExecuteActivity(routeCtx, "Activities.DispatchMatches", activity.DispatchInput{RunID: in.RunID, RoutingRegion: in.RoutingRegion, TargetPatch: in.Patch, Limit: activityBatchSize}).Get(routeCtx, &result); err != nil {
			return err
		}
		if !result.HasMore {
			return nil
		}
		if !result.NextEligibleAt.IsZero() {
			wait := result.NextEligibleAt.Sub(workflow.Now(ctx))
			if wait > 0 {
				if err := workflow.NewTimer(ctx, wait).Get(ctx, nil); err != nil {
					return err
				}
			}
		}
		if batches >= 499 {
			return workflow.NewContinueAsNewError(ctx, RouteDispatch, in)
		}
	}
}

func withCrawlActivityOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: time.Minute, MaximumAttempts: 5},
	})
}

func waitStage(ctx workflow.Context, control, results workflow.ReceiveChannel, total int, cancel workflow.CancelFunc, state *tftcontract.CrawlStatus) (bool, bool, error) {
	settled := 0
	pause, cancelled := false, false
	var firstErr error
	for settled < total {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(results, func(ch workflow.ReceiveChannel, _ bool) {
			var result stageResult
			ch.Receive(ctx, &result)
			settled++
			if result.Err == nil {
				state.StageCompleted++
			} else if !pause && !cancelled {
				if firstErr == nil || isApplicationErrorType(result.Err, "RIOT_AUTH") {
					firstErr = result.Err
				}
				cancel()
			}
		})
		selector.AddReceive(control, func(ch workflow.ReceiveChannel, _ bool) {
			var command tftcontract.ControlCommand
			ch.Receive(ctx, &command)
			switch strings.ToLower(command.Action) {
			case "pause":
				pause = true
				state.PausePending = true
				cancel()
			case "cancel":
				cancelled = true
				cancel()
			}
		})
		selector.Select(ctx)
	}
	return pause, cancelled, firstErr
}

func waitPaused(ctx workflow.Context, control workflow.ReceiveChannel, runID int64, state *tftcontract.CrawlStatus) (bool, bool, error) {
	stateCtx := stateActivityContext(ctx)
	state.State, state.PausePending = "paused", false
	if err := workflow.ExecuteActivity(stateCtx, "Activities.SetRunState", activity.RunStateInput{RunID: runID, Status: "paused", DesiredState: "paused", Stage: state.Stage}).Get(stateCtx, nil); err != nil {
		return false, false, err
	}
	for {
		var command tftcontract.ControlCommand
		control.Receive(ctx, &command)
		switch strings.ToLower(command.Action) {
		case "resume":
			if err := workflow.ExecuteActivity(stateCtx, "Activities.SetRunState", activity.RunStateInput{RunID: runID, Status: "running", DesiredState: "running", Stage: state.Stage}).Get(stateCtx, nil); err != nil {
				return false, false, err
			}
			state.State = "running"
			return true, false, nil
		case "cancel":
			return false, true, nil
		}
	}
}

func waitAuth(ctx workflow.Context, control workflow.ReceiveChannel, runID int64, state *tftcontract.CrawlStatus, cause error) (bool, bool, error) {
	stateCtx := stateActivityContext(ctx)
	state.State, state.PausePending = "paused_needs_auth", false
	if err := workflow.ExecuteActivity(stateCtx, "Activities.SetRunState", activity.RunStateInput{RunID: runID, Status: "paused_needs_auth", DesiredState: "paused", Stage: state.Stage, LastError: cause.Error()}).Get(stateCtx, nil); err != nil {
		return false, false, err
	}
	for {
		var command tftcontract.ControlCommand
		control.Receive(ctx, &command)
		switch strings.ToLower(command.Action) {
		case "resume":
			if err := workflow.ExecuteActivity(stateCtx, "Activities.SetRunState", activity.RunStateInput{RunID: runID, Status: "running", DesiredState: "running", Stage: state.Stage}).Get(stateCtx, nil); err != nil {
				return false, false, err
			}
			state.State = "running"
			return true, false, nil
		case "cancel":
			return false, true, nil
		}
	}
}

func handleAuthError(ctx workflow.Context, control workflow.ReceiveChannel, runID int64, state *tftcontract.CrawlStatus, cause error) (bool, bool, bool, error) {
	if !isApplicationErrorType(cause, "RIOT_AUTH") {
		return false, false, false, nil
	}
	resume, cancel, err := waitAuth(ctx, control, runID, state, cause)
	return resume, cancel, true, err
}

func isApplicationErrorType(err error, wanted string) bool {
	var appErr *temporal.ApplicationError
	return errors.As(err, &appErr) && appErr.Type() == wanted
}

func setTerminal(ctx workflow.Context, runID int64, status, stage string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	stateCtx := stateActivityContext(ctx)
	return workflow.ExecuteActivity(stateCtx, "Activities.SetRunState", activity.RunStateInput{RunID: runID, Status: status, DesiredState: map[bool]string{true: "cancelled", false: "running"}[status == "cancelled"], Stage: stage, LastError: message}).Get(stateCtx, nil)
}

func failRun(ctx workflow.Context, runID int64, stage string, cause error) error {
	if persistErr := setTerminal(ctx, runID, "failed", stage, cause); persistErr != nil {
		return errors.Join(cause, persistErr)
	}
	return cause
}

func stateActivityContext(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2,
			MaximumInterval: time.Minute, MaximumAttempts: 0,
		},
	})
}
