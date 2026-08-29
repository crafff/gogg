package resolver

import (
	"errors"
	"testing"
	"time"

	summonersvc "github.com/crafff/gogg/apps/api/internal/service/summoner"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
)

func TestMapSummonerRateLimitIncludesRoundedRetryDuration(t *testing.T) {
	mapped := mapSummonerError(&summonersvc.RateLimitError{
		RetryAfter: 1500 * time.Millisecond,
		Scope:      summonersvc.RateLimitIP,
	})

	var domain *domainerr.Error
	if !errors.As(mapped, &domain) {
		t.Fatalf("mapped error type = %T, want *domainerr.Error", mapped)
	}
	if domain.Code != "RATE_LIMITED" {
		t.Fatalf("code = %q", domain.Code)
	}
	if domain.Extensions["retryAfterSeconds"] != 2 {
		t.Fatalf("retryAfterSeconds = %#v, want 2", domain.Extensions["retryAfterSeconds"])
	}
	if domain.Extensions["rateLimitScope"] != "IP" {
		t.Fatalf("rateLimitScope = %#v, want IP", domain.Extensions["rateLimitScope"])
	}
}

func TestMapSummonerResultIncludesMatchParticipants(t *testing.T) {
	mapped := mapSummonerResult(&summonersvc.Result{
		Profile: summonersvc.Profile{Region: "KR", GameName: "Hide on bush", TagLine: "KR1"},
		Matches: []summonersvc.Match{{
			MatchID: "KR_123", AverageTier: stringTestPointer("EMERALD"),
			AverageDivision: stringTestPointer("II"), TierCoverage: 8,
			Participants: []summonersvc.Participant{{
				ParticipantID: 1, TeamID: 100, IsCurrentPlayer: true,
				GameName: "Hide on bush", TagLine: "KR1", ChampionID: 7, ChampionName: "LeBlanc",
				Kills: 10, Deaths: 2, Assists: 8, KDA: 9, ItemIDs: []int{3089}, SummonerSpellIDs: []int{4, 12},
				Rank: &summonersvc.ParticipantRank{Tier: "MASTER", LeaguePoints: intTestPointer(321)},
			}},
		}},
	})

	if mapped == nil || len(mapped.History.Items) != 1 || len(mapped.History.Items[0].Participants) != 1 {
		t.Fatalf("participant mapping shape = %#v", mapped)
	}
	participant := mapped.History.Items[0].Participants[0]
	if !participant.IsCurrentPlayer || participant.GameName != "Hide on bush" || participant.ChampionID != 7 {
		t.Fatalf("participant = %#v", participant)
	}
	if participant.Rank == nil || participant.Rank.Tier != "MASTER" || participant.Rank.LeaguePoints == nil || *participant.Rank.LeaguePoints != 321 {
		t.Fatalf("participant rank = %#v", participant.Rank)
	}
	match := mapped.History.Items[0]
	if match.AverageTier == nil || *match.AverageTier != "EMERALD" || match.TierCoverage != 8 {
		t.Fatalf("match average rank = %#v coverage=%d", match.AverageTier, match.TierCoverage)
	}
}

func stringTestPointer(value string) *string { return &value }
func intTestPointer(value int) *int          { return &value }
