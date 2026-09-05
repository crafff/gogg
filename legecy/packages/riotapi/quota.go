package riotapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QuotaScope identifies every Riot limit dimension touched by one request.
// Product is informational: application limits intentionally omit it because
// Riot counts LoL and TFT together for one credential and routing region.
type QuotaScope struct {
	Credential string
	Product    string
	Route      string
	Method     string
	Service    string
	Family     string
}

// QuotaCoordinator coordinates Riot limits across clients and processes.
// Implementations must fail closed: returning an error from Acquire prevents
// the HTTP request from being sent.
type QuotaCoordinator interface {
	Acquire(context.Context, QuotaScope) error
	Observe(context.Context, QuotaScope, int, http.Header) error
}

// CredentialFingerprint creates a stable non-secret key for quota state.
func CredentialFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:8])
}

func quotaScopeForRequest(apiKey, requestURL string) (QuotaScope, error) {
	u, err := url.Parse(requestURL)
	if err != nil {
		return QuotaScope{}, fmt.Errorf("parse Riot request URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	product := "unknown"
	service := "unknown"
	method := path
	if len(parts) >= 2 {
		product, service = parts[0], parts[1]
	}
	if len(parts) >= 2 {
		method = operationForPath(parts)
	}
	return QuotaScope{
		Credential: CredentialFingerprint(apiKey),
		Product:    product,
		Route:      host,
		Method:     sanitizeQuotaPart(method),
		Service:    sanitizeQuotaPart(service),
		Family:     routingFamily(host),
	}, nil
}

func operationForPath(parts []string) string {
	product, service := parts[0], parts[1]
	if service == "match" {
		for _, part := range parts {
			if part == "by-puuid" {
				return product + "-match-list-by-puuid"
			}
		}
		if len(parts) > 0 && parts[len(parts)-1] == "timeline" {
			return product + "-match-timeline"
		}
		return product + "-match-detail"
	}
	if service == "account" {
		for i, part := range parts {
			if part == "by-riot-id" {
				return product + "-account-by-riot-id"
			}
			if part == "by-puuid" {
				return product + "-account-by-puuid"
			}
			if i > 1 && part == "active-shards" {
				return product + "-account-active-shard"
			}
		}
	}
	if service == "league" && len(parts) >= 4 {
		return product + "-league-" + parts[3]
	}
	if service == "summoner" && len(parts) >= 4 {
		return product + "-summoner-" + parts[3]
	}
	limit := min(len(parts), 4)
	return strings.Join(parts[:limit], "-")
}

func routingFamily(host string) string {
	switch {
	case strings.HasPrefix(host, "americas."), strings.HasPrefix(host, "na1."),
		strings.HasPrefix(host, "br1."), strings.HasPrefix(host, "la1."), strings.HasPrefix(host, "la2."):
		return "americas"
	case strings.HasPrefix(host, "asia."), strings.HasPrefix(host, "kr."), strings.HasPrefix(host, "jp1."):
		return "asia"
	case strings.HasPrefix(host, "europe."), strings.HasPrefix(host, "euw1."), strings.HasPrefix(host, "eun1."),
		strings.HasPrefix(host, "tr1."), strings.HasPrefix(host, "me1."), strings.HasPrefix(host, "ru."):
		return "europe"
	case strings.HasPrefix(host, "sea."), strings.HasPrefix(host, "oc1."), strings.HasPrefix(host, "sg2."),
		strings.HasPrefix(host, "tw2."), strings.HasPrefix(host, "vn2."):
		return "sea"
	default:
		return sanitizeQuotaPart(host)
	}
}

func sanitizeQuotaPart(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	replacer := strings.NewReplacer("/", "_", ":", "_", " ", "_")
	return replacer.Replace(v)
}

type rateWindow struct {
	Limit  int
	Count  int
	Period time.Duration
}

func parseRateWindows(limits, counts string) []rateWindow {
	limitValues := parseRateHeader(limits)
	countValues := parseRateHeader(counts)
	out := make([]rateWindow, 0, len(limitValues))
	for period, limit := range limitValues {
		if limit <= 0 || period <= 0 {
			continue
		}
		count, ok := countValues[period]
		if !ok {
			continue
		}
		out = append(out, rateWindow{
			Limit:  max(1, int(math.Floor(float64(limit)*0.9))),
			Count:  count,
			Period: time.Duration(period) * time.Second,
		})
	}
	return out
}

func parseRateHeader(value string) map[int]int {
	out := make(map[int]int)
	for _, pair := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) != 2 {
			continue
		}
		count, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		period, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 == nil && err2 == nil && count >= 0 && period > 0 {
			out[period] = count
		}
	}
	return out
}
