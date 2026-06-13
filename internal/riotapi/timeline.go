package riotapi

import (
	"context"
	"fmt"
	"net/url"
)

// TimelineDTO is the top-level response from the match timeline endpoint.
// championStats and damageStats inside participantFrames are intentionally
// omitted from the struct — the JSON decoder skips unknown fields, saving
// significant memory for large timeline responses.
type TimelineDTO struct {
	Metadata MetadataDTO     `json:"metadata"`
	Info     TimelineInfoDTO `json:"info"`
}

type TimelineInfoDTO struct {
	FrameInterval int                `json:"frameInterval"` // ms between frames (typically 60000)
	Frames        []TimelineFrameDTO `json:"frames"`
}

type TimelineFrameDTO struct {
	Timestamp         int                            `json:"timestamp"`
	Events            []TimelineEventDTO             `json:"events"`
	ParticipantFrames map[string]ParticipantFrameDTO `json:"participantFrames"` // key: "1".."10"
}

// ParticipantFrameDTO contains per-player stats at each frame boundary.
// championStats is omitted to reduce parse cost.
type ParticipantFrameDTO struct {
	ParticipantId            int `json:"participantId"`
	CurrentGold              int `json:"currentGold"`
	TotalGold                int `json:"totalGold"`
	MinionsKilled            int `json:"minionsKilled"`
	JungleMinionsKilled      int `json:"jungleMinionsKilled"`
	Level                    int `json:"level"`
	XP                       int `json:"xp"`
	TimeEnemySpentControlled int `json:"timeEnemySpentControlled"`
	Position                 struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"position"`
	DamageStats ParticipantDamageStats `json:"damageStats"`
}

// ParticipantDamageStats holds cumulative damage figures at a frame boundary.
type ParticipantDamageStats struct {
	TotalDamageDone               int `json:"totalDamageDone"`
	TotalDamageDoneToChampions    int `json:"totalDamageDoneToChampions"`
	MagicDamageDoneToChampions    int `json:"magicDamageDoneToChampions"`
	PhysicalDamageDoneToChampions int `json:"physicalDamageDoneToChampions"`
	TrueDamageDoneToChampions     int `json:"trueDamageDoneToChampions"`
	TotalDamageTaken              int `json:"totalDamageTaken"`
}

// TimelineEventDTO captures only the event fields we use.
// victimDamageDealt/Received arrays are intentionally omitted.
type TimelineEventDTO struct {
	Type      string `json:"type"`
	Timestamp int    `json:"timestamp"`

	// item events
	ParticipantId int `json:"participantId"`
	ItemId        int `json:"itemId"`

	// ITEM_UNDO
	BeforeId int `json:"beforeId"`

	// SKILL_LEVEL_UP
	SkillSlot   int    `json:"skillSlot"`
	LevelUpType string `json:"levelUpType"`

	// LEVEL_UP
	Level int `json:"level"`

	// CHAMPION_KILL
	KillerId                int   `json:"killerId"`
	VictimId                int   `json:"victimId"`
	AssistingParticipantIds []int `json:"assistingParticipantIds"`

	// WARD_PLACED
	CreatorId int    `json:"creatorId"`
	WardType  string `json:"wardType"`

	// CHAMPION_SPECIAL_KILL
	KillType        string `json:"killType"`
	MultiKillLength int    `json:"multiKillLength"`

	// BUILDING_KILL / TURRET_PLATE_DESTROYED
	LaneType     string `json:"laneType"`
	BuildingType string `json:"buildingType"`
	TeamId       int    `json:"teamId"`
}

// GetMatchTimeline returns the timeline for the given match ID.
func (c *Client) GetMatchTimeline(ctx context.Context, matchID string) (*TimelineDTO, error) {
	u := fmt.Sprintf("%s/lol/match/v5/matches/%s/timeline", c.regionalURL, url.PathEscape(matchID))
	var dto TimelineDTO
	return &dto, c.doRequest(ctx, u, &dto)
}