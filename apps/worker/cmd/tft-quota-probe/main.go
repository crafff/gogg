// Command tft-quota-probe performs a deliberately low-rate observation of
// Riot's LoL and TFT application counters on the same regional route. It does
// not attempt to exhaust a bucket or provoke HTTP 429 responses.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/tft/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tft-quota-probe: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("tft-quota-probe", flag.ContinueOnError)
	platform := flags.String("platform", "KR", "TFT platform route")
	puuid := flags.String("puuid", "", "PUUID used for one-match list probes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	apiKey := strings.TrimSpace(os.Getenv("RIOT_API_KEY"))
	if apiKey == "" {
		return fmt.Errorf("RIOT_API_KEY is required")
	}
	if strings.TrimSpace(*puuid) == "" {
		return fmt.Errorf("--puuid is required")
	}
	region := config.RoutingRegion(*platform)
	if region == "" {
		return fmt.Errorf("unsupported platform %q", *platform)
	}
	base := "https://" + strings.ToLower(region) + ".api.riotgames.com"
	paths := []struct {
		product string
		path    string
	}{
		{"lol", "/lol/match/v5/matches/by-puuid/" + url.PathEscape(*puuid) + "/ids?start=0&count=1"},
		{"tft", "/tft/match/v1/matches/by-puuid/" + url.PathEscape(*puuid) + "/ids?start=0&count=1"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}
	for _, probe := range paths {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+probe.path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Riot-Token", apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("%s regional probe: %w", probe.product, err)
		}
		_ = resp.Body.Close()
		fmt.Printf("product=%s route=%s status=%d app_limit=%q app_count=%q method_limit=%q method_count=%q rate_type=%q retry_after=%q\n",
			probe.product, region, resp.StatusCode,
			resp.Header.Get("X-App-Rate-Limit"), resp.Header.Get("X-App-Rate-Limit-Count"),
			resp.Header.Get("X-Method-Rate-Limit"), resp.Header.Get("X-Method-Rate-Limit-Count"),
			resp.Header.Get("X-Rate-Limit-Type"), resp.Header.Get("Retry-After"))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("Riot rejected the credential with HTTP %d", resp.StatusCode)
		}
	}
	return nil
}
