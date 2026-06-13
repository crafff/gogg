package config

import "fmt"

type Mode string

const (
	ModeIncremental Mode = "incremental"
	ModeHistorical  Mode = "historical"
)

type Execution string

const (
	ExecutionPipeline   Execution = "pipeline"
	ExecutionSequential Execution = "sequential"
)

// RunProfile defines a complete set of parameters for one crawler run.
type RunProfile struct {
	Region            string    `mapstructure:"region"`
	Mode              Mode      `mapstructure:"mode"`
	Version           string    `mapstructure:"version"`
	TargetTiers       []string  `mapstructure:"target_tiers"`
	RankPrefetchTiers []string  `mapstructure:"rank_prefetch_tiers"`
	Queue             string    `mapstructure:"queue"`
	Execution         Execution `mapstructure:"execution"`
}

func (p *RunProfile) Validate() error {
	if p.Region == "" {
		return fmt.Errorf("region must be set (e.g. KR, NA1, EUW1)")
	}
	if p.Mode == "" {
		p.Mode = ModeIncremental
	}
	if p.Mode != ModeIncremental && p.Mode != ModeHistorical {
		return fmt.Errorf("invalid mode %q, must be incremental or historical", p.Mode)
	}
	if p.Mode == ModeHistorical && p.Version == "" {
		return fmt.Errorf("mode=historical requires version to be set")
	}
	if len(p.TargetTiers) == 0 {
		return fmt.Errorf("target_tiers must not be empty")
	}
	if p.Queue == "" {
		p.Queue = "RANKED_SOLO_5x5"
	}
	if p.Execution == "" {
		p.Execution = ExecutionPipeline
	}
	if len(p.RankPrefetchTiers) == 0 {
		p.RankPrefetchTiers = p.TargetTiers
	}
	return nil
}

// MergeFlags applies CLI flag overrides onto the profile (non-zero values win).
func (p *RunProfile) MergeFlags(targetTiers []string, mode Mode, version string, execution Execution, region string) {
	if len(targetTiers) > 0 {
		p.TargetTiers = targetTiers
	}
	if mode != "" {
		p.Mode = mode
	}
	if version != "" {
		p.Version = version
	}
	if execution != "" {
		p.Execution = execution
	}
	if region != "" {
		p.Region = region
	}
}