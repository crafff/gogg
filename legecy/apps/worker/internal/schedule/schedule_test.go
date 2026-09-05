package schedule

import (
	"testing"

	"github.com/stretchr/testify/require"

	config "github.com/crafff/gogg/apps/worker/internal/config"
)

// TestBuildPlan_ResolvesProfileAndQueue verifies schedule entry →
// Temporal Plan translation: schedule rows reference a profile by name;
// the plan needs to resolve the profile, region, and task queue.
func TestBuildPlan_ResolvesProfileAndQueue(t *testing.T) {
	cfg := &config.Config{
		Schedule: []config.ScheduleEntry{
			{Cron: "0 4 * * *", Profile: "daily_kr"},
			{Cron: "30 4 * * *", Profile: "daily_na"},
		},
		Profiles: map[string]config.RunProfile{
			"daily_kr": {
				Region: "KR", Mode: config.ModeIncremental,
				TargetTiers: []string{"CHALLENGER"},
				Queue:       "RANKED_SOLO_5x5",
			},
			"daily_na": {
				Region: "NA1", Mode: config.ModeIncremental,
				TargetTiers: []string{"CHALLENGER"},
				Queue:       "RANKED_SOLO_5x5",
			},
		},
	}

	plans, err := BuildPlan(cfg)
	require.NoError(t, err)
	require.Len(t, plans, 2)

	require.Equal(t, "gogg-crawl-daily_kr", plans[0].ID)
	require.Equal(t, "crawl-kr", plans[0].TaskQueue)
	require.Equal(t, "0 4 * * *", plans[0].Cron)
	require.Equal(t, "daily_kr", plans[0].Input.ProfileName)
	require.Equal(t, "KR", plans[0].Input.Profile.Region)

	require.Equal(t, "gogg-crawl-daily_na", plans[1].ID)
	require.Equal(t, "crawl-na1", plans[1].TaskQueue)
	require.Equal(t, "daily_na", plans[1].Input.ProfileName)
	require.Equal(t, "NA1", plans[1].Input.Profile.Region)
}

// TestBuildPlan_RejectsUnknownProfile catches the typo case — a
// schedule referencing a profile that doesn't exist must fail loudly
// at worker startup, not at the first scheduled fire.
func TestBuildPlan_RejectsUnknownProfile(t *testing.T) {
	cfg := &config.Config{
		Schedule: []config.ScheduleEntry{
			{Cron: "0 4 * * *", Profile: "does_not_exist"},
		},
		Profiles: map[string]config.RunProfile{},
	}

	_, err := BuildPlan(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does_not_exist")
}

// TestBuildPlan_RejectsEmptyCron and TestBuildPlan_RejectsEmptyProfile
// are the obvious shape checks that ensure malformed YAML fails fast.
func TestBuildPlan_RejectsEmptyCron(t *testing.T) {
	cfg := &config.Config{
		Schedule: []config.ScheduleEntry{{Cron: "", Profile: "daily_kr"}},
	}
	_, err := BuildPlan(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cron is empty")
}

func TestBuildPlan_RejectsEmptyProfile(t *testing.T) {
	cfg := &config.Config{
		Schedule: []config.ScheduleEntry{{Cron: "0 4 * * *", Profile: ""}},
	}
	_, err := BuildPlan(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "profile is empty")
}
