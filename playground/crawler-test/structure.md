# crawler-test 代码结构与扩展方式

## 1. 当前目录结构（已对齐现状）

```text
crawler-test/
├── config.yaml
├── cmd/
│   └── crawler/
│       └── main.go
└── internal/
    ├── crawler/
    │   ├── types.go
    │   ├── router.go
    │   ├── worker.go
    │   └── handler_account.go
    └── riot/
        ├── client.go
        ├── errors.go
        ├── rate_limiter.go
        └── api_account.go
```

## 2. 分层职责说明

- `internal/riot/*`：网络访问层（如何请求 Riot API）。
- `internal/crawler/*`：任务调度层（如何消费任务、分发处理）。
- `api_account.go`：面向 Riot API 的账号接口封装。
- `handler_account.go`：面向任务系统的账号任务处理器。

说明：`api_account` 和 `handler_account` 不是重复代码，而是不同层的职责拆分。

## 3. 新增一个请求功能（以 Match-V5 为例）

目标：新增一个任务类型 `MATCH_BY_ID`，可以从任务队列拉取 `matchID` 并调用 Riot Match API。

### Step 1: 在 riot 层新增 API 文件

新增 `internal/riot/api_match.go`：

```go
package riot

import (
	"context"
	"fmt"
)

type MatchDTO struct {
	Metadata struct {
		MatchID string `json:"matchId"`
	} `json:"metadata"`
}

func (c *Client) GetMatchByID(ctx context.Context, matchID string) (*MatchDTO, error) {
	url := fmt.Sprintf("https://asia.api.riotgames.com/lol/match/v5/matches/%s", matchID)
	var match MatchDTO
	if err := c.doRequest(ctx, "GET", url, &match); err != nil {
		return nil, err
	}
	return &match, nil
}
```

### Step 2: 在任务模型中声明新类型与 payload

修改 `internal/crawler/types.go`：

```go
const (
	TaskTypeAccountByRiotID TaskType = "ACCOUNT_BY_RIOT_ID"
	TaskTypeMatchByID       TaskType = "MATCH_BY_ID"
)

type MatchByIDPayload struct {
	MatchID string
}
```

### Step 3: 在 crawler 层新增处理器

新增 `internal/crawler/handler_match.go`：

```go
package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)

func HandleMatchByID(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error) {
	p, ok := payload.(MatchByIDPayload)
	if !ok {
		return nil, fmt.Errorf("invalid payload for %s", TaskTypeMatchByID)
	}

	return client.GetMatchByID(ctx, p.MatchID)
}
```

### Step 4: 在 router 注册处理器

修改 `internal/crawler/router.go` 的 `NewDefaultRouter()`：

```go
func NewDefaultRouter() *Router {
	r := NewRouter()
	r.Register(TaskTypeAccountByRiotID, HandleAccountByRiotID)
	r.Register(TaskTypeMatchByID, HandleMatchByID)
	r.Register(TaskTypeChallengeLeaguesByQueue, HandleChallengeLeaguesByQueue)
	r.Register(TaskTypeVersion, HandleVersions)
	r.Register(TaskTypeMatchByPUUID, HandleMatchByPuuid)
	return r
}
```

### Step 5: 在生产者投递新任务

修改 `cmd/crawler/main.go`（或你自己的任务生产逻辑）：

```go
taskQueue <- crawler.Task{
	Type: crawler.TaskTypeMatchByID,
	Payload: crawler.MatchByIDPayload{
		MatchID: "KR_1234567890",
	},
}
```

## 4. 扩展约定（建议遵循）

- 一个新请求类型，固定新增三处：`api_xxx.go`、`handler_xxx.go`、`router register`。
- `worker.go` 保持通用，不写具体业务分支。
- payload 尽量使用结构体，不直接用 `map[string]string`。
- 新增后执行：`go test ./...`。

## 5. 新增一个Processor

目标：在获取到versions信息后，新增一个Processor来处理这个结果（例如打印版本列表）。

### Step 1: 在processer层新增processor文件
新增 `internal/crawler/processor_versions.go`：

### Step 2: 在storage层新增repo文件
新增 `internal/storage/repo_versions.go`：

### Step 3: 在result_processor里面新增一个case来调用这个processor
修改 `internal/crawler/worker.go` 的结果处理部分：



## 关于PostgreSQL Docker镜像
docker-compose.yaml 已经配置好了 PostgreSQL 镜像，默认用户名和密码都是 `gogg`，数据库名也是 `gogg`。你可以通过以下命令来管理这个数据库：
1. 启动数据库
```bash
cd crawler-test
docker compose up -d
```
2. 查看状态
```bash
docker compose ps
```
3. 停止数据库
```bash
docker compose down
```
4. 删掉数据卷（会丢数据）
```bash
docker compose down -v
```

### 5. 数据库dsn配置
dsn格式示例：`postgres://gogg:goggpass@localhost:5432/gogg?sslmode=disable`，其中：
- `gogg`：用户名
- `goggpass`：密码
- `localhost`：数据库地址
- `5432`：数据库端口
- `gogg`：数据库名
- `sslmode=disable`：禁用SSL连接（本地开发环境常用）
