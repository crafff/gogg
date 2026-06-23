package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadUnifiedConfig(t *testing.T) {
	path := writeConfig(t, `
temporal:
  host_port: localhost:7233
  namespace: default
  task_queues: [crawl-kr]
logging:
  level: info
  format: json
riot:
  api_key: RGAPI-test
regions:
  - name: KR
    base_url: https://kr.api.riotgames.com
database:
  dsn: postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable
schedule:
  - cron: "0 4 * * *"
    profile: daily_kr
run_profiles:
  daily_kr:
    region: KR
    mode: incremental
    target_tiers: [CHALLENGER]
    queue: RANKED_SOLO_5x5
    execution: pipeline
`)
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "localhost:7233", cfg.Temporal.HostPort)
	require.Equal(t, "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable", cfg.Database.DSN)
	require.Len(t, cfg.Schedule, 1)

	profile, err := cfg.Profile("daily_kr")
	require.NoError(t, err)
	require.Equal(t, "KR", profile.Region)
}

func TestResolvedRegionsInheritGlobalRiotKey(t *testing.T) {
	cfg := Config{
		Riot: RiotConfig{APIKey: "RGAPI-test"},
		Regions: []RegionConfig{
			{Name: "KR", BaseURL: "https://kr.api.riotgames.com"},
			{Name: "NA1", APIKey: "RGAPI-na", BaseURL: "https://na1.api.riotgames.com"},
		},
	}

	regions := cfg.ResolvedRegions()
	require.Equal(t, "RGAPI-test", regions[0].APIKey)
	require.Equal(t, "RGAPI-na", regions[1].APIKey)
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worker.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}
