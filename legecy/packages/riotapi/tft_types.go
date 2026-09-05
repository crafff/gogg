package riotapi

// TFTLeagueListDTO is returned by the challenger, grandmaster and master
// TFT league endpoints.
type TFTLeagueListDTO struct {
	LeagueID string              `json:"leagueId"`
	Tier     string              `json:"tier"`
	Queue    string              `json:"queue"`
	Name     string              `json:"name"`
	Entries  []TFTLeagueEntryDTO `json:"entries"`
}

type TFTLeagueEntryDTO struct {
	LeagueID     string `json:"leagueId"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	Puuid        string `json:"puuid"`
	SummonerID   string `json:"summonerId"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	Veteran      bool   `json:"veteran"`
	Inactive     bool   `json:"inactive"`
	FreshBlood   bool   `json:"freshBlood"`
	HotStreak    bool   `json:"hotStreak"`
}

type TFTSummonerDTO struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	Puuid         string `json:"puuid"`
	ProfileIconID int    `json:"profileIconId"`
	RevisionDate  int64  `json:"revisionDate"`
	SummonerLevel int64  `json:"summonerLevel"`
}

type TFTMatchDTO struct {
	Metadata TFTMatchMetadataDTO `json:"metadata"`
	Info     TFTMatchInfoDTO     `json:"info"`
}

type TFTMatchMetadataDTO struct {
	DataVersion  string   `json:"data_version"`
	MatchID      string   `json:"match_id"`
	Participants []string `json:"participants"`
}

type TFTMatchInfoDTO struct {
	EndOfGameResult string              `json:"end_of_game_result"`
	GameCreation    int64               `json:"game_creation"`
	GameDatetime    int64               `json:"game_datetime"`
	GameLength      float64             `json:"game_length"`
	GameVersion     string              `json:"game_version"`
	MapID           int                 `json:"map_id"`
	QueueID         int                 `json:"queue_id"`
	TFTGameType     string              `json:"tft_game_type"`
	TFTSetCoreName  string              `json:"tft_set_core_name"`
	TFTSetNumber    int                 `json:"tft_set_number"`
	Participants    []TFTParticipantDTO `json:"participants"`
}

type TFTParticipantDTO struct {
	Puuid                string          `json:"puuid"`
	Placement            int             `json:"placement"`
	Level                int             `json:"level"`
	GoldLeft             int             `json:"gold_left"`
	LastRound            int             `json:"last_round"`
	PlayersEliminated    int             `json:"players_eliminated"`
	TimeEliminated       float64         `json:"time_eliminated"`
	TotalDamageToPlayers int             `json:"total_damage_to_players"`
	Augments             []string        `json:"augments"`
	Companion            TFTCompanionDTO `json:"companion"`
	Traits               []TFTTraitDTO   `json:"traits"`
	Units                []TFTUnitDTO    `json:"units"`
}

type TFTCompanionDTO struct {
	ContentID string `json:"content_ID"`
	ItemID    int    `json:"item_ID"`
	SkinID    int    `json:"skin_ID"`
	Species   string `json:"species"`
}

type TFTTraitDTO struct {
	Name        string `json:"name"`
	NumUnits    int    `json:"num_units"`
	Style       int    `json:"style"`
	TierCurrent int    `json:"tier_current"`
	TierTotal   int    `json:"tier_total"`
}

type TFTUnitDTO struct {
	CharacterID string   `json:"character_id"`
	ItemNames   []string `json:"itemNames"`
	Name        string   `json:"name"`
	Rarity      int      `json:"rarity"`
	Tier        int      `json:"tier"`
}
