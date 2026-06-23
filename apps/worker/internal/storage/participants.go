package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Participant struct {
	MatchID       string
	PUUID         *string
	ParticipantID int

	TierAtMatch        *string
	DivisionAtMatch    *string
	LPAtMatch          *int
	TierSnapshotDeltaH *int

	SummonerLevel      int
	TeamID             int
	TeamPosition       string
	IndividualPosition string
	Lane               string
	Role               string
	Win                bool

	ChampionID        int
	ChampionName      string
	ChampLevel        int
	ChampExperience   int
	ChampionTransform int

	PlayerAugment1, PlayerAugment2, PlayerAugment3 int
	PlayerAugment4, PlayerAugment5, PlayerAugment6 int

	Summoner1ID, Summoner2ID                           int
	Summoner1Casts, Summoner2Casts                     int
	Spell1Casts, Spell2Casts, Spell3Casts, Spell4Casts int

	Kills, Deaths, Assists                     int
	DoubleKills, TripleKills, QuadraKills      int
	PentaKills, UnrealKills                    int
	KillingSprees, LargestKillingSpree         int
	LargestMultiKill                           int
	FirstBloodKill, FirstBloodAssist           bool
	LongestTimeSpentLiving, TotalTimeSpentDead int

	TotalDamageDealt, TotalDamageDealtToChampions          int
	PhysicalDamageDealt, PhysicalDamageDealtToChampions    int
	MagicDamageDealt, MagicDamageDealtToChampions          int
	TrueDamageDealt, TrueDamageDealtToChampions            int
	LargestCriticalStrike                                  int
	TotalDamageTaken, PhysicalDamageTaken                  int
	MagicDamageTaken, TrueDamageTaken, DamageSelfMitigated int
	TotalHeal, TotalHealsOnTeammates, TotalUnitsHealed     int
	TotalDamageShieldedOnTeammates                         int
	TimeCCingOthers, TotalTimeCCDealt                      int

	GoldEarned, GoldSpent                               int
	Item0, Item1, Item2, Item3, Item4, Item5, Item6     int
	ItemsPurchased, ConsumablesPurchased, RoleBoundItem int

	TotalMinionsKilled, NeutralMinionsKilled        int
	TotalAllyJungleMinions, TotalEnemyJungleMinions int
	BaronKills, DragonKills                         int
	ObjectivesStolen, ObjectivesStolenAssists       int

	VisionScore, VisionWardsBought, SightWardsBought int
	WardsPlaced, DetectorWardsPlaced, WardsKilled    int

	TurretKills, TurretTakedowns, TurretsLost          int
	FirstTowerKill, FirstTowerAssist                   bool
	InhibitorKills, InhibitorTakedowns, InhibitorsLost int
	NexusKills, NexusLost, NexusTakedowns              int
	DamageDealtToObjectives, DamageDealtToBuildings    int
	DamageDealtToTurrets, DamageDealtToEpicMonsters    int

	AllInPings, BasicPings, AssistMePings, CommandPings     int
	DangerPings, EnemyMissingPings, GetBackPings, HoldPings int
	OnMyWayPings, NeedVisionPings, PushPings, RetreatPings  int
	EnemyVisionPings, VisionClearedPings                    int

	GameEndedInEarlySurrender, GameEndedInSurrender bool
	TeamEarlySurrendered                            bool
	TimePlayed                                      int
}

// InsertParticipants bulk-inserts all participants for a match in a single batch.
func (s *Store) InsertParticipants(ctx context.Context, participants []Participant) error {
	batch := &pgx.Batch{}
	for i := range participants {
		p := &participants[i]
		batch.Queue(`
			INSERT INTO match_participants (
				match_id, puuid, participant_id,
				tier_at_match, division_at_match, lp_at_match, tier_snapshot_delta_h,
				summoner_level, team_id, team_position, individual_position, lane, role, win,
				champion_id, champion_name, champ_level, champ_experience, champion_transform,
				player_augment1, player_augment2, player_augment3,
				player_augment4, player_augment5, player_augment6,
				summoner1_id, summoner2_id, summoner1_casts, summoner2_casts,
				spell1_casts, spell2_casts, spell3_casts, spell4_casts,
				kills, deaths, assists,
				double_kills, triple_kills, quadra_kills, penta_kills, unreal_kills,
				killing_sprees, largest_killing_spree, largest_multi_kill,
				first_blood_kill, first_blood_assist,
				longest_time_spent_living, total_time_spent_dead,
				total_damage_dealt, total_damage_dealt_to_champions,
				physical_damage_dealt, physical_damage_dealt_to_champions,
				magic_damage_dealt, magic_damage_dealt_to_champions,
				true_damage_dealt, true_damage_dealt_to_champions,
				largest_critical_strike,
				total_damage_taken, physical_damage_taken, magic_damage_taken,
				true_damage_taken, damage_self_mitigated,
				total_heal, total_heals_on_teammates, total_units_healed,
				total_damage_shielded_on_teammates, time_ccing_others, total_time_cc_dealt,
				gold_earned, gold_spent,
				item0, item1, item2, item3, item4, item5, item6,
				items_purchased, consumables_purchased, role_bound_item,
				total_minions_killed, neutral_minions_killed,
				total_ally_jungle_minions, total_enemy_jungle_minions,
				baron_kills, dragon_kills, objectives_stolen, objectives_stolen_assists,
				vision_score, vision_wards_bought, sight_wards_bought,
				wards_placed, detector_wards_placed, wards_killed,
				turret_kills, turret_takedowns, turrets_lost,
				first_tower_kill, first_tower_assist,
				inhibitor_kills, inhibitor_takedowns, inhibitors_lost,
				nexus_kills, nexus_lost, nexus_takedowns,
				damage_dealt_to_objectives, damage_dealt_to_buildings,
				damage_dealt_to_turrets, damage_dealt_to_epic_monsters,
				all_in_pings, basic_pings, assist_me_pings, command_pings,
				danger_pings, enemy_missing_pings, get_back_pings, hold_pings,
				on_my_way_pings, need_vision_pings, push_pings, retreat_pings,
				enemy_vision_pings, vision_cleared_pings,
				game_ended_in_early_surrender, game_ended_in_surrender,
				team_early_surrendered, time_played
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
				$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,
				$37,$38,$39,$40,$41,$42,$43,$44,$45,$46,$47,$48,$49,$50,$51,$52,$53,
				$54,$55,$56,$57,$58,$59,$60,$61,$62,$63,$64,$65,$66,$67,$68,$69,$70,
				$71,$72,$73,$74,$75,$76,$77,$78,$79,$80,$81,$82,$83,$84,$85,$86,$87,
				$88,$89,$90,$91,$92,$93,$94,$95,$96,$97,$98,$99,$100,$101,$102,$103,
				$104,$105,$106,$107,$108,$109,$110,$111,$112,$113,$114,
				$115,$116,$117,$118,$119,$120,$121,$122,$123,$124,$125,$126,$127
			) ON CONFLICT DO NOTHING`,
			p.MatchID, p.PUUID, p.ParticipantID,
			p.TierAtMatch, p.DivisionAtMatch, p.LPAtMatch, p.TierSnapshotDeltaH,
			p.SummonerLevel, p.TeamID, p.TeamPosition, p.IndividualPosition, p.Lane, p.Role, p.Win,
			p.ChampionID, p.ChampionName, p.ChampLevel, p.ChampExperience, p.ChampionTransform,
			p.PlayerAugment1, p.PlayerAugment2, p.PlayerAugment3,
			p.PlayerAugment4, p.PlayerAugment5, p.PlayerAugment6,
			p.Summoner1ID, p.Summoner2ID, p.Summoner1Casts, p.Summoner2Casts,
			p.Spell1Casts, p.Spell2Casts, p.Spell3Casts, p.Spell4Casts,
			p.Kills, p.Deaths, p.Assists,
			p.DoubleKills, p.TripleKills, p.QuadraKills, p.PentaKills, p.UnrealKills,
			p.KillingSprees, p.LargestKillingSpree, p.LargestMultiKill,
			p.FirstBloodKill, p.FirstBloodAssist,
			p.LongestTimeSpentLiving, p.TotalTimeSpentDead,
			p.TotalDamageDealt, p.TotalDamageDealtToChampions,
			p.PhysicalDamageDealt, p.PhysicalDamageDealtToChampions,
			p.MagicDamageDealt, p.MagicDamageDealtToChampions,
			p.TrueDamageDealt, p.TrueDamageDealtToChampions,
			p.LargestCriticalStrike,
			p.TotalDamageTaken, p.PhysicalDamageTaken, p.MagicDamageTaken,
			p.TrueDamageTaken, p.DamageSelfMitigated,
			p.TotalHeal, p.TotalHealsOnTeammates, p.TotalUnitsHealed,
			p.TotalDamageShieldedOnTeammates, p.TimeCCingOthers, p.TotalTimeCCDealt,
			p.GoldEarned, p.GoldSpent,
			p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5, p.Item6,
			p.ItemsPurchased, p.ConsumablesPurchased, p.RoleBoundItem,
			p.TotalMinionsKilled, p.NeutralMinionsKilled,
			p.TotalAllyJungleMinions, p.TotalEnemyJungleMinions,
			p.BaronKills, p.DragonKills, p.ObjectivesStolen, p.ObjectivesStolenAssists,
			p.VisionScore, p.VisionWardsBought, p.SightWardsBought,
			p.WardsPlaced, p.DetectorWardsPlaced, p.WardsKilled,
			p.TurretKills, p.TurretTakedowns, p.TurretsLost,
			p.FirstTowerKill, p.FirstTowerAssist,
			p.InhibitorKills, p.InhibitorTakedowns, p.InhibitorsLost,
			p.NexusKills, p.NexusLost, p.NexusTakedowns,
			p.DamageDealtToObjectives, p.DamageDealtToBuildings,
			p.DamageDealtToTurrets, p.DamageDealtToEpicMonsters,
			p.AllInPings, p.BasicPings, p.AssistMePings, p.CommandPings,
			p.DangerPings, p.EnemyMissingPings, p.GetBackPings, p.HoldPings,
			p.OnMyWayPings, p.NeedVisionPings, p.PushPings, p.RetreatPings,
			p.EnemyVisionPings, p.VisionClearedPings,
			p.GameEndedInEarlySurrender, p.GameEndedInSurrender,
			p.TeamEarlySurrendered, p.TimePlayed,
		)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

// UpdateParticipantTier backfills tier info for a single participant.
func (s *Store) UpdateParticipantTier(ctx context.Context, matchID, puuid, tier, division string, lp, deltaH int) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE match_participants
		SET tier_at_match = $1, division_at_match = $2, lp_at_match = $3, tier_snapshot_delta_h = $4
		WHERE match_id = $5 AND puuid = $6`,
		tier, division, lp, deltaH, matchID, puuid)
	return err
}

// GetParticipantsMissingTier returns distinct puuids that have at least one
// match_participants row with tier_at_match IS NULL, scoped to the given region.
func (s *Store) GetParticipantsMissingTier(ctx context.Context, region string, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT mp.puuid
		FROM match_participants mp
		JOIN matches m ON m.match_id = mp.match_id
		WHERE mp.tier_at_match IS NULL AND mp.puuid IS NOT NULL
		  AND m.region = $1
		LIMIT $2`, region, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var puuids []string
	for rows.Next() {
		var puuid string
		if err := rows.Scan(&puuid); err != nil {
			return nil, err
		}
		puuids = append(puuids, puuid)
	}
	return puuids, rows.Err()
}

// MarkParticipantUnranked sets tier_at_match = 'UNRANKED' for all participants
// with this puuid where tier_at_match is currently null, so phase35 will not
// query the Riot API for them again in future runs.
func (s *Store) MarkParticipantUnranked(ctx context.Context, puuid string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE match_participants
		SET tier_at_match = 'UNRANKED'
		WHERE puuid = $1 AND tier_at_match IS NULL`, puuid)
	return err
}

// UpdateParticipantTierByPUUID backfills tier info for ALL participants
// with the given puuid where tier_at_match is currently null.
// tier_snapshot_delta_h is computed per-match as the hours between
// the snapshot time (now) and each match's game_start_ts.
func (s *Store) UpdateParticipantTierByPUUID(ctx context.Context, puuid, tier, division string, lp int) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE match_participants mp
		SET tier_at_match        = $1,
		    division_at_match    = $2,
		    lp_at_match          = $3,
		    tier_snapshot_delta_h = ROUND(ABS(EXTRACT(EPOCH FROM (NOW() - m.game_start_ts)) / 3600))::int
		FROM matches m
		WHERE mp.match_id = m.match_id
		  AND mp.puuid = $4
		  AND mp.tier_at_match IS NULL`,
		tier, division, lp, puuid)
	return err
}

type PerkRow struct {
	MatchID     string
	PUUID       string
	StatDefense int
	StatFlex    int
	StatOffense int
	Style0      int
	Style1      int
	Perk        [6]int
	Vars        [6][3]int // Vars[perkIdx][varIdx]
}

// InsertPerks bulk-inserts perk rows.
func (s *Store) InsertPerks(ctx context.Context, perks []PerkRow) error {
	batch := &pgx.Batch{}
	for _, p := range perks {
		batch.Queue(`
			INSERT INTO match_perks
			  (match_id, puuid, stat_defense, stat_flex, stat_offense,
			   style0, style1,
			   perk0, perk1, perk2, perk3, perk4, perk5,
			   var01, var02,
			   var11, var12, var13,
			   var21, var22, var23,
			   var31, var32, var33,
			   var41, var42, var43,
			   var51, var52, var53)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
			        $14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,
			        $25,$26,$27,$28,$29,$30)
			ON CONFLICT DO NOTHING`,
			p.MatchID, p.PUUID, p.StatDefense, p.StatFlex, p.StatOffense,
			p.Style0, p.Style1,
			p.Perk[0], p.Perk[1], p.Perk[2], p.Perk[3], p.Perk[4], p.Perk[5],
			p.Vars[0][0], p.Vars[0][1],
			p.Vars[1][0], p.Vars[1][1], p.Vars[1][2],
			p.Vars[2][0], p.Vars[2][1], p.Vars[2][2],
			p.Vars[3][0], p.Vars[3][1], p.Vars[3][2],
			p.Vars[4][0], p.Vars[4][1], p.Vars[4][2],
			p.Vars[5][0], p.Vars[5][1], p.Vars[5][2],
		)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

// InsertBans bulk-inserts ban rows for a match.
func (s *Store) InsertBans(ctx context.Context, matchID string, teamID int, bans []struct{ ChampionID, PickTurn int }) error {
	batch := &pgx.Batch{}
	for _, b := range bans {
		batch.Queue(`
			INSERT INTO match_bans (match_id, team_id, pick_turn, champion_id)
			VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			matchID, teamID, b.PickTurn, b.ChampionID)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

// InsertTeam inserts a match_teams row.
func (s *Store) InsertTeam(ctx context.Context, matchID string, teamID int, win bool,
	baronKills, dragonKills, towerKills, inhibitorKills, riftHeraldKills int,
	feats []byte) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO match_teams
		  (match_id, team_id, win, baron_kills, dragon_kills,
		   tower_kills, inhibitor_kills, rift_herald_kills, feats)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT DO NOTHING`,
		matchID, teamID, win, baronKills, dragonKills,
		towerKills, inhibitorKills, riftHeraldKills, feats)
	return err
}

// GetParticipantTiersForMatch returns all (puuid, tier_score) pairs for a match where tier is known.
func (s *Store) GetParticipantTiersForMatch(ctx context.Context, matchID string) ([]struct {
	PUUID        string
	Tier         string
	Division     *string
	LeaguePoints *int
}, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT puuid, tier_at_match, division_at_match, lp_at_match
		FROM match_participants
		WHERE match_id = $1 AND tier_at_match IS NOT NULL AND tier_at_match != 'UNRANKED'`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		PUUID        string
		Tier         string
		Division     *string
		LeaguePoints *int
	}
	for rows.Next() {
		var r struct {
			PUUID        string
			Tier         string
			Division     *string
			LeaguePoints *int
		}
		if err := rows.Scan(&r.PUUID, &r.Tier, &r.Division, &r.LeaguePoints); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
