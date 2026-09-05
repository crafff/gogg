package riotapi

// MatchDetailDTO mirrors the Riot Match V5 response.
type MatchDetailDTO struct {
	Metadata MetadataDTO `json:"metadata"`
	Info     InfoDTO     `json:"info"`
}

type MetadataDTO struct {
	DataVersion  string   `json:"dataVersion"`
	MatchID      string   `json:"matchId"`
	Participants []string `json:"participants"`
}

type InfoDTO struct {
	EndOfGameResult    string           `json:"endOfGameResult"`
	GameCreation       int64            `json:"gameCreation"`
	GameDuration       int64            `json:"gameDuration"`
	GameEndTimestamp   int64            `json:"gameEndTimestamp"`
	GameID             int64            `json:"gameId"`
	GameMode           string           `json:"gameMode"`
	GameName           string           `json:"gameName"`
	GameStartTimestamp int64            `json:"gameStartTimestamp"`
	GameType           string           `json:"gameType"`
	GameVersion        string           `json:"gameVersion"`
	MapID              int              `json:"mapId"`
	PlatformID         string           `json:"platformId"`
	QueueID            int              `json:"queueId"`
	TournamentCode     string           `json:"tournamentCode"`
	Participants       []ParticipantDTO `json:"participants"`
	Teams              []TeamDTO        `json:"teams"`
}

type ParticipantDTO struct {
	ParticipantID  int    `json:"participantId"`
	Puuid          string `json:"puuid"`
	SummonerID     string `json:"summonerId"`
	SummonerLevel  int    `json:"summonerLevel"`
	RiotIDGameName string `json:"riotIdGameName"`
	RiotIDTagline  string `json:"riotIdTagline"`
	ProfileIcon    int    `json:"profileIcon"`

	TeamID             int    `json:"teamId"`
	TeamPosition       string `json:"teamPosition"`
	IndividualPosition string `json:"individualPosition"`
	Lane               string `json:"lane"`
	Role               string `json:"role"`
	Placement          int    `json:"placement"`
	Win                bool   `json:"win"`

	ChampionID        int    `json:"championId"`
	ChampionName      string `json:"championName"`
	ChampLevel        int    `json:"champLevel"`
	ChampExperience   int    `json:"champExperience"`
	ChampionTransform int    `json:"championTransform"`

	PlayerAugment1 int `json:"playerAugment1"`
	PlayerAugment2 int `json:"playerAugment2"`
	PlayerAugment3 int `json:"playerAugment3"`
	PlayerAugment4 int `json:"playerAugment4"`
	PlayerAugment5 int `json:"playerAugment5"`
	PlayerAugment6 int `json:"playerAugment6"`

	Summoner1ID    int `json:"summoner1Id"`
	Summoner2ID    int `json:"summoner2Id"`
	Summoner1Casts int `json:"summoner1Casts"`
	Summoner2Casts int `json:"summoner2Casts"`

	Spell1Casts int `json:"spell1Casts"`
	Spell2Casts int `json:"spell2Casts"`
	Spell3Casts int `json:"spell3Casts"`
	Spell4Casts int `json:"spell4Casts"`

	Kills                  int  `json:"kills"`
	Deaths                 int  `json:"deaths"`
	Assists                int  `json:"assists"`
	DoubleKills            int  `json:"doubleKills"`
	TripleKills            int  `json:"tripleKills"`
	QuadraKills            int  `json:"quadraKills"`
	PentaKills             int  `json:"pentaKills"`
	UnrealKills            int  `json:"unrealKills"`
	KillingSprees          int  `json:"killingSprees"`
	LargestKillingSpree    int  `json:"largestKillingSpree"`
	LargestMultiKill       int  `json:"largestMultiKill"`
	FirstBloodKill         bool `json:"firstBloodKill"`
	FirstBloodAssist       bool `json:"firstBloodAssist"`
	LongestTimeSpentLiving int  `json:"longestTimeSpentLiving"`
	TotalTimeSpentDead     int  `json:"totalTimeSpentDead"`

	TotalDamageDealt               int `json:"totalDamageDealt"`
	TotalDamageDealtToChampions    int `json:"totalDamageDealtToChampions"`
	PhysicalDamageDealt            int `json:"physicalDamageDealt"`
	PhysicalDamageDealtToChampions int `json:"physicalDamageDealtToChampions"`
	MagicDamageDealt               int `json:"magicDamageDealt"`
	MagicDamageDealtToChampions    int `json:"magicDamageDealtToChampions"`
	TrueDamageDealt                int `json:"trueDamageDealt"`
	TrueDamageDealtToChampions     int `json:"trueDamageDealtToChampions"`
	LargestCriticalStrike          int `json:"largestCriticalStrike"`

	TotalDamageTaken    int `json:"totalDamageTaken"`
	PhysicalDamageTaken int `json:"physicalDamageTaken"`
	MagicDamageTaken    int `json:"magicDamageTaken"`
	TrueDamageTaken     int `json:"trueDamageTaken"`
	DamageSelfMitigated int `json:"damageSelfMitigated"`

	TotalHeal                      int `json:"totalHeal"`
	TotalHealsOnTeammates          int `json:"totalHealsOnTeammates"`
	TotalUnitsHealed               int `json:"totalUnitsHealed"`
	TotalDamageShieldedOnTeammates int `json:"totalDamageShieldedOnTeammates"`
	TimeCCingOthers                int `json:"timeCCingOthers"`
	TotalTimeCCDealt               int `json:"totalTimeCCDealt"`

	GoldEarned int `json:"goldEarned"`
	GoldSpent  int `json:"goldSpent"`

	Item0                int `json:"item0"`
	Item1                int `json:"item1"`
	Item2                int `json:"item2"`
	Item3                int `json:"item3"`
	Item4                int `json:"item4"`
	Item5                int `json:"item5"`
	Item6                int `json:"item6"`
	ItemsPurchased       int `json:"itemsPurchased"`
	ConsumablesPurchased int `json:"consumablesPurchased"`
	RoleBoundItem        int `json:"roleBoundItem"`

	TotalMinionsKilled            int `json:"totalMinionsKilled"`
	NeutralMinionsKilled          int `json:"neutralMinionsKilled"`
	TotalAllyJungleMinionsKilled  int `json:"totalAllyJungleMinionsKilled"`
	TotalEnemyJungleMinionsKilled int `json:"totalEnemyJungleMinionsKilled"`
	BaronKills                    int `json:"baronKills"`
	DragonKills                   int `json:"dragonKills"`
	ObjectivesStolen              int `json:"objectivesStolen"`
	ObjectivesStolenAssists       int `json:"objectivesStolenAssists"`

	VisionScore             int `json:"visionScore"`
	VisionWardsBoughtInGame int `json:"visionWardsBoughtInGame"`
	SightWardsBoughtInGame  int `json:"sightWardsBoughtInGame"`
	WardsPlaced             int `json:"wardsPlaced"`
	DetectorWardsPlaced     int `json:"detectorWardsPlaced"`
	WardsKilled             int `json:"wardsKilled"`

	TurretKills        int  `json:"turretKills"`
	TurretTakedowns    int  `json:"turretTakedowns"`
	TurretsLost        int  `json:"turretsLost"`
	FirstTowerKill     bool `json:"firstTowerKill"`
	FirstTowerAssist   bool `json:"firstTowerAssist"`
	InhibitorKills     int  `json:"inhibitorKills"`
	InhibitorTakedowns int  `json:"inhibitorTakedowns"`
	InhibitorsLost     int  `json:"inhibitorsLost"`
	NexusKills         int  `json:"nexusKills"`
	NexusLost          int  `json:"nexusLost"`
	NexusTakedowns     int  `json:"nexusTakedowns"`

	DamageDealtToObjectives   int `json:"damageDealtToObjectives"`
	DamageDealtToBuildings    int `json:"damageDealtToBuildings"`
	DamageDealtToTurrets      int `json:"damageDealtToTurrets"`
	DamageDealtToEpicMonsters int `json:"damageDealtToEpicMonsters"`

	AllInPings         int `json:"allInPings"`
	BasicPings         int `json:"basicPings"`
	AssistMePings      int `json:"assistMePings"`
	CommandPings       int `json:"commandPings"`
	DangerPings        int `json:"dangerPings"`
	EnemyMissingPings  int `json:"enemyMissingPings"`
	GetBackPings       int `json:"getBackPings"`
	HoldPings          int `json:"holdPings"`
	OnMyWayPings       int `json:"onMyWayPings"`
	NeedVisionPings    int `json:"needVisionPings"`
	PushPings          int `json:"pushPings"`
	RetreatPings       int `json:"retreatPings"`
	EnemyVisionPings   int `json:"enemyVisionPings"`
	VisionClearedPings int `json:"visionClearedPings"`

	GameEndedInEarlySurrender bool `json:"gameEndedInEarlySurrender"`
	GameEndedInSurrender      bool `json:"gameEndedInSurrender"`
	TeamEarlySurrendered      bool `json:"teamEarlySurrendered"`
	TimePlayed                int  `json:"timePlayed"`

	Perks PerksDTO `json:"perks"`
}

type PerksDTO struct {
	StatPerks struct {
		Defense int `json:"defense"`
		Flex    int `json:"flex"`
		Offense int `json:"offense"`
	} `json:"statPerks"`
	Styles []PerkStyleDTO `json:"styles"`
}

type PerkStyleDTO struct {
	Description string             `json:"description"`
	Style       int                `json:"style"`
	Selections  []PerkSelectionDTO `json:"selections"`
}

type PerkSelectionDTO struct {
	Perk int `json:"perk"`
	Var1 int `json:"var1"`
	Var2 int `json:"var2"`
	Var3 int `json:"var3"`
}

type TeamDTO struct {
	TeamID     int      `json:"teamId"`
	Win        bool     `json:"win"`
	Bans       []BanDTO `json:"bans"`
	Objectives struct {
		Baron      ObjectiveDTO `json:"baron"`
		Dragon     ObjectiveDTO `json:"dragon"`
		Inhibitor  ObjectiveDTO `json:"inhibitor"`
		RiftHerald ObjectiveDTO `json:"riftHerald"`
		Tower      ObjectiveDTO `json:"tower"`
		Champion   ObjectiveDTO `json:"champion"`
		Atakhan    ObjectiveDTO `json:"atakhan"`
		Horde      ObjectiveDTO `json:"horde"`
	} `json:"objectives"`
	Feats map[string]struct {
		FeatState int `json:"featState"`
	} `json:"feats"`
}

type ObjectiveDTO struct {
	First bool `json:"first"`
	Kills int  `json:"kills"`
}

type BanDTO struct {
	ChampionID int `json:"championId"`
	PickTurn   int `json:"pickTurn"`
}
