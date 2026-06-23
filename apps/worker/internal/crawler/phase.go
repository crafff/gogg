package crawler

import "context"

// Phase represents one stage of the crawler pipeline.
type Phase interface {
	ID() int
	Name() string
	// Run executes the phase. It should respect ctx cancellation between operations.
	Run(ctx context.Context, state *RunState) error
	// IsDone returns true if this phase was already completed for the given run state.
	IsDone(ctx context.Context, state *RunState) (bool, error)
}
