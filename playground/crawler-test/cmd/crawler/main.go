package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"crawler-test/internal/crawler" // 替换为你的实际模块名
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
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 加载配置
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("配置加载成功，Worker=%d，队列大小=%d", cfg.Crawler.WorkerCount, cfg.Crawler.QueueSize)

	// 2. 初始化核心组件 (注入配置中的参数)
	client := riot.NewClient(cfg.Riot.APIKey)
	limiter := riot.NewRateLimiter()
	router := crawler.NewDefaultRouter()

	var resultProcessor crawler.ResultProcessor
	if cfg.Database.Enabled {
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

		resultProcessor = process.NewResultProcessor(store)
		log.Println("数据库持久化已启用")
	} else {
		log.Println("数据库持久化未启用，跳过写库")
	}

	// 3. 创建任务队列 (使用配置中的 QueueSize)
	taskQueue := make(chan crawler.Task, cfg.Crawler.QueueSize)

	// 5. 启动 Worker 池 (使用配置中的 WorkerCount)
	log.Printf("正在启动 %d 个 Worker...", cfg.Crawler.WorkerCount)
	for i := 1; i <= cfg.Crawler.WorkerCount; i++ {
		wg.Add(1)
		go crawler.StartWorker(ctx, i, taskQueue, client, limiter, router, resultProcessor, &wg)
	}

	// 6. 模拟生产任务投递
	var taskIDCounter uint64
	go func() {
		log.Println("任务投递器启动")
		// players := []crawler.AccountByRiotIDPayload{
		// 	{GameName: "RuitaoZhou", TagLine: "123"},
		// 	{GameName: "RuitaoZhou", TagLine: "123"},
		// 	{GameName: "RuitaoZhou", TagLine: "123"},
		// 	{GameName: "RuitaoZhou", TagLine: "123"},
		// 	{GameName: "RuitaoZhou", TagLine: "123"},
		// 	{GameName: "RuitaoZhou", TagLine: "123"},
		// }
		leagues := []crawler.ChallengeLeaguesByQueuePayload{
			{Queue: "RANKED_SOLO_5x5"},
		}
		// taskID := atomic.AddUint64(&taskIDCounter, 1)
		// for _, p := range players {
		// 	task := crawler.Task{
		// 		ID:      fmt.Sprintf("task-%d", taskID),
		// 		Type:    crawler.TaskTypeAccountByRiotID,
		// 		Payload: p,
		// 		Retries: 3,
		// 	}
		// 	taskQueue <- task
		// 	log.Printf("任务已入队: id=%s type=%s 玩家=%s#%s", task.ID, task.Type, p.GameName, p.TagLine)
		// }
		for _, l := range leagues {
			taskID := atomic.AddUint64(&taskIDCounter, 1)
			task := crawler.Task{
				ID:      fmt.Sprintf("task-%d", taskID),
				Type:    crawler.TaskTypeChallengeLeaguesByQueue,
				Payload: l,
				Retries: 3,
			}
			taskQueue <- task
			log.Printf("任务已入队: id=%s type=%s 队列=%s", task.ID, task.Type, l.Queue)
		}
		log.Printf("任务投递完成，总数=%d", len(leagues))
	}()

	// 7. 阻塞主线程，等待退出信号
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	receivedSig := <-sig
	log.Printf("收到退出信号: %s，开始优雅关闭", receivedSig.String())
	cancel()

	wg.Wait()
	log.Println("爬虫已安全退出")
}
