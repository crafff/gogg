package summoner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase4"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const (
	rankedSoloQueue = "RANKED_SOLO_5x5"
	masterBaseScore = 2800
)

type EnrichRanksInput struct {
	JobID  string `json:"job_id"`
	Region string `json:"region"`
}

type EnrichRanksOutput struct {
	Targets         int `json:"targets"`
	Ranked          int `json:"ranked"`
	Unranked        int `json:"unranked"`
	Failed          int `json:"failed"`
	MatchesComputed int `json:"matches_computed"`
}

// EnrichMatchRanks resolves ranked-solo entries only for participants in the
// lookup job. It deliberately does not reuse crawler phase 3.5, whose pending
// query is region-wide and would turn one summoner lookup into a full backlog
// scan.
func (a *Activities) EnrichMatchRanks(ctx context.Context, in EnrichRanksInput) (EnrichRanksOutput, error) {
	client, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return EnrichRanksOutput{}, temporal.NewNonRetryableApplicationError(err.Error(), "INVALID_REGION", err)
	}
	if err := a.queries.MarkSummonerLookupJobRunning(ctx, "ENRICH_RANKS", nil, in.JobID); err != nil {
		return EnrichRanksOutput{}, err
	}

	targets, err := a.queries.ListSummonerLookupRankTargets(ctx, in.JobID)
	if err != nil {
		return EnrichRanksOutput{}, err
	}
	out := EnrichRanksOutput{Targets: len(targets)}
	for index, puuid := range targets {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		activity.RecordHeartbeat(ctx, out)
		entries, fetchErr := client.GetEntriesByPUUID(ctx, puuid)
		if fetchErr != nil {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			if riotapi.IsGlobalPermanent(fetchErr) {
				return out, temporal.NewNonRetryableApplicationError(
					"Riot rank API is unavailable", "RIOT_UNAVAILABLE", fetchErr,
				)
			}
			if riotapi.IsRetryable(fetchErr) {
				return out, fetchErr
			}
			// A participant-scoped terminal result (for example a stale PUUID)
			// must not discard all otherwise usable match history.
			out.Failed++
			continue
		}

		entry, found := rankedSoloEntry(entries)
		puuidCopy := puuid
		if !found {
			if err := a.queries.MarkSummonerLookupParticipantUnranked(ctx, in.JobID, &puuidCopy); err != nil {
				return out, err
			}
			out.Unranked++
		} else {
			tier := strings.ToUpper(entry.Tier)
			leaguePoints := int32(entry.LeaguePoints)
			if err := a.queries.BackfillSummonerLookupParticipantRank(ctx, sqlcgen.BackfillSummonerLookupParticipantRankParams{
				Tier: &tier, Division: strings.ToUpper(entry.Rank), LeaguePoints: &leaguePoints,
				JobID: in.JobID, Puuid: &puuidCopy,
			}); err != nil {
				return out, err
			}
			out.Ranked++
		}

		if (index+1)%10 == 0 || index == len(targets)-1 {
			if err := a.queries.MarkSummonerLookupJobRunning(ctx, "ENRICH_RANKS", nil, in.JobID); err != nil {
				return out, err
			}
			activity.RecordHeartbeat(ctx, out)
		}
	}

	thresholdRow, err := a.queries.GetSummonerLookupApexThresholds(ctx, in.JobID)
	if err != nil {
		return out, err
	}
	thresholds := lookupApexThresholds(thresholdRow)
	matchIDs, err := a.queries.ListSummonerLookupMatchIDs(ctx, in.JobID)
	if err != nil {
		return out, err
	}
	calculator := phase4.New(a.rt.Store)
	for index, matchID := range matchIDs {
		if err := calculator.ComputeAndStore(ctx, matchID, thresholds); err != nil {
			return out, fmt.Errorf("compute average rank for %s: %w", matchID, err)
		}
		out.MatchesComputed++
		if (index+1)%10 == 0 || index == len(matchIDs)-1 {
			activity.RecordHeartbeat(ctx, out)
		}
	}
	return out, nil
}

func rankedSoloEntry(entries []riotapi.LeagueEntryDTO) (riotapi.LeagueEntryDTO, bool) {
	for _, entry := range entries {
		if entry.QueueType == rankedSoloQueue && strings.TrimSpace(entry.Tier) != "" {
			return entry, true
		}
	}
	return riotapi.LeagueEntryDTO{}, false
}

func lookupApexThresholds(row sqlcgen.GetSummonerLookupApexThresholdsRow) storage.ApexThresholds {
	noThreshold := int(^uint(0) >> 1)
	thresholds := storage.ApexThresholds{
		ChallengerMinScore:  noThreshold,
		GrandmasterMinScore: noThreshold,
	}
	if row.ChallengerMinLp >= 0 {
		thresholds.ChallengerMinScore = masterBaseScore + int(row.ChallengerMinLp)
	}
	if row.GrandmasterMinLp >= 0 {
		thresholds.GrandmasterMinScore = masterBaseScore + int(row.GrandmasterMinLp)
	}
	return thresholds
}

type FinalizeInput struct {
	JobID         string `json:"job_id"`
	Region        string `json:"region"`
	PUUID         string `json:"puuid"`
	MatchFailures int    `json:"match_failures"`
	RankFailures  int    `json:"rank_failures"`
}

func (a *Activities) FinalizeLookupJob(ctx context.Context, in FinalizeInput) error {
	if err := a.queries.MarkSummonerLookupJobRunning(ctx, "FINALIZE", &in.PUUID, in.JobID); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := a.queries.UpsertSummonerProfile(ctx, sqlcgen.UpsertSummonerProfileParams{
		Region: strings.ToUpper(in.Region), Puuid: in.PUUID, MatchesRefreshedAt: timestamp(now),
	}); err != nil {
		return err
	}

	status := "COMPLETED"
	var errorCode, errorMessage *string
	if in.MatchFailures > 0 || in.RankFailures > 0 {
		status = "PARTIAL"
		code := "PARTIAL_LOOKUP_FAILURE"
		message := fmt.Sprintf(
			"%d recent match detail request(s) and %d participant rank request(s) failed",
			in.MatchFailures, in.RankFailures,
		)
		errorCode, errorMessage = &code, &message
	}
	return a.queries.FinishSummonerLookupJob(ctx, sqlcgen.FinishSummonerLookupJobParams{
		Status: status, ErrorCode: errorCode, ErrorMessage: errorMessage, ID: in.JobID,
	})
}
