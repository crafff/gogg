package resolver

import (
	"errors"
	"fmt"
	"math"
	"time"

	summonersvc "github.com/crafff/gogg/apps/api/internal/service/summoner"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

func mapIdentity(in gqlgenerated.SummonerIdentityInput) summonersvc.Identity {
	return summonersvc.Identity{Region: in.Region, GameName: in.GameName, TagLine: in.TagLine}
}

func mapSummonerResult(in *summonersvc.Result) *gqlgenerated.SummonerResult {
	if in == nil {
		return nil
	}
	profile := &gqlgenerated.SummonerProfile{
		Region: in.Profile.Region, GameName: in.Profile.GameName, TagLine: in.Profile.TagLine,
		ProfileIconID: in.Profile.ProfileIconID, SummonerLevel: int(in.Profile.SummonerLevel), IsStale: in.Profile.IsStale,
	}
	if in.Profile.LastRefreshedAt != nil {
		formatted := formatTime(*in.Profile.LastRefreshedAt)
		profile.LastRefreshedAt = &formatted
	}
	ranks := make([]*gqlgenerated.SummonerRank, 0, len(in.Ranks))
	for _, rank := range in.Ranks {
		ranks = append(ranks, &gqlgenerated.SummonerRank{
			QueueType: rank.QueueType, Tier: rank.Tier, Division: rank.Division,
			LeaguePoints: rank.LeaguePoints, Wins: rank.Wins, Losses: rank.Losses, WinRate: rank.WinRate,
		})
	}
	matches := make([]*gqlgenerated.SummonerMatch, 0, len(in.Matches))
	for _, match := range in.Matches {
		participants := make([]*gqlgenerated.SummonerMatchParticipant, 0, len(match.Participants))
		for _, participant := range match.Participants {
			var rank *gqlgenerated.SummonerMatchParticipantRank
			if participant.Rank != nil {
				rank = &gqlgenerated.SummonerMatchParticipantRank{
					Tier: participant.Rank.Tier, Division: participant.Rank.Division,
					LeaguePoints:       participant.Rank.LeaguePoints,
					SnapshotDeltaHours: participant.Rank.SnapshotDeltaHours,
				}
			}
			participants = append(participants, &gqlgenerated.SummonerMatchParticipant{
				ParticipantID: participant.ParticipantID, TeamID: participant.TeamID,
				IsCurrentPlayer: participant.IsCurrentPlayer, GameName: participant.GameName, TagLine: participant.TagLine,
				Position: participant.Position, Win: participant.Win,
				ChampionID: participant.ChampionID, ChampionName: participant.ChampionName, ChampionLevel: participant.ChampionLevel,
				Kills: participant.Kills, Deaths: participant.Deaths, Assists: participant.Assists, Kda: participant.KDA,
				MinionsKilled: participant.MinionsKilled, GoldEarned: participant.GoldEarned,
				DamageToChampions: participant.DamageToChampions, VisionScore: participant.VisionScore,
				ItemIds: participant.ItemIDs, SummonerSpellIds: participant.SummonerSpellIDs,
				PrimaryStyleID: participant.PrimaryStyleID, SecondaryStyleID: participant.SecondaryStyleID,
				PerkIds: participant.PerkIDs, Rank: rank,
			})
		}
		matches = append(matches, &gqlgenerated.SummonerMatch{
			MatchID: match.MatchID, Queue: gqlgenerated.SummonerQueueFilter(match.Queue), QueueID: match.QueueID,
			GameStartTime: formatTime(match.GameStartTime), DurationSeconds: match.DurationSeconds, Version: match.Version,
			EndOfGameResult: match.EndOfGameResult, Position: match.Position, Win: match.Win,
			ChampionID: match.ChampionID, ChampionName: match.ChampionName, ChampionLevel: match.ChampionLevel,
			Kills: match.Kills, Deaths: match.Deaths, Assists: match.Assists, Kda: match.KDA,
			MinionsKilled: match.MinionsKilled, CsPerMinute: match.CSPerMinute, GoldEarned: match.GoldEarned,
			DamageToChampions: match.DamageToChampions, VisionScore: match.VisionScore,
			ItemIds: match.ItemIDs, SummonerSpellIds: match.SummonerSpellIDs,
			PrimaryStyleID: match.PrimaryStyleID, SecondaryStyleID: match.SecondaryStyleID,
			PerkIds: match.PerkIDs, StatShardIds: match.StatShardIDs,
			EarlySurrender: match.EarlySurrender, Surrender: match.Surrender,
			AverageTier: match.AverageTier, AverageDivision: match.AverageDivision,
			TierCoverage: match.TierCoverage,
			Participants: participants,
		})
	}
	return &gqlgenerated.SummonerResult{
		Profile: profile, Ranks: ranks,
		History: &gqlgenerated.SummonerMatchConnection{
			Items: matches,
			PageInfo: &gqlgenerated.SummonerMatchPageInfo{
				EndCursor: in.PageInfo.EndCursor, HasNextPage: in.PageInfo.HasNextPage, Returned: in.PageInfo.Returned,
			},
		},
	}
}

func mapLookupJob(in *summonersvc.Job) *gqlgenerated.SummonerLookupJob {
	if in == nil {
		return nil
	}
	out := &gqlgenerated.SummonerLookupJob{
		ID: in.ID, Region: in.Region, GameName: in.GameName, TagLine: in.TagLine,
		Status: gqlgenerated.SummonerLookupStatus(in.Status), Stage: gqlgenerated.SummonerLookupStage(in.Stage),
		ScannedCount: in.ScannedCount, SupportedCount: in.SupportedCount,
		FetchedCount: in.FetchedCount, FailedCount: in.FailedCount,
		ErrorCode: in.ErrorCode, ErrorMessage: in.ErrorMessage,
		CreatedAt: formatTime(in.CreatedAt), UpdatedAt: formatTime(in.UpdatedAt),
	}
	if in.CompletedAt != nil {
		formatted := formatTime(*in.CompletedAt)
		out.CompletedAt = &formatted
	}
	return out
}

func mapSummonerError(err error) error {
	var validation *summonersvc.ValidationError
	if errors.As(err, &validation) {
		return domainerr.Wrap("BAD_USER_INPUT", validation.Error(), err)
	}
	var limited *summonersvc.RateLimitError
	if errors.As(err, &limited) {
		seconds := max(1, int(math.Ceil(limited.RetryAfter.Seconds())))
		return domainerr.WrapWithExtensions(
			"RATE_LIMITED",
			fmt.Sprintf("refresh limit reached; retry in %d seconds", seconds),
			err,
			map[string]any{
				"retryAfterSeconds": seconds,
				"rateLimitScope":    string(limited.Scope),
			},
		)
	}
	if errors.Is(err, summonersvc.ErrRefreshUnavailable) {
		return domainerr.Wrap("SERVICE_UNAVAILABLE", "summoner refresh is temporarily unavailable", err)
	}
	return err
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }
