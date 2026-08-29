package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadStorageRootsFromEnvironment(t *testing.T) {
	t.Setenv("APP_CONFIG_PATH", "")
	t.Setenv("GOGG_RIOT_API_KEY", "test-key")
	t.Setenv("GOGG_RAW_ARCHIVE_ROOT", "/mnt/gogg-db/riot-raw")
	t.Setenv("GOGG_TFT_STATIC_ROOT", "/mnt/gogg-db/game-assets")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "/mnt/gogg-db/riot-raw", cfg.Raw.Root)
	require.Equal(t, "/mnt/gogg-db/game-assets", cfg.TFT.StaticRoot)
}

func TestValidateRequiresEnglishStaticCatalog(t *testing.T) {
	cfg := Default()
	cfg.Riot.APIKey = "test-key"
	cfg.TFT.StaticLocales = []string{"zh_cn"}

	require.ErrorContains(t, cfg.Validate(), "tft.static_locales must include en_us")
}

func TestValidateBoundsStaticDownloadConcurrency(t *testing.T) {
	cfg := Default()
	cfg.Riot.APIKey = "test-key"
	cfg.TFT.StaticDownloadBatch = 0
	cfg.TFT.StaticDownloadWorkers = 65

	err := cfg.Validate()
	require.ErrorContains(t, err, "tft.static_download_batch must be 1..1000")
	require.ErrorContains(t, err, "tft.static_download_workers must be 1..64")
}

func TestValidateRequiresSafeStaticDownloadBatchRatio(t *testing.T) {
	cfg := Default()
	cfg.Riot.APIKey = "test-key"
	cfg.TFT.StaticDownloadBatch = 9
	cfg.TFT.StaticDownloadWorkers = 1

	require.ErrorContains(t, cfg.Validate(), "must not exceed 8 times")
}
