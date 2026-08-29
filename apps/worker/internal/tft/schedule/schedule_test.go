package schedule

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crafff/gogg/apps/worker/internal/tft/config"
	"github.com/crafff/gogg/packages/tftcontract"
)

func TestBuildPlansUsesGlobalPlatformsAndIsolatedQueue(t *testing.T) {
	cfg := config.Default()
	cfg.Riot.APIKey = "test"
	plans, err := BuildPlans(cfg)
	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.Equal(t, tftcontract.DefaultScheduleID, plans[0].ID)
	require.Equal(t, tftcontract.SeedTaskQueue, plans[0].TaskQueue)
	require.Equal(t, tftcontract.DefaultStaticScheduleID, plans[1].ID)
	require.Equal(t, tftcontract.StaticTaskQueue, plans[1].TaskQueue)
}
