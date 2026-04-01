package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type MatchSeed struct {
	MatchID      string
	EndOfGameResult string
	GameCreation    time.Time
	GameDuration    time.Duration
	GameEndTime     time.Time
	GameID          int64
	GameMode        string
	GameName        string
	GameStartTime   time.Time
	GameType        string
	GameVersion     string
	MapID           int
	PlatformID      string
	QueueID         int
	TournamentCode  string
}

type MatchTeamSeed struct {
	MatchID    string
	TeamID     int
	Win        bool
	Objectives struct {
		Atakhan struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"atakhan"`
		Baron struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"baron"`
		Champion struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"champion"`
		Dragon struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"dragon"`
		Horde struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"horde"`
		Inhibitor struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"inhibitor"`
		RiftHerald struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"riftHerald"`
		Tower struct {
			First bool `json:"first"`
			Kills int  `json:"kills"`
		} `json:"tower"`
	} // `json:"objectives"`
	Feats struct {
		EpicMonsterKill struct {
			FeatState int `json:"featState"`
		} `json:"EPIC_MONSTER_KILL"`
		FirstBlood struct {
			FeatState int `json:"featState"`
		} `json:"FIRST_BLOOD"`
		FirstTurret struct {
			FeatState int `json:"featState"`
		} `json:"FIRST_TURRET"`
	} // `json:"feats"`
}

type BanSeed struct {
	MatchID    string
	TeamID     int
	PickTurn   int
	ChampionID int
}

type MatchParticipantSeed struct {
	MatchID string
	ParticipantID int
	Puuid string
	TeamID int

	ChampionID int
	ChampionName string
	TeamPosition string
	IndividualPosition string
	Lane string
	Role string

	Win bool

	Kills int
	Deaths int
	Assists int

	GoldEarned int
	GoldSpent int
	ChampLevel int
	TotalMinionsKilled int
	NeutralMinionsKilled int

	TotalDamageDealtToChampions int
	PhysicalDamageDealtToChampions int
	MagicDamageDealtToChampions int
	TrueDamageDealtToChampions int
	TotalDamageTaken int
	DamageSelfMitigated int
	TotalHeal int

	VisionScore int
	WardsPlaced int
	WardsKilled int
	VisionWardsBoughtInGame int

	Item0 int
	Item1 int
	Item2 int
	Item3 int
	Item4 int
	Item5 int
	Item6 int
	Summoner1ID int
	Summoner2ID int

}

type MatchParticipantPerksExtSeed struct {
	MatchID string
	ParticipantID int

	ChampionID int
	Win bool

	PrimaryStyle string
	SubStyle string

	Perk0 int
	Perk1 int
	Perk2 int
	Perk3 int
	Perk4 int
	Perk5 int

	StatDefense int
	StatFlex int
	StatOffense int
}


// 更新之前已经在数据库里面但只有match_id的比赛记录，补充其它字段数据
func (s *Store) UpdateMatch(ctx context.Context, seed MatchSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if seed.MatchID == "" {
		return fmt.Errorf("match_id is required")
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE matches
		SET end_of_game_result = $2,
			game_creation = $3,
			game_duration = $4,
			game_end_time = $5,
			game_id = $6,
			game_mode = $7,
			game_name = $8,
			game_start_time = $9,
			game_type = $10,
			game_version = $11,
			map_id = $12,
			platform_id = $13,
			queue_id = $14,
			tournament_code = $15,
			processed_at = NOW()
		WHERE match_id = $1
	`, seed.MatchID, seed.EndOfGameResult, seed.GameCreation, seed.GameDuration, seed.GameEndTime,
		seed.GameID, seed.GameMode, seed.GameName, seed.GameStartTime, seed.GameType, seed.GameVersion,
		seed.MapID, seed.PlatformID, seed.QueueID, seed.TournamentCode)
	if err != nil {
		return fmt.Errorf("update match detail for match_id %s: %w", seed.MatchID, err)
	}

	return nil
}

// 插入MatchTeams,如果已经存在则跳过
func (s *Store) InsertMatchTeams(ctx context.Context, seeds []MatchTeamSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if len(seeds) == 0 {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fmt.Printf("开始批量插入比赛队伍数据: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.MatchID == "" {
			continue
		}

		_, err := tx.Exec(ctx, `
INSERT INTO match_teams (match_id, team_id, win, objectives, feats)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (match_id, team_id) DO UPDATE
SET win = EXCLUDED.win,
	objectives = EXCLUDED.objectives,
	feats = EXCLUDED.feats;
`, seed.MatchID, seed.TeamID, seed.Win, seed.Objectives, seed.Feats)
		if err != nil {
			return fmt.Errorf("insert match team for match_id %s team_id %d: %w", seed.MatchID, seed.TeamID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *Store) InsertBans(ctx context.Context, seeds []BanSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if len(seeds) == 0 {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fmt.Printf("开始批量插入比赛禁用数据: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.MatchID == "" {
			continue
		}
		
		_, err := tx.Exec(ctx, `
INSERT INTO bans (match_id, team_id, champion_id, pick_turn)
VALUES ($1, $2, $3, $4)
ON CONFLICT (match_id, team_id, pick_turn) DO NOTHING;
`, seed.MatchID, seed.TeamID, seed.ChampionID, seed.PickTurn)
		if err != nil {
			return fmt.Errorf("insert ban for match_id %s team_id %d champion_id %d: %w", seed.MatchID, seed.TeamID, seed.ChampionID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *Store) InsertMatchParticipants(ctx context.Context, seeds []MatchParticipantSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if len(seeds) == 0 {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fmt.Printf("开始批量插入比赛参与者数据: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.MatchID == "" {
			continue
		}

		_, err := tx.Exec(ctx, `
INSERT INTO match_participants (
  match_id, participant_id, puuid, team_id, champion_id, champion_name, team_position, individual_position, lane, role, win,
  kills, deaths, assists, gold_earned, gold_spent, champ_level,
  total_minions_killed, neutral_minions_killed,
  total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions,
  total_damage_taken, damage_self_mitigated, total_heal,
  vision_score, wards_placed, wards_killed, vision_wards_bought_in_game,
  item0, item1, item2, item3, item4, item5, item6,
  summoner1_id, summoner2_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
  $12, $13, $14, $15, $16, $17,
  $18, $19,
  $20, $21, $22, $23,
  $24, $25, $26,
  $27, $28, $29, $30,
  $31, $32, $33, $34, $35, $36, $37,
  $38, $39
)
ON CONFLICT (match_id, participant_id) DO UPDATE
SET puuid = CASE WHEN EXCLUDED.puuid = '' THEN match_participants.puuid ELSE EXCLUDED.puuid END,
	team_id = EXCLUDED.team_id,
	champion_id = EXCLUDED.champion_id,
	champion_name = EXCLUDED.champion_name,
	team_position = EXCLUDED.team_position,
	individual_position = EXCLUDED.individual_position,
	lane = EXCLUDED.lane,
	role = EXCLUDED.role,
	win = EXCLUDED.win,
	kills = EXCLUDED.kills,
	deaths = EXCLUDED.deaths,
	assists = EXCLUDED.assists,
	gold_earned = EXCLUDED.gold_earned,
	gold_spent = EXCLUDED.gold_spent,
	champ_level = EXCLUDED.champ_level,
	total_minions_killed = EXCLUDED.total_minions_killed,
	neutral_minions_killed = EXCLUDED.neutral_minions_killed,
	total_damage_dealt_to_champions = EXCLUDED.total_damage_dealt_to_champions,
	physical_damage_dealt_to_champions = EXCLUDED.physical_damage_dealt_to_champions,
	magic_damage_dealt_to_champions = EXCLUDED.magic_damage_dealt_to_champions,
	true_damage_dealt_to_champions = EXCLUDED.true_damage_dealt_to_champions,
	total_damage_taken = EXCLUDED.total_damage_taken,
	damage_self_mitigated = EXCLUDED.damage_self_mitigated,
	total_heal = EXCLUDED.total_heal,
	vision_score = EXCLUDED.vision_score,
	wards_placed = EXCLUDED.wards_placed,
	wards_killed = EXCLUDED.wards_killed,
	vision_wards_bought_in_game = EXCLUDED.vision_wards_bought_in_game,
	item0 = EXCLUDED.item0,
	item1 = EXCLUDED.item1,
	item2 = EXCLUDED.item2,
	item3 = EXCLUDED.item3,
	item4 = EXCLUDED.item4,
	item5 = EXCLUDED.item5,
	item6 = EXCLUDED.item6,
	summoner1_id = EXCLUDED.summoner1_id,
	summoner2_id = EXCLUDED.summoner2_id;
`, seed.MatchID, seed.ParticipantID, seed.Puuid, seed.TeamID, seed.ChampionID, seed.ChampionName, seed.TeamPosition, seed.IndividualPosition, seed.Lane, seed.Role, seed.Win,
	seed.Kills, seed.Deaths, seed.Assists, seed.GoldEarned, seed.GoldSpent, seed.ChampLevel,
	seed.TotalMinionsKilled, seed.NeutralMinionsKilled,
	seed.TotalDamageDealtToChampions, seed.PhysicalDamageDealtToChampions, seed.MagicDamageDealtToChampions, seed.TrueDamageDealtToChampions,
	seed.TotalDamageTaken, seed.DamageSelfMitigated, seed.TotalHeal,
	seed.VisionScore, seed.WardsPlaced, seed.WardsKilled, seed.VisionWardsBoughtInGame,
	seed.Item0, seed.Item1, seed.Item2, seed.Item3, seed.Item4, seed.Item5, seed.Item6,
	seed.Summoner1ID, seed.Summoner2ID,)
		if err != nil {
			return fmt.Errorf("insert match participant for match_id %s participant_id %d: %w", seed.MatchID, seed.ParticipantID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *Store) InsertMatchParticipantPerksExt(ctx context.Context, seeds []MatchParticipantPerksExtSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if len(seeds) == 0 {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fmt.Printf("开始批量插入比赛参与者符文数据: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.MatchID == "" {
			continue
		}

		_, err := tx.Exec(ctx, `
INSERT INTO match_participant_perks_ext (
  match_id, participant_id, champion_id, win,
  primary_style, sub_style,
  perk_0, perk_1, perk_2, perk_3, perk_4, perk_5,
  stat_defense, stat_flex, stat_offense
) VALUES (
  $1, $2, $3, $4,
  $5, $6,
  $7, $8, $9, $10, $11, $12,
  $13, $14, $15
)
ON CONFLICT (match_id, participant_id) DO UPDATE
SET champion_id = EXCLUDED.champion_id,
	win = EXCLUDED.win,
	primary_style = EXCLUDED.primary_style,
	sub_style = EXCLUDED.sub_style,
	perk_0 = EXCLUDED.perk_0,
	perk_1 = EXCLUDED.perk_1,
	perk_2 = EXCLUDED.perk_2,
	perk_3 = EXCLUDED.perk_3,
	perk_4 = EXCLUDED.perk_4,
	perk_5 = EXCLUDED.perk_5,
	stat_defense = EXCLUDED.stat_defense,
	stat_flex = EXCLUDED.stat_flex,
	stat_offense = EXCLUDED.stat_offense;
`, seed.MatchID, seed.ParticipantID, seed.ChampionID, seed.Win,
	seed.PrimaryStyle, seed.SubStyle,
	seed.Perk0, seed.Perk1, seed.Perk2, seed.Perk3, seed.Perk4, seed.Perk5,
	seed.StatDefense, seed.StatFlex, seed.StatOffense,
	)
		if err != nil {
			return fmt.Errorf("insert match participant perks ext for match_id %s participant_id %d: %w", seed.MatchID, seed.ParticipantID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
