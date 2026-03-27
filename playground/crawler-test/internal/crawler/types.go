package crawler

import "context"

// TaskType 标记一个抓取任务的请求种类。
type TaskType string

const (
	TaskTypeAccountByRiotID         TaskType = "ACCOUNT_BY_RIOT_ID"
	TaskTypeChallengeLeaguesByQueue TaskType = "CHALLENGE_LEAGUES_BY_QUEUE"
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

type ChallengeLeaguesByQueuePayload struct {
	Queue string
}

// ResultProcessor 在任务成功拉取后对结果做后处理（例如入库）。
type ResultProcessor interface {
	Process(ctx context.Context, task Task, result interface{}) error
}
