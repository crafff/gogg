// Package phase3 fetches full match details for pending match IDs.
package phase3

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
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
	ids, err := p.store.GetPendingMatchIDs(ctx, state.Region(), state.Profile.Version, 1)
	if err != nil {
		return false, err
	}
	return len(ids) == 0, nil
}

const logEvery = 50

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	region := state.Region()
	version := state.Profile.Version
	totalPending, err := p.store.CountPendingMatchIDs(ctx, region, version)
	if err != nil {
		return err
	}
	slog.Info("phase3: start", "pending", totalPending)

	processed := 0
	failed := 0
	start := time.Now()

	logProgress := func() {
		elapsed := time.Since(start).Seconds()
		rate := float64(processed) / elapsed
		var eta string
		remaining := totalPending - processed
		if rate > 0 && remaining > 0 {
			etaSec := float64(remaining) / rate
			eta = time.Duration(etaSec * float64(time.Second)).Round(time.Second).String()
		} else {
			eta = "?"
		}
		pct := 0.0
		if totalPending > 0 {
			pct = float64(processed) / float64(totalPending) * 100
		}
		slog.Info("phase3: progress",
			"processed", processed,
			"total", totalPending,
			"pct", int(pct),
			"failed", failed,
			"rate_per_s", math.Round(rate*10)/10,
			"eta", eta,
		)
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
			break
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := p.processMatch(ctx, region, id); err != nil {
				slog.Warn("phase3: failed to process match", "match_id", id, "err", err)
				if err2 := p.store.IncrementMatchRetry(ctx, id); err2 != nil {
					return err2
				}
				failed++
			}
			processed++
			if processed%logEvery == 0 {
				logProgress()
			}
		}
	}

	logProgress()
	return nil
}

func (p *Phase) processMatch(ctx context.Context, region string, matchID string) error {
	detail, err := p.riot.GetMatchDetail(ctx, matchID)
	if err != nil {
		return err
	}
	info := &detail.Info

	// Ensure all participants exist in the players table before writing FKs.
	for _, dp := range info.Participants {
		if dp.Puuid == "" {
			continue
		}
		if err := p.store.UpsertPlayer(ctx, dp.Puuid, region, nil, nil); err != nil {
			return err
		}
	}

	// Write match header.
	gameStart := time.UnixMilli(info.GameStartTimestamp)
	gameEnd := time.UnixMilli(info.GameEndTimestamp)
	dur := int(info.GameDuration)
	queueID := info.QueueID

	h := &storage.MatchHeader{
		MatchID:         matchID,
		DataVersion:     ptr(detail.Metadata.DataVersion),
		PlatformID:      ptr(info.PlatformID),
		QueueID:         &queueID,
		GameVersion:     ptr(info.GameVersion),
		GameMode:        ptr(info.GameMode),
		GameType:        ptr(info.GameType),
		GameStartTS:     &gameStart,
		GameEndTS:       &gameEnd,
		GameDuration:    &dur,
		EndOfGameResult: ptr(info.EndOfGameResult),
	}
	if err := p.store.UpdateMatchDetail(ctx, h); err != nil {
		return err
	}

	// Write participants.
	participants := make([]storage.Participant, len(info.Participants))
	for i, dp := range info.Participants {
		part := participantFromDTO(matchID, &dp)

		// Rank inference: find closest snapshot to game start time.
		if dp.Puuid != "" {
			snap, _ := p.store.GetClosestSnapshot(ctx, dp.Puuid, region, gameStart)
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
	if err := p.store.InsertParticipants(ctx, participants); err != nil {
		return err
	}

	// Write perks.
	var perks []storage.PerkRow
	for _, dp := range info.Participants {
		perk := perkRowFromDTO(matchID, dp.Puuid, &dp.Perks)
		perks = append(perks, perk)
	}
	if err := p.store.InsertPerks(ctx, perks); err != nil {
		return err
	}

	// Write teams + bans.
	for _, team := range info.Teams {
		featsJSON, _ := json.Marshal(team.Feats)

		obj := team.Objectives
		if err := p.store.InsertTeam(ctx, matchID, team.TeamID, team.Win,
			obj.Baron.Kills, obj.Dragon.Kills, obj.Tower.Kills,
			obj.Inhibitor.Kills, obj.RiftHerald.Kills, featsJSON); err != nil {
			return err
		}

		bans := make([]struct{ ChampionID, PickTurn int }, len(team.Bans))
		for i, b := range team.Bans {
			bans[i] = struct{ ChampionID, PickTurn int }{b.ChampionID, b.PickTurn}
		}
		if err := p.store.InsertBans(ctx, matchID, team.TeamID, bans); err != nil {
			return err
		}
	}

	return nil
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

func perkRowFromDTO(matchID, puuid string, perks *riotapi.PerksDTO) storage.PerkRow {
	row := storage.PerkRow{
		MatchID:     matchID,
		PUUID:       puuid,
		StatDefense: perks.StatPerks.Defense,
		StatFlex:    perks.StatPerks.Flex,
		StatOffense: perks.StatPerks.Offense,
	}
	for i, style := range perks.Styles {
		switch i {
		case 0:
			row.Style0 = style.Style
		case 1:
			row.Style1 = style.Style
		}
		for j, sel := range style.Selections {
			idx := i*3 + j
			if idx < 6 {
				row.Perk[idx] = sel.Perk
				row.Vars[idx] = [3]int{sel.Var1, sel.Var2, sel.Var3}
			}
		}
	}
	return row
}

func ptr[T any](v T) *T { return &v }
