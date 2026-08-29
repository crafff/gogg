package summoner

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase3"
	"github.com/crafff/gogg/apps/worker/internal/runtime"
	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const historyScanLimit = 100

var supportedQueues = map[int]struct{}{
	400: {},
	420: {},
	440: {},
	480: {},
}

type Activities struct {
	rt      *runtime.Runtime
	queries *sqlcgen.Queries
}

func New(rt *runtime.Runtime) *Activities {
	return &Activities{rt: rt, queries: sqlcgen.New(rt.Store.Pool)}
}

type ResolveInput struct {
	JobID    string `json:"job_id"`
	Region   string `json:"region"`
	GameName string `json:"game_name"`
	TagLine  string `json:"tag_line"`
}

type ResolveOutput struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"game_name"`
	TagLine  string `json:"tag_line"`
}

func (a *Activities) ResolveAndRefreshProfile(ctx context.Context, in ResolveInput) (ResolveOutput, error) {
	_, _ = a.queries.DeleteExpiredSummonerLookupJobs(ctx)
	if err := a.queries.MarkSummonerLookupJobRunning(ctx, "RESOLVE_ACCOUNT", nil, in.JobID); err != nil {
		return ResolveOutput{}, err
	}

	client, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return ResolveOutput{}, temporal.NewNonRetryableApplicationError(err.Error(), "INVALID_REGION", err)
	}
	account, err := client.GetAccountByRiotID(ctx, in.GameName, in.TagLine)
	if err != nil {
		if riotapi.IsNotFound(err) {
			return ResolveOutput{}, temporal.NewNonRetryableApplicationError("Riot ID not found", "NOT_FOUND", err)
		}
		if riotapi.IsUnauthorized(err) {
			return ResolveOutput{}, temporal.NewNonRetryableApplicationError("Riot API credentials rejected", "RIOT_UNAVAILABLE", err)
		}
		return ResolveOutput{}, err
	}
	if strings.TrimSpace(account.Puuid) == "" {
		return ResolveOutput{}, temporal.NewNonRetryableApplicationError("Riot account response has no PUUID", "INVALID_RESPONSE", nil)
	}
	if err := a.rt.Store.UpsertPlayer(ctx, account.Puuid, strings.ToUpper(in.Region), &account.GameName, &account.TagLine); err != nil {
		return ResolveOutput{}, err
	}
	if err := a.queries.MarkSummonerLookupJobRunning(ctx, "REFRESH_PROFILE", &account.Puuid, in.JobID); err != nil {
		return ResolveOutput{}, err
	}

	now := time.Now().UTC()
	profile, err := client.GetSummonerByPUUID(ctx, account.Puuid)
	if err != nil {
		if riotapi.IsNotFound(err) {
			// Account-V1 is regional-routing scoped and can resolve an account
			// that belongs to another platform. A platform Summoner-V4 404 means
			// the selected server does not contain this player; retrying the same
			// platform only delays a useful answer.
			return ResolveOutput{}, notFoundInRegionError(err)
		}
		return ResolveOutput{}, err
	}
	iconID := int32(profile.ProfileIconID)
	level := profile.SummonerLevel
	if err := a.queries.UpsertSummonerProfile(ctx, sqlcgen.UpsertSummonerProfileParams{
		Region:              strings.ToUpper(in.Region),
		Puuid:               account.Puuid,
		ProfileIconID:       &iconID,
		SummonerLevel:       &level,
		IdentityRefreshedAt: timestamp(now),
	}); err != nil {
		return ResolveOutput{}, err
	}

	entries, err := client.GetEntriesByPUUID(ctx, account.Puuid)
	if err != nil {
		if riotapi.IsNotFound(err) {
			return ResolveOutput{}, temporal.NewApplicationErrorWithCause(
				"ranked profile is not yet available in the selected region",
				"RANK_PROFILE_NOT_READY",
				err,
			)
		}
		return ResolveOutput{}, err
	}
	kept := make([]string, 0, 2)
	for _, entry := range entries {
		if entry.QueueType != "RANKED_SOLO_5x5" && entry.QueueType != "RANKED_FLEX_SR" {
			continue
		}
		division := entry.Rank
		if err := a.queries.UpsertSummonerCurrentRank(ctx, sqlcgen.UpsertSummonerCurrentRankParams{
			Region:       strings.ToUpper(in.Region),
			Puuid:        account.Puuid,
			QueueType:    entry.QueueType,
			Tier:         entry.Tier,
			Division:     &division,
			LeaguePoints: int32(entry.LeaguePoints),
			Wins:         int32(entry.Wins),
			Losses:       int32(entry.Losses),
			RefreshedAt:  timestamp(now),
		}); err != nil {
			return ResolveOutput{}, err
		}
		kept = append(kept, entry.QueueType)
	}
	if len(kept) == 0 {
		if err := a.queries.DeleteAllSummonerCurrentRanks(ctx, strings.ToUpper(in.Region), account.Puuid); err != nil {
			return ResolveOutput{}, err
		}
	} else if err := a.queries.DeleteMissingSummonerCurrentRanks(ctx, strings.ToUpper(in.Region), account.Puuid, kept); err != nil {
		return ResolveOutput{}, err
	}

	return ResolveOutput{PUUID: account.Puuid, GameName: account.GameName, TagLine: account.TagLine}, nil
}

type FetchInput struct {
	JobID  string `json:"job_id"`
	Region string `json:"region"`
	PUUID  string `json:"puuid"`
}

type FetchOutput struct {
	Scanned   int `json:"scanned"`
	Supported int `json:"supported"`
	Fetched   int `json:"fetched"`
	Failed    int `json:"failed"`
}

func (a *Activities) FetchRecentMatches(ctx context.Context, in FetchInput) (FetchOutput, error) {
	client, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return FetchOutput{}, temporal.NewNonRetryableApplicationError(err.Error(), "INVALID_REGION", err)
	}
	if err := a.queries.MarkSummonerLookupJobRunning(ctx, "FETCH_MATCHES", &in.PUUID, in.JobID); err != nil {
		return FetchOutput{}, err
	}
	ids, err := client.GetMatchIDsByPUUID(ctx, in.PUUID, 0, 0, 0, 0, historyScanLimit)
	if err != nil {
		return FetchOutput{}, err
	}

	out := FetchOutput{}
	for ordinal, matchID := range ids {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out.Scanned++
		info, err := a.rt.Store.GetMatchFetchInfo(ctx, matchID)
		if err != nil {
			return out, err
		}
		if info == nil {
			if err := a.rt.Store.UpsertOnDemandMatchID(ctx, matchID, strings.ToUpper(in.Region)); err != nil {
				return out, err
			}
			info, err = a.rt.Store.GetMatchFetchInfo(ctx, matchID)
			if err != nil {
				return out, err
			}
		}

		if info == nil || info.FetchStatus != "done" || info.QueueID == nil {
			detail, fetchErr := client.GetMatchDetailUnrecorded(ctx, matchID)
			if fetchErr != nil {
				if riotapi.IsRetryable(fetchErr) {
					return out, fetchErr
				}
				out.Failed++
				if riotapi.IsNotFound(fetchErr) {
					_ = a.rt.Store.MarkMatchNotFound(ctx, matchID, fetchErr.Error())
				}
				// Keep the 100-item scan window and progress monotonic even when
				// Riot permanently rejects an individual match detail.
				if err := a.queries.UpsertSummonerLookupJobMatch(ctx, sqlcgen.UpsertSummonerLookupJobMatchParams{
					JobID: in.JobID, MatchID: matchID, Ordinal: int32(ordinal), Supported: false,
				}); err != nil {
					return out, err
				}
				if ordinal%5 == 0 || ordinal == len(ids)-1 {
					if err := a.updateProgress(ctx, in.JobID, out); err != nil {
						return out, err
					}
					activity.RecordHeartbeat(ctx, out)
				}
				continue
			}
			if err := phase3.IngestMatchDetailWithOptions(ctx, a.rt.Store, strings.ToUpper(in.Region), matchID, detail, phase3.IngestOptions{}); err != nil {
				return out, err
			}
			out.Fetched++
			info, err = a.rt.Store.GetMatchFetchInfo(ctx, matchID)
			if err != nil {
				return out, err
			}
		}

		supported := false
		if info.QueueID != nil {
			_, supported = supportedQueues[*info.QueueID]
		}
		if supported {
			out.Supported++
		}
		if err := a.queries.UpsertSummonerLookupJobMatch(ctx, sqlcgen.UpsertSummonerLookupJobMatchParams{
			JobID: in.JobID, MatchID: matchID, Ordinal: int32(ordinal), Supported: supported,
		}); err != nil {
			return out, err
		}
		if ordinal%5 == 0 || ordinal == len(ids)-1 {
			if err := a.updateProgress(ctx, in.JobID, out); err != nil {
				return out, err
			}
			activity.RecordHeartbeat(ctx, out)
		}
	}

	return out, nil
}

func notFoundInRegionError(cause error) error {
	return temporal.NewNonRetryableApplicationError(
		"Riot account was found, but it has no summoner profile in the selected region",
		"NOT_FOUND_IN_REGION",
		cause,
	)
}

func (a *Activities) updateProgress(ctx context.Context, jobID string, out FetchOutput) error {
	return a.queries.UpdateSummonerLookupJobProgress(ctx, sqlcgen.UpdateSummonerLookupJobProgressParams{
		Stage: "FETCH_MATCHES", ScannedCount: int32(out.Scanned), SupportedCount: int32(out.Supported),
		FetchedCount: int32(out.Fetched), FailedCount: int32(out.Failed), ID: jobID,
	})
}

type FailInput struct {
	JobID   string `json:"job_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a *Activities) FailLookupJob(ctx context.Context, in FailInput) error {
	code, message := in.Code, in.Message
	if code == "" {
		code = "REFRESH_FAILED"
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	return a.queries.FinishSummonerLookupJob(ctx, sqlcgen.FinishSummonerLookupJobParams{
		Status: "FAILED", ErrorCode: &code, ErrorMessage: &message, ID: in.JobID,
	})
}

func timestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func ErrorCode(err error) string {
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) && appErr.Type() != "" {
		return appErr.Type()
	}
	return "REFRESH_FAILED"
}
