package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)

// TaskHandler 统一任务处理器签名。
type TaskHandler func(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error)

// Router 管理任务类型到处理器的映射。
type Router struct {
	handlers map[TaskType]TaskHandler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[TaskType]TaskHandler)}
}

func (r *Router) Register(taskType TaskType, handler TaskHandler) {
	r.handlers[taskType] = handler
}

func (r *Router) Handle(ctx context.Context, task Task, client *riot.Client) (interface{}, error) {
	handler, ok := r.handlers[task.Type]
	if !ok {
		return nil, fmt.Errorf("no handler registered for task type: %s", task.Type)
	}

	return handler(ctx, task.Payload, client)
}

// NewDefaultRouter 注册内置请求处理器。
func NewDefaultRouter() *Router {
	r := NewRouter()
	r.Register(TaskTypeAccountByRiotID, HandleAccountByRiotID)
	r.Register(TaskTypeChallengeLeaguesByQueue, HandleChallengeLeaguesByQueue)
	r.Register(TaskTypeVersion, HandleVersions)
	r.Register(TaskTypeMatchByPUUID, HandleMatchByPuuid)
	r.Register(TaskTypeMatchDetailByMatchID, HandleMatchDetailByMatchID)
	return r
}
