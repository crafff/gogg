package crawler

import (
	"context"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
)

// ExecutionStrategy determines how phases are ordered and grouped.
type ExecutionStrategy interface {
	Execute(ctx context.Context, phases []Phase, state *RunState) error
}

// SequentialStrategy runs each phase to completion before starting the next.
// Order: Phase0 → Phase1(all tiers) → Phase2(all puuids) → Phase3 → Phase35 → Phase4
type SequentialStrategy struct{}

func (s *SequentialStrategy) Execute(ctx context.Context, phases []Phase, state *RunState) error {
	for _, p := range phases {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := runPhase(ctx, p, state); err != nil {
			return err
		}
	}
	return nil
}

// PipelineStrategy runs Phase0, then Phase1 for all prefetch tiers, then for each
// target tier runs Phase2→Phase3 sequentially.
// Phase35 and Phase4 run once at the end after all tiers are processed.
type PipelineStrategy struct {
	// phase IDs of per-tier phases (Phase2, Phase3)
	PerTierPhaseIDs []int
}

func NewPipelineStrategy() *PipelineStrategy {
	return &PipelineStrategy{PerTierPhaseIDs: []int{2, 3, 35, 4, 5, 55}}
}

func (s *PipelineStrategy) Execute(ctx context.Context, phases []Phase, state *RunState) error {
	byID := make(map[int]Phase, len(phases))
	for _, p := range phases {
		byID[p.ID()] = p
	}

	// Phase 0 always runs first
	if p, ok := byID[0]; ok {
		if err := runPhase(ctx, p, state); err != nil {
			return err
		}
	}

	// Phase 1 runs for all prefetch tiers upfront
	if p, ok := byID[1]; ok {
		if err := runPhase(ctx, p, state); err != nil {
			return err
		}
	}

	// Phase 2 → 3 → 35 → 4 per target tier
	for _, tier := range state.Profile.TargetTiers {
		if err := ctx.Err(); err != nil {
			return err
		}
		phaselog.Step(phaselog.Meta{RunID: state.ID, Region: state.Region(), Tier: tier}, "pipeline_tier_started")
		state.CurrentTier = tier
		tierCopy := tier
		if err := state.SaveCheckpoint(ctx, s.PerTierPhaseIDs[0], &tierCopy); err != nil {
			return err
		}
		for _, id := range s.PerTierPhaseIDs {
			if p, ok := byID[id]; ok {
				if err := runPhase(ctx, p, state); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runPhase(ctx context.Context, p Phase, state *RunState) error {
	if state.donePhases[p.ID()] {
		phaselog.Skipped(phaseMeta(state, p, ""), "prior_run")
		return nil
	}
	done, err := p.IsDone(ctx, state)
	if err != nil {
		return err
	}
	if done {
		phaselog.Skipped(phaseMeta(state, p, ""), "already_done")
		return nil
	}
	var tier *string
	if state.CurrentTier != "" {
		t := state.CurrentTier
		tier = &t
	}
	if err := state.SaveCheckpoint(ctx, p.ID(), tier); err != nil {
		return err
	}
	tierValue := ""
	if tier != nil {
		tierValue = *tier
	}
	phaselog.Started(phaseMeta(state, p, tierValue), "scope", "runner")
	if err := p.Run(ctx, state); err != nil {
		return err
	}
	phaselog.Completed(phaseMeta(state, p, tierValue), "scope", "runner")
	return nil
}

func phaseMeta(state *RunState, p Phase, tier string) phaselog.Meta {
	version := ""
	if state.Profile != nil {
		version = state.Profile.Version
	}
	return phaselog.Meta{
		RunID:   state.ID,
		Region:  state.Region(),
		Phase:   p.Name(),
		PhaseID: p.ID(),
		Version: version,
		Tier:    tier,
	}
}
