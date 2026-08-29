// Package phase3 fetches full match details for pending match IDs.
package phase3

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/heartbeat"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

type matchFetcher interface {
	GetMatchDetail(ctx context.Context, matchID string) (*riotapi.MatchDetailDTO, error)
}

const batchSize = 500

type Phase struct {
	riot  matchFetcher
	store *storage.Store
}

func New(riot matchFetcher, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int      { return 3 }
func (p *Phase) Name() string { return "Phase3:MatchDetails" }

func (p *Phase) IsDone(ctx context.Context, state *crawler.RunState) (bool, error) {
	count, err := p.store.CountPendingMatchIDs(ctx, state.Region(), state.Profile.Version)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

const logEvery = 50

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	region := state.Region()
	version := state.Profile.Version
	totalPending, err := p.store.CountPendingMatchIDs(ctx, region, version)
	if err != nil {
		return err
	}
	meta := phaselog.Meta{RunID: state.ID, Region: region, Phase: p.Name(), PhaseID: p.ID(), Version: version}
	phaselog.Step(meta, "pending_loaded", "pending", totalPending)

	processed := 0
	failed := 0
	start := time.Now()

	logProgress := func() {
		phaselog.Progress(meta, processed, totalPending, failed, start)
		heartbeat.Record(ctx, map[string]any{
			"run_id":    state.ID,
			"region":    region,
			"version":   version,
			"processed": processed,
			"total":     totalPending,
			"failed":    failed,
		})
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids, err := p.store.GetPendingMatchIDs(ctx, region, version, batchSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			next, err := p.store.NextMatchRetryAt(ctx, region, version)
			if err != nil {
				return err
			}
			if next == nil {
				break
			}
			if err := waitUntil(ctx, *next); err != nil {
				return err
			}
			continue
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := p.processMatch(ctx, region, id); err != nil {
				var apiErr *riotapi.APIError
				if !errors.As(err, &apiErr) || riotapi.IsGlobalPermanent(err) {
					return err
				}
				phaselog.Warn(meta, "match_failed", "match_id", id, "err", err)
				var err2 error
				if riotapi.IsNotFound(err) {
					err2 = p.store.MarkMatchNotFound(ctx, id, err.Error())
				} else {
					err2 = p.store.IncrementMatchRetry(ctx, id, string(apiErr.Kind), apiErr.StatusCode, err.Error())
				}
				if err2 != nil {
					return err2
				}
				failed++
			}
			processed++
			if processed%25 == 0 {
				heartbeat.Record(ctx, map[string]any{
					"run_id":    state.ID,
					"region":    region,
					"version":   version,
					"processed": processed,
					"total":     totalPending,
					"failed":    failed,
				})
			}
			if processed%logEvery == 0 {
				logProgress()
			}
		}
	}

	logProgress()
	return nil
}

func waitUntil(ctx context.Context, at time.Time) error {
	for {
		delay := time.Until(at)
		if delay <= 0 {
			return nil
		}
		wait := min(delay, time.Minute)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			heartbeat.Record(ctx, map[string]any{"waiting_for_retry_at": at})
		}
	}
}

func (p *Phase) processMatch(ctx context.Context, region string, matchID string) error {
	detail, err := p.riot.GetMatchDetail(ctx, matchID)
	if err != nil {
		return err
	}
	return IngestMatchDetail(ctx, p.store, region, matchID, detail)
}

// IngestMatchDetail maps an already-fetched Match V5 response into the
// relational schema. It is shared by live crawling and offline bundle import.
func IngestMatchDetail(ctx context.Context, store *storage.Store, region string, matchID string, detail *riotapi.MatchDetailDTO) error {
	return IngestMatchDetailWithOptions(ctx, store, region, matchID, detail, IngestOptions{InferRanks: true})
}

type IngestOptions struct {
	InferRanks bool
}

// IngestMatchDetailWithOptions lets on-demand summoner refreshes reuse the
// canonical mapping without running crawler-only rank inference.
func IngestMatchDetailWithOptions(ctx context.Context, store *storage.Store, region string, matchID string, detail *riotapi.MatchDetailDTO, opts IngestOptions) error {
	info := &detail.Info

	// Ensure all participants exist in the players table before writing FKs.
	for _, dp := range info.Participants {
		if dp.Puuid == "" {
			continue
		}
		var gameName, tagLine *string
		if dp.RiotIDGameName != "" {
			gameName = ptr(dp.RiotIDGameName)
		}
		if dp.RiotIDTagline != "" {
			tagLine = ptr(dp.RiotIDTagline)
		}
		if err := store.UpsertPlayerFromMatch(ctx, dp.Puuid, region, gameName, tagLine); err != nil {
			return err
		}
	}

	// Write match header.
	gameStart := time.UnixMilli(info.GameStartTimestamp)
	gameEnd := time.UnixMilli(info.GameEndTimestamp)
	dur := int(info.GameDuration)
	queueID := info.QueueID
	version := riotapi.ExtractCDragonPatch(info.GameVersion)

	h := &storage.MatchHeader{
		MatchID:         matchID,
		DataVersion:     ptr(detail.Metadata.DataVersion),
		PlatformID:      ptr(info.PlatformID),
		QueueID:         &queueID,
		Version:         &version,
		GameVersion:     ptr(info.GameVersion),
		GameMode:        ptr(info.GameMode),
		GameType:        ptr(info.GameType),
		GameStartTS:     &gameStart,
		GameEndTS:       &gameEnd,
		GameDuration:    &dur,
		EndOfGameResult: ptr(info.EndOfGameResult),
	}
	// Write participants.
	participants := make([]storage.Participant, len(info.Participants))
	for i, dp := range info.Participants {
		part := participantFromDTO(matchID, &dp)

		// Rank inference: find closest snapshot to game start time.
		if opts.InferRanks && dp.Puuid != "" {
			snap, _ := store.GetClosestSnapshot(ctx, dp.Puuid, region, gameStart)
			if snap != nil {
				part.TierAtMatch = &snap.Tier
				part.DivisionAtMatch = snap.Division
				part.LPAtMatch = snap.LeaguePoints
				deltaH := int(math.Round(math.Abs(snap.CreatedAt.Sub(gameStart).Hours())))
				part.TierSnapshotDeltaH = &deltaH
			}
		}
		participants[i] = part
	}

	// Write perks.
	var perks []storage.PerkRow
	for _, dp := range info.Participants {
		perk := PerkRowFromDTO(matchID, dp.Puuid, &dp.Perks)
		perks = append(perks, perk)
	}

	// Write teams + bans.
	var teams []storage.TeamDetail
	for _, team := range info.Teams {
		featsJSON, _ := json.Marshal(team.Feats)

		obj := team.Objectives
		bans := make([]storage.BanDetail, len(team.Bans))
		for i, b := range team.Bans {
			bans[i] = storage.BanDetail{ChampionID: b.ChampionID, PickTurn: b.PickTurn}
		}
		teams = append(teams, storage.TeamDetail{
			MatchID:         matchID,
			TeamID:          team.TeamID,
			Win:             team.Win,
			BaronKills:      obj.Baron.Kills,
			DragonKills:     obj.Dragon.Kills,
			TowerKills:      obj.Tower.Kills,
			InhibitorKills:  obj.Inhibitor.Kills,
			RiftHeraldKills: obj.RiftHerald.Kills,
			Feats:           featsJSON,
			Bans:            bans,
		})
	}

	return store.SaveMatchDetail(ctx, h, participants, perks, teams)
}

func participantFromDTO(matchID string, dp *riotapi.ParticipantDTO) storage.Participant {
	puuid := dp.Puuid
	return storage.Participant{
		MatchID: matchID, PUUID: &puuid, ParticipantID: dp.ParticipantID,
		SummonerLevel: dp.SummonerLevel, TeamID: dp.TeamID,
		TeamPosition: dp.TeamPosition, IndividualPosition: dp.IndividualPosition,
		Lane: dp.Lane, Role: dp.Role, Win: dp.Win,
		ChampionID: dp.ChampionID, ChampionName: dp.ChampionName,
		ChampLevel: dp.ChampLevel, ChampExperience: dp.ChampExperience,
		ChampionTransform: dp.ChampionTransform,
		PlayerAugment1:    dp.PlayerAugment1, PlayerAugment2: dp.PlayerAugment2,
		PlayerAugment3: dp.PlayerAugment3, PlayerAugment4: dp.PlayerAugment4,
		PlayerAugment5: dp.PlayerAugment5, PlayerAugment6: dp.PlayerAugment6,
		Summoner1ID: dp.Summoner1ID, Summoner2ID: dp.Summoner2ID,
		Summoner1Casts: dp.Summoner1Casts, Summoner2Casts: dp.Summoner2Casts,
		Spell1Casts: dp.Spell1Casts, Spell2Casts: dp.Spell2Casts,
		Spell3Casts: dp.Spell3Casts, Spell4Casts: dp.Spell4Casts,
		Kills: dp.Kills, Deaths: dp.Deaths, Assists: dp.Assists,
		DoubleKills: dp.DoubleKills, TripleKills: dp.TripleKills,
		QuadraKills: dp.QuadraKills, PentaKills: dp.PentaKills,
		UnrealKills: dp.UnrealKills, KillingSprees: dp.KillingSprees,
		LargestKillingSpree: dp.LargestKillingSpree, LargestMultiKill: dp.LargestMultiKill,
		FirstBloodKill: dp.FirstBloodKill, FirstBloodAssist: dp.FirstBloodAssist,
		LongestTimeSpentLiving: dp.LongestTimeSpentLiving, TotalTimeSpentDead: dp.TotalTimeSpentDead,
		TotalDamageDealt: dp.TotalDamageDealt, TotalDamageDealtToChampions: dp.TotalDamageDealtToChampions,
		PhysicalDamageDealt: dp.PhysicalDamageDealt, PhysicalDamageDealtToChampions: dp.PhysicalDamageDealtToChampions,
		MagicDamageDealt: dp.MagicDamageDealt, MagicDamageDealtToChampions: dp.MagicDamageDealtToChampions,
		TrueDamageDealt: dp.TrueDamageDealt, TrueDamageDealtToChampions: dp.TrueDamageDealtToChampions,
		LargestCriticalStrike: dp.LargestCriticalStrike,
		TotalDamageTaken:      dp.TotalDamageTaken, PhysicalDamageTaken: dp.PhysicalDamageTaken,
		MagicDamageTaken: dp.MagicDamageTaken, TrueDamageTaken: dp.TrueDamageTaken,
		DamageSelfMitigated: dp.DamageSelfMitigated,
		TotalHeal:           dp.TotalHeal, TotalHealsOnTeammates: dp.TotalHealsOnTeammates,
		TotalUnitsHealed: dp.TotalUnitsHealed, TotalDamageShieldedOnTeammates: dp.TotalDamageShieldedOnTeammates,
		TimeCCingOthers: dp.TimeCCingOthers, TotalTimeCCDealt: dp.TotalTimeCCDealt,
		GoldEarned: dp.GoldEarned, GoldSpent: dp.GoldSpent,
		Item0: dp.Item0, Item1: dp.Item1, Item2: dp.Item2, Item3: dp.Item3,
		Item4: dp.Item4, Item5: dp.Item5, Item6: dp.Item6,
		ItemsPurchased: dp.ItemsPurchased, ConsumablesPurchased: dp.ConsumablesPurchased,
		RoleBoundItem:      dp.RoleBoundItem,
		TotalMinionsKilled: dp.TotalMinionsKilled, NeutralMinionsKilled: dp.NeutralMinionsKilled,
		TotalAllyJungleMinions:  dp.TotalAllyJungleMinionsKilled,
		TotalEnemyJungleMinions: dp.TotalEnemyJungleMinionsKilled,
		BaronKills:              dp.BaronKills, DragonKills: dp.DragonKills,
		ObjectivesStolen: dp.ObjectivesStolen, ObjectivesStolenAssists: dp.ObjectivesStolenAssists,
		VisionScore: dp.VisionScore, VisionWardsBought: dp.VisionWardsBoughtInGame,
		SightWardsBought: dp.SightWardsBoughtInGame, WardsPlaced: dp.WardsPlaced,
		DetectorWardsPlaced: dp.DetectorWardsPlaced, WardsKilled: dp.WardsKilled,
		TurretKills: dp.TurretKills, TurretTakedowns: dp.TurretTakedowns, TurretsLost: dp.TurretsLost,
		FirstTowerKill: dp.FirstTowerKill, FirstTowerAssist: dp.FirstTowerAssist,
		InhibitorKills: dp.InhibitorKills, InhibitorTakedowns: dp.InhibitorTakedowns, InhibitorsLost: dp.InhibitorsLost,
		NexusKills: dp.NexusKills, NexusLost: dp.NexusLost, NexusTakedowns: dp.NexusTakedowns,
		DamageDealtToObjectives: dp.DamageDealtToObjectives, DamageDealtToBuildings: dp.DamageDealtToBuildings,
		DamageDealtToTurrets: dp.DamageDealtToTurrets, DamageDealtToEpicMonsters: dp.DamageDealtToEpicMonsters,
		AllInPings: dp.AllInPings, BasicPings: dp.BasicPings, AssistMePings: dp.AssistMePings,
		CommandPings: dp.CommandPings, DangerPings: dp.DangerPings,
		EnemyMissingPings: dp.EnemyMissingPings, GetBackPings: dp.GetBackPings,
		HoldPings: dp.HoldPings, OnMyWayPings: dp.OnMyWayPings,
		NeedVisionPings: dp.NeedVisionPings, PushPings: dp.PushPings, RetreatPings: dp.RetreatPings,
		EnemyVisionPings: dp.EnemyVisionPings, VisionClearedPings: dp.VisionClearedPings,
		GameEndedInEarlySurrender: dp.GameEndedInEarlySurrender,
		GameEndedInSurrender:      dp.GameEndedInSurrender,
		TeamEarlySurrendered:      dp.TeamEarlySurrendered, TimePlayed: dp.TimePlayed,
	}
}

// PerkRowFromDTO maps Riot's ordered 4-primary + 2-secondary selections to
// the six persistent perk slots. It is exported for the historical perk-only
// backfill command, which must use exactly the same mapping as live crawl.
func PerkRowFromDTO(matchID, puuid string, perks *riotapi.PerksDTO) storage.PerkRow {
	row := storage.PerkRow{
		MatchID:     matchID,
		PUUID:       puuid,
		StatDefense: perks.StatPerks.Defense,
		StatFlex:    perks.StatPerks.Flex,
		StatOffense: perks.StatPerks.Offense,
	}
	perkIndex := 0
	for i, style := range perks.Styles {
		switch i {
		case 0:
			row.Style0 = style.Style
		case 1:
			row.Style1 = style.Style
		}
		for _, sel := range style.Selections {
			if perkIndex >= len(row.Perk) {
				break
			}
			row.Perk[perkIndex] = sel.Perk
			row.Vars[perkIndex] = [3]int{sel.Var1, sel.Var2, sel.Var3}
			perkIndex++
		}
	}
	return row
}

func ptr[T any](v T) *T { return &v }
