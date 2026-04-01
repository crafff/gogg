package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"

	"crawler-test/internal/crawler"
	"crawler-test/internal/process"
	"crawler-test/internal/riot"
	"crawler-test/internal/storage"
)

// Config 映射 config.yaml 的数据结构
type Config struct {
	Riot struct {
		APIKey  string `yaml:"api_key"`
		BaseURL string `yaml:"base_url"`
	} `yaml:"riot"`
	Database struct {
		Enabled         bool   `yaml:"enabled"`
		DSN             string `yaml:"dsn"`
		MaxOpenConns    int32  `yaml:"max_open_conns"`
		MaxIdleConns    int32  `yaml:"max_idle_conns"`
		ConnMaxLifetime int    `yaml:"conn_max_lifetime_seconds"`
	} `yaml:"database"`
	Crawler struct {
		WorkerCount int `yaml:"worker_count"`
		QueueSize   int `yaml:"queue_size"`
	} `yaml:"crawler"`
}

// loadConfig 读取并解析 YAML 文件
func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Crawler.WorkerCount <= 0 {
		cfg.Crawler.WorkerCount = 1
	}
	if cfg.Crawler.QueueSize <= 0 {
		cfg.Crawler.QueueSize = 100
	}

	// 简单的防御性校验
	if cfg.Riot.APIKey == "" || cfg.Riot.APIKey == "RGAPI-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" {
		return nil, fmt.Errorf("invalid riot api key in config")
	}
	if cfg.Database.Enabled && cfg.Database.DSN == "" {
		return nil, fmt.Errorf("database dsn is required when database.enabled=true")
	}

	return &cfg, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 加载配置
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("配置加载成功")
	if !cfg.Database.Enabled {
		log.Fatal("自动流水线依赖数据库，请设置 database.enabled=true")
	}

	// 2. 初始化核心组件
	client := riot.NewClient(cfg.Riot.APIKey)
	limiter := riot.NewRateLimiter()

	store, err := storage.NewStore(ctx, storage.Config{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Database.ConnMaxLifetime) * time.Second,
	})
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer store.Close()

	if err := store.InitSchema(ctx); err != nil {
		log.Fatalf("初始化数据库表结构失败: %v", err)
	}

	resultProcessor := process.NewResultProcessor(store)

	if err := runAutoPipeline(ctx, client, limiter, resultProcessor, store); err != nil {
		log.Fatalf("自动流水线执行失败: %v", err)
	}

	log.Println("自动流水线执行完成")
}

func runAutoPipeline(ctx context.Context, client *riot.Client, limiter *riot.RateLimiter, resultProcessor *process.ResultProcessor, store *storage.Store) error {
	var taskIDCounter uint64
	nextTaskID := func() string {
		return fmt.Sprintf("task-%d", atomic.AddUint64(&taskIDCounter, 1))
	}

	const matchPageSize = 100
	var err error

	log.Println("步骤1/4: 获取版本信息并入库")
	var versions *riot.VersionResponse
	for {
		if err := limiter.Wait(ctx); err != nil {
			return fmt.Errorf("wait limiter before versions: %w", err)
		}
		versions, err = client.GetVersions(ctx)
		retry, retryErr := waitForRateLimitRetry(ctx, err)
		if retryErr != nil {
			return retryErr
		}
		if retry {
			continue
		}
		if err != nil {
			return fmt.Errorf("fetch versions: %w", err)
		}
		break
	}
	if err := resultProcessor.Process(ctx, crawler.Task{
		ID:      nextTaskID(),
		Type:    crawler.TaskTypeVersion,
		Payload: crawler.VersionPayload{},
		Retries: 1,
	}, versions); err != nil {
		return fmt.Errorf("persist versions: %w", err)
	}

	latestVersion, err := store.GetLatestActiveVersion(ctx)
	if err != nil {
		return fmt.Errorf("load latest active version: %w", err)
	}
	log.Printf("最新版本: version=%s start_at=%s", latestVersion.Version, latestVersion.StartAt.UTC().Format(time.RFC3339))

	log.Println("步骤2/4: 获取 challenger puuid 并入库")
	queue := "RANKED_SOLO_5x5"
	var league *riot.LeagueListDTO
	for {
		if err := limiter.Wait(ctx); err != nil {
			return fmt.Errorf("wait limiter before challenger: %w", err)
		}
		league, err = client.GetChallengerLeaguesByQueue(ctx, queue)
		retry, retryErr := waitForRateLimitRetry(ctx, err)
		if retryErr != nil {
			return retryErr
		}
		if retry {
			continue
		}
		if err != nil {
			return fmt.Errorf("fetch challenger by queue %s: %w", queue, err)
		}
		break
	}
	if err := resultProcessor.Process(ctx, crawler.Task{
		ID:   nextTaskID(),
		Type: crawler.TaskTypeChallengeLeaguesByQueue,
		Payload: crawler.ChallengeLeaguesByQueuePayload{
			Queue: queue,
		},
		Retries: 2,
	}, league); err != nil {
		return fmt.Errorf("persist challenger players: %w", err)
	}

	log.Println("步骤3/4: 根据 puuid 分页增量抓取 match id 并更新 sync_state")
	players, err := store.ListPlayers(ctx)
	if err != nil {
		return fmt.Errorf("list players: %w", err)
	}
	log.Printf("待同步玩家数: %d", len(players))

	for i, player := range players {
		if err := ctx.Err(); err != nil {
			return err
		}

		syncState, err := store.GetPlayerMatchSyncState(ctx, player.ID, latestVersion.ID)
		if err != nil {
			log.Printf("读取玩家同步状态失败，跳过 player_id=%d puuid=%s err=%v", player.ID, player.Puuid, err)
			continue
		}

		windowStart := latestVersion.StartAt.UTC()
		if syncState != nil && syncState.LastCheckedAt != nil && syncState.LastCheckedAt.After(windowStart) {
			windowStart = syncState.LastCheckedAt.UTC()
		}

		startTime := windowStart.Unix()
		totalFetched := 0
		for offset := 0; ; offset += matchPageSize {
			var matches *riot.Matchs
			for {
				if err := limiter.Wait(ctx); err != nil {
					return fmt.Errorf("wait limiter before match ids: %w", err)
				}

				matches, err = client.GetMatchsByPuuid(ctx, player.Puuid, startTime, -1, "", offset, matchPageSize)
				retry, retryErr := waitForRateLimitRetry(ctx, err)
				if retryErr != nil {
					return retryErr
				}
				if retry {
					continue
				}
				if err != nil {
					log.Printf("抓取 match ids 失败，跳过该玩家: player_id=%d puuid=%s offset=%d err=%v", player.ID, player.Puuid, offset, err)
					totalFetched = -1
				}
				break
			}
			if totalFetched < 0 {
				break
			}

			if len(*matches) == 0 {
				break
			}

			if err := resultProcessor.Process(ctx, crawler.Task{
				ID:   nextTaskID(),
				Type: crawler.TaskTypeMatchByPUUID,
				Payload: crawler.MatchByPUUIDPayload{
					Puuid:     player.Puuid,
					StartTime: startTime,
					Start:     offset,
					Count:     matchPageSize,
				},
				Retries: 2,
			}, matches); err != nil {
				log.Printf("入库 match ids 失败，跳过该玩家: player_id=%d puuid=%s offset=%d err=%v", player.ID, player.Puuid, offset, err)
				totalFetched = -1
				break
			}

			totalFetched += len(*matches)
			if len(*matches) < matchPageSize {
				break
			}
		}

		if totalFetched >= 0 {
			now := time.Now().UTC()
			if err := store.UpsertPlayerMatchSyncState(ctx, storage.PlayerMatchSyncStateSeed{
				PlayerID:      player.ID,
				VersionID:     latestVersion.ID,
				LastCheckedAt: &now,
			}); err != nil {
				log.Printf("更新玩家同步状态失败: player_id=%d puuid=%s err=%v", player.ID, player.Puuid, err)
			} else {
				log.Printf("玩家同步完成 %d/%d: player_id=%d puuid=%s window_start=%s fetched=%d", i+1, len(players), player.ID, player.Puuid, windowStart.Format(time.RFC3339), totalFetched)
			}
		}
	}

	log.Println("步骤4/4: 获取 processed_at 为空的比赛详情")
	pendingMatchIDs, err := store.ListPendingMatchIDs(ctx, 0)
	if err != nil {
		return fmt.Errorf("list pending match ids: %w", err)
	}
	log.Printf("待补全比赛详情数量: %d", len(pendingMatchIDs))

	for i, matchID := range pendingMatchIDs {
		if err := ctx.Err(); err != nil {
			return err
		}

		var detail *riot.MatchDetailDTO
		for {
			if err := limiter.Wait(ctx); err != nil {
				return fmt.Errorf("wait limiter before match detail: %w", err)
			}
			detail, err = client.GetMatchDetailByMatchID(ctx, matchID)
			retry, retryErr := waitForRateLimitRetry(ctx, err)
			if retryErr != nil {
				return retryErr
			}
			if retry {
				continue
			}
			if err != nil {
				log.Printf("抓取比赛详情失败，跳过 match_id=%s err=%v", matchID, err)
			}
			break
		}
		if err != nil {
			continue
		}

		if err := resultProcessor.Process(ctx, crawler.Task{
			ID:   nextTaskID(),
			Type: crawler.TaskTypeMatchDetailByMatchID,
			Payload: crawler.MatchDetailByMatchIDPayload{
				MatchID: matchID,
			},
			Retries: 2,
		}, detail); err != nil {
			log.Printf("入库比赛详情失败，跳过 match_id=%s err=%v", matchID, err)
			continue
		}

		log.Printf("比赛详情同步完成 %d/%d: match_id=%s", i+1, len(pendingMatchIDs), matchID)
	}

	return nil
}

func waitForRateLimitRetry(ctx context.Context, err error) (bool, error) {
	if err == nil {
		return false, nil
	}

	var rateLimitErr *riot.RateLimitError
	if !errors.As(err, &rateLimitErr) {
		return false, nil
	}

	log.Printf("触发限流，等待后重试: retry_after=%s", rateLimitErr.RetryAfter)
	timer := time.NewTimer(rateLimitErr.RetryAfter)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-timer.C:
		return true, nil
	}
}
