package crawl

import (
	"context"
	"fmt"

	legacycfg "github.com/crafff/gogg/internal/config"
)

// ResolveProfileInput names the legacy run_profiles map key. The
// caller (CLI / scheduler) can supply just this and let the worker
// translate it into the full profile that drives the workflow.
type ResolveProfileInput struct {
	ProfileName string `json:"profile_name"`
}

// ProfileSnapshot is the JSON-shaped projection of legacycfg.RunProfile
// that flows through the workflow. We don't pass legacycfg types
// directly because that's still package-private mapstructure-decorated.
type ProfileSnapshot struct {
	Region            string   `json:"region"`
	Mode              string   `json:"mode"`
	Version           string   `json:"version,omitempty"`
	TargetTiers       []string `json:"target_tiers"`
	RankPrefetchTiers []string `json:"rank_prefetch_tiers"`
	Queue             string   `json:"queue"`
	Execution         string   `json:"execution"`
}

// ResolveProfile looks up a named profile from the legacy config the
// worker loaded at startup. Idempotent + deterministic given the
// config snapshot, so safe to invoke from a Workflow.
func (a *Activities) ResolveProfile(_ context.Context, in ResolveProfileInput) (ProfileSnapshot, error) {
	rp, err := a.rt.Cfg.Profile(in.ProfileName)
	if err != nil {
		return ProfileSnapshot{}, fmt.Errorf("resolve profile %q: %w", in.ProfileName, err)
	}
	if err := rp.Validate(); err != nil {
		return ProfileSnapshot{}, fmt.Errorf("profile %q invalid: %w", in.ProfileName, err)
	}
	return ProfileSnapshot{
		Region:            rp.Region,
		Mode:              string(rp.Mode),
		Version:           rp.Version,
		TargetTiers:       rp.TargetTiers,
		RankPrefetchTiers: rp.RankPrefetchTiers,
		Queue:             rp.Queue,
		Execution:         string(rp.Execution),
	}, nil
}

// SilenceLegacyImport keeps the legacy import path live so that a
// future rename in internal/config surfaces here at compile time and
// we don't quietly drift. Removed when packages/domain/profile lands
// in chunk 4.
var _ = legacycfg.ModeIncremental
