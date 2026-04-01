package process

import (
	"context"
	"time"
	"fmt"

	"crawler-test/internal/crawler"
	"crawler-test/internal/riot"
	"crawler-test/internal/storage"
)

func (p *ResultProcessor) processMatchDetail(ctx context.Context, task crawler.Task, dto *riot.MatchDetailDTO) error {

	fmt.Printf("处理比赛详情: task_id=%s\n", task.ID)

	// 1. 准备更新matches表的种子数据
	matchSeed := buildMatchSeed(task.ID, dto)
	fmt.Printf("生成比赛详情种子: task_id=%s match_id=%d\n", task.ID, matchSeed.MatchID)

	// 2. 准备更新match_teams表的种子数据
	teamSeeds := buildMatchTeamSeeds(task.ID, dto)
	fmt.Printf("生成比赛队伍种子: task_id=%s match_id=%d teams=%d\n", task.ID, matchSeed.MatchID, len(teamSeeds))

	// 3. 准备更新bans表的种子数据
	banSeeds := buildBanSeeds(task.ID, dto)
	fmt.Printf("生成比赛禁用种子: task_id=%s match_id=%d bans=%d\n", task.ID, matchSeed.MatchID, len(banSeeds))

	// 4. 准备更新match_participants表的种子数据
	participantSeeds := buildMatchParticipantSeeds(task.ID, dto)
	fmt.Printf("生成比赛参与者种子: task_id=%s match_id=%d participants=%d\n", task.ID, matchSeed.MatchID, len(participantSeeds))

	// 5. 准备更新match_perks_ext表的种子数据
	perksExtSeeds := buildMatchPerksExtSeeds(task.ID, dto)
	fmt.Printf("生成比赛符文种子: task_id=%s match_id=%d perks_ext=%d\n", task.ID, matchSeed.MatchID, len(perksExtSeeds))
	
	// 6. 用每种种子更新数据库
	if err := p.store.UpdateMatch(ctx, matchSeed); err != nil {
		return fmt.Errorf("更新比赛详情失败: %w", err)
	}

	if err := p.store.InsertMatchTeams(ctx, teamSeeds); err != nil {
		return fmt.Errorf("更新比赛队伍失败: %w", err)
	}

	if err := p.store.InsertBans(ctx, banSeeds); err != nil {
		return fmt.Errorf("更新比赛禁用失败: %w", err)
	}

	if err := p.store.InsertMatchParticipants(ctx, participantSeeds); err != nil {
		return fmt.Errorf("更新比赛参与者失败: %w", err)
	}

	if err := p.store.InsertMatchParticipantPerksExt(ctx, perksExtSeeds); err != nil {
		return fmt.Errorf("更新比赛符文失败: %w", err)
	}

	return nil
}

func buildMatchSeed(taskID string, dto *riot.MatchDetailDTO) storage.MatchSeed {
	if dto == nil {
		return storage.MatchSeed{}
	}

	return storage.MatchSeed{
		MatchID:       dto.Metadata.MatchID,
		EndOfGameResult: dto.Info.EndOfGameResult,
		GameCreation:   time.UnixMilli(dto.Info.GameCreation),
		GameDuration:   time.Duration(dto.Info.GameDuration) * time.Second,
		GameEndTime: time.UnixMilli(dto.Info.GameEndTimestamp),
		GameID: 	   dto.Info.GameID,
		GameMode: 	   dto.Info.GameMode,
		GameName: 	   dto.Info.GameName,
		GameStartTime: time.UnixMilli(dto.Info.GameStartTimestamp),
		GameType: 	   dto.Info.GameType,
		GameVersion:   dto.Info.GameVersion,
		MapID: 	   dto.Info.MapID,
		PlatformID:    dto.Info.PlatformID,
		QueueID: 	   dto.Info.QueueID,
		TournamentCode: dto.Info.TournamentCode,
	}
}

func buildMatchTeamSeeds(taskID string, dto *riot.MatchDetailDTO) []storage.MatchTeamSeed {
	if dto == nil || dto.Info.Teams == nil {
		return nil
	}

	seeds := make([]storage.MatchTeamSeed, 0, len(dto.Info.Teams))
	for _, team := range dto.Info.Teams {
		seeds = append(seeds, storage.MatchTeamSeed{
			MatchID:   	dto.Metadata.MatchID,
			TeamID:    	team.TeamID,
			Win:       	team.Win,
			Objectives: team.Objectives,
			Feats: 		team.Feats,
		})
	}

	return seeds
}

func buildBanSeeds(taskID string, dto *riot.MatchDetailDTO) []storage.BanSeed {
	if dto == nil || dto.Info.Teams == nil {
		return nil
	}
	
	seeds := make([]storage.BanSeed, 0)
	for _, team := range dto.Info.Teams {
		for _, ban := range team.Bans {
			seeds = append(seeds, storage.BanSeed{
				MatchID:   	dto.Metadata.MatchID,
				TeamID:    	team.TeamID,
				ChampionID: ban.ChampionID,
				PickTurn:   ban.PickTurn,
			})
		}
	}
	
	return seeds
}

func buildMatchParticipantSeeds(taskID string, dto *riot.MatchDetailDTO) []storage.MatchParticipantSeed {
	if dto == nil || dto.Info.Participants == nil {
		return nil
	}

	seeds := make([]storage.MatchParticipantSeed, 0, len(dto.Info.Participants))
	for _, p := range dto.Info.Participants {
		seeds = append(seeds, storage.MatchParticipantSeed{
			MatchID:           	dto.Metadata.MatchID,
			ParticipantID:     	p.ParticipantID,
			Puuid:             	p.Puuid,
			TeamID:            	p.TeamID,
			
			ChampionID:        	p.ChampionID,
			ChampionName:      	p.ChampionName,
			TeamPosition:      	p.TeamPosition,
			IndividualPosition:  p.IndividualPosition,
			Lane:              	p.Lane,
			Role:              	p.Role,
			
			Win:               	p.Win,
			
			Kills:             	p.Kills,
			Deaths:            	p.Deaths,
			Assists:           	p.Assists,
			
			GoldEarned:       	p.GoldEarned,
			GoldSpent:  		p.GoldSpent,
			ChampLevel:		   	p.ChampLevel,
			TotalMinionsKilled:   	p.TotalMinionsKilled,
			NeutralMinionsKilled: 	p.NeutralMinionsKilled,
		
			TotalDamageDealtToChampions: p.TotalDamageDealtToChampions,
			PhysicalDamageDealtToChampions: p.PhysicalDamageDealtToChampions,
			MagicDamageDealtToChampions: p.MagicDamageDealtToChampions,
			TrueDamageDealtToChampions: p.TrueDamageDealtToChampions,
			TotalDamageTaken: p.TotalDamageTaken,
			DamageSelfMitigated: p.DamageSelfMitigated,
			
			VisionScore: p.VisionScore,
			WardsPlaced: p.WardsPlaced,
			WardsKilled: p.WardsKilled,
			VisionWardsBoughtInGame: p.VisionWardsBoughtInGame,

	
			Item0: p.Item0,
			Item1: p.Item1,
			Item2: p.Item2,
			Item3: p.Item3,
			Item4: p.Item4,
			Item5: p.Item5,
			Item6: p.Item6,

			Summoner1ID: p.Summoner1ID,
			Summoner2ID: p.Summoner2ID,
		})
	}

	return seeds
}

func buildMatchPerksExtSeeds(taskID string, dto *riot.MatchDetailDTO) []storage.MatchParticipantPerksExtSeed {
	if dto == nil || dto.Info.Participants == nil {
		return nil
	}

	seeds := make([]storage.MatchParticipantPerksExtSeed, 0)
	for _, p := range dto.Info.Participants {
		if len(p.Perks.Styles) < 2 || len(p.Perks.Styles[0].Selections) < 4 || len(p.Perks.Styles[1].Selections) < 2 {
			fmt.Printf("符文数据不完整，跳过: match_id=%s participant_id=%d\n", dto.Metadata.MatchID, p.ParticipantID)
			continue
		}
		seeds = append(seeds, storage.MatchParticipantPerksExtSeed{
			MatchID:      	dto.Metadata.MatchID,
			ParticipantID: 	p.ParticipantID,

			ChampionID: 	 	p.ChampionID,
			Win: 			 	p.Win,

			PrimaryStyle: p.Perks.Styles[0].Description,
			SubStyle: p.Perks.Styles[1].Description,

			Perk0: p.Perks.Styles[0].Selections[0].Perk,
			Perk1: p.Perks.Styles[0].Selections[1].Perk,
			Perk2: p.Perks.Styles[0].Selections[2].Perk,
			Perk3: p.Perks.Styles[0].Selections[3].Perk,
			Perk4: p.Perks.Styles[1].Selections[0].Perk,
			Perk5: p.Perks.Styles[1].Selections[1].Perk,

			StatDefense: p.Perks.StatPerks.Defense,
			StatFlex: p.Perks.StatPerks.Flex,
			StatOffense: p.Perks.StatPerks.Offense,
		})
	}

	return seeds

}

