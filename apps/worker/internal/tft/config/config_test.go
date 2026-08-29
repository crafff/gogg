package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRequiresEnglishStaticCatalog(t *testing.T) {
	cfg := Default()
	cfg.Riot.APIKey = "test-key"
	cfg.TFT.StaticLocales = []string{"zh_cn"}

	require.ErrorContains(t, cfg.Validate(), "tft.static_locales must include en_us")
}
