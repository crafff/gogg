package crawler

import "context"

// TaskType 标记一个抓取任务的请求种类。
type TaskType string

const (
	TaskTypeAccountByRiotID         TaskType = "ACCOUNT_BY_RIOT_ID"
	TaskTypeChallengeLeaguesByQueue TaskType = "CHALLENGE_LEAGUES_BY_QUEUE"
	TaskTypeVersion                 TaskType = "VERSION"
	TaskTypeMatchByPUUID            TaskType = "MATCH_BY_PUUID"
	TaskTypeMatchDetailByMatchID    TaskType = "MATCH_DETAIL_BY_ID"
)

// Task 是 Worker 消费的统一任务模型。
type Task struct {
	ID      string
	Type    TaskType
	Payload interface{}
	// Retries 表示失败后的最大重试次数（不含首次请求）。
	Retries int
}

// AccountByRiotIDPayload 是账号查询任务的参数。
type AccountByRiotIDPayload struct {
	GameName string
	TagLine  string
}

// ChallengeLeaguesByQueuePayload 是挑战者榜查询任务的参数。
type ChallengeLeaguesByQueuePayload struct {
	Queue string
}

// VersionPayload 是版本查询任务的参数，目前没有字段，可以根据需要扩展。
type VersionPayload struct {
}

// MatchByPUUIDPayload 是根据玩家 PUUID 查询比赛列表的任务参数。
type MatchByPUUIDPayload struct {
	Puuid		string
	MatchType	string		// 可选参数，表示比赛类型（ranked, normal, tourney, tutorial), 不选传入空字符串表示全部类型
	StartTime	int64		// Unix 时间戳，单位秒,可选, 默认传入-1
	EndTime		int64		// Unix 时间戳，单位秒,可选，不选传入-1
	Start		int			// 分页用，表示从第几条开始取,可选,不选输入-1
	Count 		int 		// 分页用，表示取多少条,可选,不选传入-1
}

type MatchDetailByMatchIDPayload struct {
	MatchID string
}

// ResultProcessor 在任务成功拉取后对结果做后处理（例如入库）。
type ResultProcessor interface {
	Process(ctx context.Context, task Task, result interface{}) error
}
