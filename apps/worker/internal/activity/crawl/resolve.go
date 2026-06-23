package crawl

import (
	"context"
	"fmt"
)

// ResolveProfileInput names the run_profiles map key. The
// caller (CLI / scheduler) can supply just this and let the worker
// translate it into the full profile that drives the workflow.
type ResolveProfileInput struct {
	ProfileName string `json:"profile_name"`
}

// ProfileSnapshot is the JSON-shaped projection of a configured run
// profile that flows through the workflow.
type ProfileSnapshot struct {
	Region            string   `json:"region"`
	Mode              string   `json:"mode"`
	Version           string   `json:"version,omitempty"`
	TargetTiers       []string `json:"target_tiers"`
	RankPrefetchTiers []string `json:"rank_prefetch_tiers"`
	Queue             string   `json:"queue"`
	Execution         string   `json:"execution"`
}

// ResolveProfile looks up a named profile from the crawler config the
// worker loaded at startup.
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
