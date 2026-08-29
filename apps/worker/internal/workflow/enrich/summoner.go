package enrich

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	sumact "github.com/crafff/gogg/apps/worker/internal/activity/summoner"
)

type SummonerInput struct {
	JobID    string `json:"job_id"`
	Region   string `json:"region"`
	GameName string `json:"game_name"`
	TagLine  string `json:"tag_line"`
}

type SummonerOutput struct {
	PUUID         string `json:"puuid"`
	Scanned       int    `json:"scanned"`
	Supported     int    `json:"supported"`
	Fetched       int    `json:"fetched"`
	Failed        int    `json:"failed"`
	RankTargets   int    `json:"rank_targets"`
	Ranked        int    `json:"ranked"`
	Unranked      int    `json:"unranked"`
	RankFailed    int    `json:"rank_failed"`
	MatchesRanked int    `json:"matches_ranked"`
}

var profileOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 5 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval: 2 * time.Second, BackoffCoefficient: 2,
		MaximumInterval: time.Minute, MaximumAttempts: 5,
	},
}

var matchesOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 15 * time.Minute,
	HeartbeatTimeout:    45 * time.Second,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval: 5 * time.Second, BackoffCoefficient: 2,
		MaximumInterval: 2 * time.Minute, MaximumAttempts: 3,
	},
}

var rankOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 30 * time.Minute,
	HeartbeatTimeout:    3 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval: 5 * time.Second, BackoffCoefficient: 2,
		MaximumInterval: 2 * time.Minute, MaximumAttempts: 3,
	},
}

var finalizeOptions = workflow.ActivityOptions{
	StartToCloseTimeout: time.Minute,
	RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
}

var failureOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 30 * time.Second,
	RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
}

func EnrichSummonerWorkflow(ctx workflow.Context, in SummonerInput) (SummonerOutput, error) {
	profileCtx := workflow.WithActivityOptions(ctx, profileOptions)
	var resolved sumact.ResolveOutput
	if err := workflow.ExecuteActivity(profileCtx, (*sumact.Activities).ResolveAndRefreshProfile, sumact.ResolveInput{
		JobID: in.JobID, Region: in.Region, GameName: in.GameName, TagLine: in.TagLine,
	}).Get(profileCtx, &resolved); err != nil {
		fail(ctx, in.JobID, sumact.ErrorCode(err), err.Error())
		return SummonerOutput{}, fmt.Errorf("resolve summoner: %w", err)
	}

	matchesCtx := workflow.WithActivityOptions(ctx, matchesOptions)
	var matches sumact.FetchOutput
	if err := workflow.ExecuteActivity(matchesCtx, (*sumact.Activities).FetchRecentMatches, sumact.FetchInput{
		JobID: in.JobID, Region: in.Region, PUUID: resolved.PUUID,
	}).Get(matchesCtx, &matches); err != nil {
		fail(ctx, in.JobID, sumact.ErrorCode(err), err.Error())
		return SummonerOutput{}, fmt.Errorf("fetch recent matches: %w", err)
	}

	// Existing workflow histories completed inside FetchRecentMatches. The
	// version gate lets those histories replay without scheduling new commands,
	// while new lookups defer completion until rank enrichment and finalization.
	if workflow.GetVersion(ctx, "summoner-rank-enrichment", workflow.DefaultVersion, 1) == workflow.DefaultVersion {
		return SummonerOutput{
			PUUID: resolved.PUUID, Scanned: matches.Scanned, Supported: matches.Supported,
			Fetched: matches.Fetched, Failed: matches.Failed,
		}, nil
	}

	rankCtx := workflow.WithActivityOptions(ctx, rankOptions)
	var ranks sumact.EnrichRanksOutput
	if err := workflow.ExecuteActivity(rankCtx, (*sumact.Activities).EnrichMatchRanks, sumact.EnrichRanksInput{
		JobID: in.JobID, Region: in.Region,
	}).Get(rankCtx, &ranks); err != nil {
		fail(ctx, in.JobID, sumact.ErrorCode(err), err.Error())
		return SummonerOutput{}, fmt.Errorf("enrich participant ranks: %w", err)
	}

	finalizeCtx := workflow.WithActivityOptions(ctx, finalizeOptions)
	if err := workflow.ExecuteActivity(finalizeCtx, (*sumact.Activities).FinalizeLookupJob, sumact.FinalizeInput{
		JobID: in.JobID, Region: in.Region, PUUID: resolved.PUUID,
		MatchFailures: matches.Failed, RankFailures: ranks.Failed,
	}).Get(finalizeCtx, nil); err != nil {
		fail(ctx, in.JobID, sumact.ErrorCode(err), err.Error())
		return SummonerOutput{}, fmt.Errorf("finalize summoner lookup: %w", err)
	}

	return SummonerOutput{
		PUUID: resolved.PUUID, Scanned: matches.Scanned, Supported: matches.Supported,
		Fetched: matches.Fetched, Failed: matches.Failed, RankTargets: ranks.Targets,
		Ranked: ranks.Ranked, Unranked: ranks.Unranked, RankFailed: ranks.Failed,
		MatchesRanked: ranks.MatchesComputed,
	}, nil
}

func fail(ctx workflow.Context, jobID, code, message string) {
	dctx, cancel := workflow.NewDisconnectedContext(ctx)
	defer cancel()
	dctx = workflow.WithActivityOptions(dctx, failureOptions)
	_ = workflow.ExecuteActivity(dctx, (*sumact.Activities).FailLookupJob, sumact.FailInput{
		JobID: jobID, Code: code, Message: message,
	}).Get(dctx, nil)
}
