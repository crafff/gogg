// crawler-lite runs the crawler without Temporal. It stores coarse
// progress in the existing runs row and resumes by re-running the
// current phase.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/config"
	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase0"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase1"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase2"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase3"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase35"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase4"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase5"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase55"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
	"github.com/crafff/gogg/apps/worker/internal/runtime"
	"github.com/crafff/gogg/apps/worker/internal/storage"
)

var phaseOrder = []int{0, 1, 2, 3, 35, 4, 5, 55}
var perTierPhaseOrder = []int{2, 3, 35, 4, 5, 55}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "crawler-lite: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return usage()
	}
	switch os.Args[1] {
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		profile := fs.String("profile", "", "run profile name")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *profile == "" {
			return fmt.Errorf("--profile is required")
		}
		return runProfile(*profile)
	case "resume":
		fs := flag.NewFlagSet("resume", flag.ExitOnError)
		runID := fs.Int("run-id", 0, "lite run id to resume")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *runID == 0 {
			return fmt.Errorf("--run-id is required")
		}
		return resumeRun(*runID)
	case "list-runs":
		fs := flag.NewFlagSet("list-runs", flag.ExitOnError)
		limit := fs.Int("limit", 20, "number of runs")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		return listRuns(*limit)
	case "show-run":
		fs := flag.NewFlagSet("show-run", flag.ExitOnError)
		runID := fs.Int("run-id", 0, "run id")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *runID == 0 {
			return fmt.Errorf("--run-id is required")
		}
		return showRun(*runID)
	default:
		return usage()
	}
}

func usage() error {
	return fmt.Errorf("usage: crawler-lite run --profile <name> | resume --run-id <id> | list-runs | show-run --run-id <id>")
}

func runProfile(profileName string) error {
	ctx, rt, err := boot()
	if err != nil {
		return err
	}
	defer rt.Close()

	p, err := rt.Cfg.Profile(profileName)
	if err != nil {
		return err
	}
	lastRunEnd := rt.Store.GetLastCompletedRunEnd(ctx, p.Region)
	state, err := crawler.NewLiteRunState(ctx, rt.Store, &profileName, p, lastRunEnd)
	if err != nil {
		return err
	}
	return execute(ctx, rt, state, 0, "", "")
}

func resumeRun(runID int) error {
	ctx, rt, err := boot()
	if err != nil {
		return err
	}
	defer rt.Close()

	run, err := rt.Store.GetRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run %d not found", runID)
	}
	if run.RunnerType != "lite" {
		return fmt.Errorf("run %d is runner_type=%q; crawler-lite only resumes lite runs", runID, run.RunnerType)
	}
	if run.Status == "completed" {
		return fmt.Errorf("run %d is already completed", runID)
	}
	profile := profileFromRun(run)
	if err := rt.Store.ReactivateRun(ctx, runID); err != nil {
		return err
	}
	state := crawler.ResumeRunState(run, profile, rt.Store, nil)
	tier, division := "", ""
	if run.CurrentTier != nil {
		tier = *run.CurrentTier
	}
	if run.CurrentDivision != nil {
		division = *run.CurrentDivision
	}
	return execute(ctx, rt, state, run.CurrentPhase, tier, division)
}

func listRuns(limit int) error {
	ctx, rt, err := boot()
	if err != nil {
		return err
	}
	defer rt.Close()
	runs, err := rt.Store.ListRuns(ctx, limit)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tRUNNER\tSTATUS\tPROFILE\tREGION\tVERSION\tEXECUTION\tPHASE\tTIER\tDIV\tUPDATED\tERROR")
	for _, r := range runs {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ID, r.RunnerType, r.Status, value(r.Profile), r.Region, value(r.Version), r.Execution,
			phaseLabel(r.CurrentPhase), value(r.CurrentTier), value(r.CurrentDivision),
			r.UpdatedAt.Format("2006-01-02 15:04:05"), value(r.LastError))
	}
	return w.Flush()
}

func showRun(runID int) error {
	ctx, rt, err := boot()
	if err != nil {
		return err
	}
	defer rt.Close()
	r, err := rt.Store.GetRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("run %d not found", runID)
	}
	fmt.Printf("id: %d\nrunner: %s\nstatus: %s\nprofile: %s\nregion: %s\nversion: %s\nexecution: %s\nphase: %s\ntier: %s\ndivision: %s\nstarted_at: %s\nupdated_at: %s\nlast_error: %s\n",
		r.ID, r.RunnerType, r.Status, value(r.Profile), r.Region, value(r.Version), r.Execution,
		phaseLabel(r.CurrentPhase), value(r.CurrentTier), value(r.CurrentDivision),
		r.StartedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339), value(r.LastError))
	return nil
}

func boot() (context.Context, *runtime.Runtime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if cfg.Logging.Format == "text" {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	rt, err := runtime.Build(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build runtime: %w", err)
	}
	return ctx, rt, nil
}

func execute(ctx context.Context, rt *runtime.Runtime, state *crawler.RunState, startPhase int, startTier, startDivision string) error {
	phases, err := buildPhases(rt, state.Region())
	if err != nil {
		return err
	}
	if startTier != "" {
		state.CurrentTier = startTier
	}
	if startDivision != "" {
		state.CurrentDivision = startDivision
	}

	err = executeByMode(ctx, phases, state, startPhase)
	statusCtx, cancelStatus := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelStatus()
	switch {
	case err == nil:
		return state.Complete(statusCtx)
	case errors.Is(err, context.Canceled):
		if pauseErr := rt.Store.MarkRunPaused(statusCtx, state.ID); pauseErr != nil {
			return fmt.Errorf("mark paused after cancel: %w", pauseErr)
		}
		slog.Info("lite_run_paused", "run_id", state.ID)
		return nil
	default:
		_ = rt.Store.FailRunWithError(statusCtx, state.ID, err.Error())
		return err
	}
}

func executeByMode(ctx context.Context, phases map[int]crawler.Phase, state *crawler.RunState, startPhase int) error {
	if state.Profile.Execution != config.ExecutionPipeline {
		return executeSequential(ctx, phases, state, startPhase)
	}
	return executePipeline(ctx, phases, state, startPhase)
}

func executeSequential(ctx context.Context, phases map[int]crawler.Phase, state *crawler.RunState, startPhase int) error {
	started := startPhase == 0
	for _, id := range phaseOrder {
		if !started {
			if id != startPhase {
				continue
			}
			started = true
		}
		if err := executePhase(ctx, phases[id], state); err != nil {
			return err
		}
	}
	return nil
}

func executePipeline(ctx context.Context, phases map[int]crawler.Phase, state *crawler.RunState, startPhase int) error {
	if startPhase <= 0 {
		if err := executePhase(ctx, phases[0], state); err != nil {
			return err
		}
		startPhase = 1
	}
	if startPhase <= 1 {
		if err := executePhase(ctx, phases[1], state); err != nil {
			return err
		}
		startPhase = 2
		state.CurrentTier = ""
		state.CurrentDivision = ""
	}

	tierStarted := state.CurrentTier == ""
	for _, tier := range state.Profile.TargetTiers {
		resumeTier := state.CurrentTier
		if !tierStarted {
			if tier != resumeTier {
				continue
			}
			tierStarted = true
		}
		state.CurrentTier = tier
		phaselog.Step(phaselog.Meta{RunID: state.ID, Region: state.Region(), Tier: tier}, "pipeline_tier_started", "runner", "lite")
		phaseStarted := startPhase <= 2 || tier != resumeTier
		for _, id := range perTierPhaseOrder {
			if !phaseStarted {
				if id != startPhase {
					continue
				}
				phaseStarted = true
			}
			if err := executePhase(ctx, phases[id], state); err != nil {
				return err
			}
		}
		phaselog.Step(phaselog.Meta{RunID: state.ID, Region: state.Region(), Tier: tier}, "pipeline_tier_completed", "runner", "lite")
		startPhase = 2
		state.CurrentDivision = ""
	}
	return nil
}

func executePhase(ctx context.Context, p crawler.Phase, state *crawler.RunState) error {
	if p == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var tierPtr *string
	if state.CurrentTier != "" {
		t := state.CurrentTier
		tierPtr = &t
	}
	var divPtr *string
	if state.CurrentDivision != "" {
		d := state.CurrentDivision
		divPtr = &d
	}
	if err := state.SaveCheckpointDetail(ctx, p.ID(), tierPtr, divPtr); err != nil {
		return err
	}
	done, err := p.IsDone(ctx, state)
	if err != nil {
		return err
	}
	if done {
		phaselog.Skipped(litePhaseMeta(state, p), "already_done", "runner", "lite")
		return nil
	}
	phaselog.Started(litePhaseMeta(state, p), "scope", "runner", "runner", "lite")
	if err := p.Run(ctx, state); err != nil {
		return err
	}
	phaselog.Completed(litePhaseMeta(state, p), "scope", "runner", "runner", "lite")
	return nil
}

func litePhaseMeta(state *crawler.RunState, p crawler.Phase) phaselog.Meta {
	version := ""
	queue := ""
	if state.Profile != nil {
		version = state.Profile.Version
		queue = state.Profile.Queue
	}
	return phaselog.Meta{
		RunID:    state.ID,
		Region:   state.Region(),
		Phase:    p.Name(),
		PhaseID:  p.ID(),
		Version:  version,
		Tier:     state.CurrentTier,
		Division: state.CurrentDivision,
		Queue:    queue,
	}
}

func buildPhases(rt *runtime.Runtime, region string) (map[int]crawler.Phase, error) {
	riot, err := rt.RiotForRegion(region)
	if err != nil {
		return nil, err
	}
	return map[int]crawler.Phase{
		0:  phase0.New(riot, rt.Store),
		1:  phase1.New(riot, rt.Store),
		2:  phase2.New(riot, rt.Store),
		3:  phase3.New(riot, rt.Store),
		35: phase35.New(riot, rt.Store),
		4:  phase4.New(rt.Store),
		5:  phase5.New(riot, rt.Store),
		55: phase55.New(rt.Store),
	}, nil
}

func profileFromRun(r *storage.Run) *config.RunProfile {
	version := ""
	if r.Version != nil {
		version = *r.Version
	}
	return &config.RunProfile{
		Region:            r.Region,
		Mode:              config.Mode(r.Mode),
		TargetTiers:       r.TargetTiers,
		RankPrefetchTiers: r.RankPrefetchTiers,
		Queue:             r.Queue,
		Execution:         config.Execution(r.Execution),
		Version:           version,
	}
}

func value(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}

func phaseLabel(phase int) string {
	if phase == 0 {
		return "phase0"
	}
	if phase == 35 {
		return "phase3.5"
	}
	return "phase" + strconv.Itoa(phase)
}
