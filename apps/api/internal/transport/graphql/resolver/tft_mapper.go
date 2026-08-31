package resolver

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/crafff/gogg/apps/api/internal/service/tft"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

func mapTFTObservedLineups(result *tft.ObservedResult) *gqlgenerated.TFTObservedLineupsResult {
	if result == nil {
		return nil
	}
	out := &gqlgenerated.TFTObservedLineupsResult{
		DataKind: gqlgenerated.TFTObservedDataKind(result.DataKind), RunID: fmt.Sprint(result.RunID),
		Platform: result.Platform, Platforms: result.Platforms,
		QueueID: result.QueueID, SetNumber: result.SetNumber, Patch: result.Patch,
		RawGameVersions: result.RawGameVersions, Locale: result.Locale,
		AlgorithmVersion: result.AlgorithmVersion,
		SourceMatches:    int(result.SourceMatches), SourceParticipants: int(result.SourceParticipants),
		UsableParticipants: int(result.UsableParticipants), ExactLineups: int(result.ExactLineups),
		WindowStart: result.WindowStart.UTC().Format(time.RFC3339),
		WindowEnd:   result.WindowEnd.UTC().Format(time.RFC3339),
		Items:       make([]*gqlgenerated.TFTLineup, 0, len(result.Items)),
	}
	if result.CatalogSnapshot != nil {
		out.CatalogSnapshot = &gqlgenerated.TFTObservedStaticSnapshot{
			Source: result.CatalogSnapshot.Source,
			Patch:  result.CatalogSnapshot.Patch, Revision: result.CatalogSnapshot.Revision,
		}
	}
	if result.AssetSnapshot != nil {
		out.AssetSnapshot = &gqlgenerated.TFTObservedStaticSnapshot{
			Source: result.AssetSnapshot.Source,
			Patch:  result.AssetSnapshot.Patch, Revision: result.AssetSnapshot.Revision,
		}
	}
	for _, item := range result.Items {
		out.Items = append(out.Items, mapTFTLineup(item))
	}
	return out
}

func mapTFTCoverage(coverage tft.Coverage) *gqlgenerated.TFTAnalysisCoverage {
	windowStart, windowEnd := "", ""
	if !coverage.WindowStart.IsZero() {
		windowStart = coverage.WindowStart.UTC().Format(time.RFC3339)
	}
	if !coverage.WindowEnd.IsZero() {
		windowEnd = coverage.WindowEnd.UTC().Format(time.RFC3339)
	}
	return &gqlgenerated.TFTAnalysisCoverage{WindowStart: windowStart, WindowEnd: windowEnd, ExactLineups: coverage.ExactLineups, FamilyThreshold: coverage.FamilyThreshold}
}

func mapTFTLineup(item tft.Lineup) *gqlgenerated.TFTLineup {
	return &gqlgenerated.TFTLineup{
		ID:                            item.ID,
		CoreUnits:                     mapTFTEntities(item.CoreUnits),
		CommonItems:                   mapTFTCounts(item.CommonItems),
		CommonAugments:                mapTFTCounts(item.CommonAugments),
		CommonTraits:                  mapTFTCounts(item.CommonTraits),
		UnitItems:                     mapTFTUnitItems(item.UnitItems),
		StarLevels:                    mapTFTStarLevels(item.StarLevels),
		StarCompositionKnownSamples:   int(item.StarCompositionKnownSamples),
		StarCompositionUnknownSamples: int(item.StarCompositionUnknownSamples),
		StarCompositionCoverage:       item.StarCompositionCoverage,
		StarCompositions:              mapTFTStarCompositions(item.StarCompositions),
		Metrics: &gqlgenerated.TFTLineupMetrics{
			SampleSize: int(item.Metrics.SampleSize), LobbyCount: int(item.Metrics.LobbyCount),
			PickRate: item.Metrics.PickRate, AvgPlacement: item.Metrics.AvgPlacement,
			FirstRate: item.Metrics.FirstRate, Top4Rate: item.Metrics.Top4Rate,
			ContestedRate: item.Metrics.ContestedRate,
		},
	}
}

func mapTFTUnitItems(items []tft.UnitItems) []*gqlgenerated.TFTLineupUnitItems {
	out := make([]*gqlgenerated.TFTLineupUnitItems, 0, len(items))
	for _, item := range items {
		starDistribution := make([]*gqlgenerated.TFTLineupUnitStarBucket, 0, len(item.StarDistribution))
		for _, level := range item.StarDistribution {
			starDistribution = append(starDistribution, &gqlgenerated.TFTLineupUnitStarBucket{
				Stars: level.Stars, SampleSize: level.SampleSize,
				Rate: level.Rate, KnownRate: level.KnownRate,
				AvgPlacement: level.AvgPlacement, FirstRate: level.FirstRate,
				Top4Rate: level.Top4Rate,
			})
		}
		out = append(out, &gqlgenerated.TFTLineupUnitItems{
			Unit: mapTFTEntities([]tft.Entity{item.Unit})[0], CommonItems: mapTFTCounts(item.CommonItems),
			IsCore: item.CoreRank != nil, CoreRank: item.CoreRank,
			AverageItems: item.AverageItems, ItemInvestmentRate: item.ItemInvestmentRate,
			EquippedRate: item.EquippedRate, ThreeItemRate: item.ThreeItemRate,
			KnownStarSamples: int(item.KnownStarSamples), UnknownStarSamples: int(item.UnknownStarSamples),
			StarCoverage: item.StarCoverage, StarDistribution: starDistribution,
		})
	}
	return out
}

func mapTFTStarCompositions(items []tft.StarCompositionStrength) []*gqlgenerated.TFTLineupStarComposition {
	out := make([]*gqlgenerated.TFTLineupStarComposition, 0, len(items))
	for _, item := range items {
		levels := make([]*gqlgenerated.TFTLineupStarCount, 0, len(item.Levels))
		for _, level := range item.Levels {
			levels = append(levels, &gqlgenerated.TFTLineupStarCount{
				Stars: level.Stars, UnitCount: level.UnitCount,
			})
		}
		out = append(out, &gqlgenerated.TFTLineupStarComposition{
			Levels: levels, TotalStars: item.TotalStars,
			SampleSize: item.SampleSize, Rate: item.Rate,
			AvgPlacement: item.AvgPlacement, FirstRate: item.FirstRate,
			Top4Rate: item.Top4Rate,
		})
	}
	return out
}

func mapTFTStarLevels(items []tft.StarLevelStrength) []*gqlgenerated.TFTLineupStarStrength {
	out := make([]*gqlgenerated.TFTLineupStarStrength, 0, len(items))
	for _, item := range items {
		out = append(out, &gqlgenerated.TFTLineupStarStrength{
			TotalStars: item.TotalStars,
			SampleSize: int(item.SampleSize), LobbyCount: int(item.LobbyCount),
			Rate: item.Rate, AvgPlacement: item.AvgPlacement,
			FirstRate: item.FirstRate, Top4Rate: item.Top4Rate,
		})
	}
	return out
}

func mapTFTEntities(items []tft.Entity) []*gqlgenerated.TFTEntity {
	out := make([]*gqlgenerated.TFTEntity, 0, len(items))
	for _, item := range items {
		var name *string
		if item.Name != "" {
			value := item.Name
			name = &value
		}
		var iconURL *string
		if item.IconURL != "" {
			value := item.IconURL
			iconURL = &value
		}
		out = append(out, &gqlgenerated.TFTEntity{ID: item.ID, Name: name, IconURL: iconURL})
	}
	return out
}

func mapTFTHistoryResult(result *tft.HistoryResult) *gqlgenerated.TFTHistoryResult {
	if result == nil {
		return nil
	}
	profile := &gqlgenerated.TFTPlayerProfile{Platform: result.Profile.Platform, GameName: result.Profile.GameName, TagLine: result.Profile.TagLine, IsStale: result.Profile.IsStale}
	if result.Profile.LastRefreshedAt != nil {
		value := result.Profile.LastRefreshedAt.UTC().Format(time.RFC3339)
		profile.LastRefreshedAt = &value
	}
	out := &gqlgenerated.TFTHistoryResult{Profile: profile, Matches: make([]*gqlgenerated.TFTHistoryMatch, 0, len(result.Matches)), PageInfo: &gqlgenerated.TFTMatchPageInfo{EndCursor: result.PageInfo.EndCursor, HasNextPage: result.PageInfo.HasNextPage, Returned: result.PageInfo.Returned}}
	for _, match := range result.Matches {
		participants := make([]*gqlgenerated.TFTHistoryParticipant, 0, len(match.Participants))
		for _, participant := range match.Participants {
			participants = append(participants, mapTFTHistoryParticipant(participant))
		}
		out.Matches = append(out.Matches, &gqlgenerated.TFTHistoryMatch{
			MatchID: match.MatchID, Platform: match.Platform, RoutingRegion: match.RoutingRegion,
			QueueID: match.QueueID, GameVersion: match.GameVersion, Patch: match.Patch,
			GameDatetime: match.GameDatetime.UTC().Format(time.RFC3339), GameLengthSeconds: match.GameLengthSeconds,
			MapID: match.MapID, TftGameType: match.TFTGameType, SetCoreName: match.SetCoreName,
			SetNumber: match.SetNumber, EndOfGameResult: match.EndOfGameResult,
			ParticipantCount: match.ParticipantCount, Eligible: match.Eligible, ExclusionReason: match.ExclusionReason,
			Participant: mapTFTHistoryParticipant(match.Participant), Participants: participants,
		})
	}
	return out
}

func mapTFTHistoryParticipant(participant tft.HistoryParticipant) *gqlgenerated.TFTHistoryParticipant {
	traits := make([]*gqlgenerated.TFTHistoryTrait, 0, len(participant.Traits))
	for _, trait := range participant.Traits {
		traits = append(traits, &gqlgenerated.TFTHistoryTrait{Entity: mapTFTEntities([]tft.Entity{trait.Entity})[0], NumUnits: trait.NumUnits, Style: trait.Style, TierCurrent: trait.TierCurrent, TierTotal: trait.TierTotal})
	}
	units := make([]*gqlgenerated.TFTHistoryUnit, 0, len(participant.Units))
	for _, unit := range participant.Units {
		units = append(units, &gqlgenerated.TFTHistoryUnit{Entity: mapTFTEntities([]tft.Entity{unit.Entity})[0], Rarity: unit.Rarity, Tier: unit.Tier, Items: mapTFTEntities(unit.Items)})
	}
	return &gqlgenerated.TFTHistoryParticipant{
		Puuid: participant.PUUID, GameName: participant.GameName, TagLine: participant.TagLine,
		IsCurrentPlayer: participant.IsCurrentPlayer, Placement: participant.Placement, Level: participant.Level,
		GoldLeft: participant.GoldLeft, LastRound: participant.LastRound, PlayersEliminated: participant.PlayersEliminated,
		TimeEliminatedSeconds: participant.TimeEliminatedSeconds, TotalDamageToPlayers: participant.TotalDamageToPlayers,
		CompanionContentID: participant.CompanionContentID, CompanionItemID: participant.CompanionItemID,
		CompanionSkinID: participant.CompanionSkinID, CompanionSpecies: participant.CompanionSpecies,
		Abnormal: participant.Abnormal, Augments: mapTFTEntities(participant.Augments), Traits: traits, Units: units,
	}
}

func mapTFTLookupJob(job *tft.Job) *gqlgenerated.TFTLookupJob {
	if job == nil {
		return nil
	}
	out := &gqlgenerated.TFTLookupJob{ID: job.ID, Platform: job.Platform, GameName: job.GameName, TagLine: job.TagLine, Status: gqlgenerated.TFTLookupStatus(job.Status), Stage: gqlgenerated.TFTLookupStage(job.Stage), ScannedCount: job.ScannedCount, FetchedCount: job.FetchedCount, FailedCount: job.FailedCount, ErrorCode: job.ErrorCode, CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339)}
	if job.CompletedAt != nil {
		value := job.CompletedAt.UTC().Format(time.RFC3339)
		out.CompletedAt = &value
	}
	return out
}

func mapTFTError(err error) error {
	var validation *tft.ValidationError
	if errors.As(err, &validation) {
		return domainerr.Wrap("BAD_USER_INPUT", validation.Error(), err)
	}
	var limited *tft.RateLimitError
	if errors.As(err, &limited) {
		seconds := max(1, int(math.Ceil(limited.RetryAfter.Seconds())))
		return domainerr.WrapWithExtensions("RATE_LIMITED", fmt.Sprintf("refresh limit reached; retry in %d seconds", seconds), err, map[string]any{"retryAfterSeconds": seconds, "rateLimitScope": limited.Scope})
	}
	if errors.Is(err, tft.ErrRefreshUnavailable) {
		return domainerr.Wrap("SERVICE_UNAVAILABLE", "TFT refresh is temporarily unavailable", err)
	}
	return err
}

func mapTFTCounts(items []tft.Count) []*gqlgenerated.TFTEntityCount {
	out := make([]*gqlgenerated.TFTEntityCount, 0, len(items))
	for _, item := range items {
		out = append(out, &gqlgenerated.TFTEntityCount{
			Entity: mapTFTEntities([]tft.Entity{item.Entity})[0], Count: item.Count, Rate: item.Rate,
		})
	}
	return out
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func enumValue(value *gqlgenerated.TFTCohort) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func enumWindowValue(value *gqlgenerated.TFTWindow) string {
	if value == nil {
		return ""
	}
	return value.String()
}
