package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"crawler-test/internal/riot"
)

// tasks <-chan Task 表示在这个函数内部只能从tasks读取数据，不能往里面发送数据
func StartWorker(ctx context.Context, id int, tasks <-chan Task, client *riot.Client, limiter *riot.RateLimiter, router *Router, resultProcessor ResultProcessor, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Worker-%d 已启动", id)

	for {
		select {
		case <-ctx.Done(): // 主函数调用cancel()退出
			log.Printf("Worker-%d 收到停止信号: %v", id, ctx.Err())
			return
		case task, ok := <-tasks:
			if !ok {
				log.Printf("Worker-%d 任务队列已关闭", id)
				return
			}

			maxRetries := task.Retries
			if maxRetries < 0 {
				maxRetries = 0
			}

			for attempt := 1; attempt <= maxRetries+1; attempt++ {
				start := time.Now()
				log.Printf("Worker-%d 开始处理任务: id=%s type=%s 第%d次尝试", id, task.ID, task.Type, attempt)

				// 1. 严格过限流器
				if err := limiter.Wait(ctx); err != nil {
					log.Printf("Worker-%d 限流等待失败: id=%s err=%v", id, task.ID, err)
					break
				}

				// 2. 按任务类型分发到对应处理器
				result, err := router.Handle(ctx, task, client)
				if err != nil {
					var rateLimitErr *riot.RateLimitError
					if errors.As(err, &rateLimitErr) {
						log.Printf("Worker-%d 任务被限流: id=%s retry_after=%v", id, task.ID, rateLimitErr.RetryAfter)
						timer := time.NewTimer(rateLimitErr.RetryAfter)
						select {
						case <-ctx.Done():
							timer.Stop()
							log.Printf("Worker-%d 任务重试被取消: id=%s", id, task.ID)
							attempt = maxRetries + 1
						case <-timer.C:
						}
					} else {
						log.Printf("Worker-%d 任务失败: id=%s type=%s 尝试=%d 耗时=%dms 原因=%v", id, task.ID, task.Type, attempt, time.Since(start).Milliseconds(), err)
						time.Sleep(2 * time.Second) // 简单的休眠防雪崩，生产环境中应根据 err 类型处理
					}

					if attempt > maxRetries {
						log.Printf("Worker-%d 任务重试耗尽: id=%s type=%s 最大重试=%d", id, task.ID, task.Type, maxRetries)
					}
					continue
				}

				if resultProcessor != nil {
					fmt.Printf("Worker-%d 开始结果持久化: id=%s type=%s 尝试=%d\n", id, task.ID, task.Type, attempt)
					if err := resultProcessor.Process(ctx, task, result); err != nil {
						log.Printf("Worker-%d 结果持久化失败: id=%s type=%s 尝试=%d 耗时=%dms 原因=%v", id, task.ID, task.Type, attempt, time.Since(start).Milliseconds(), err)
						time.Sleep(1 * time.Second)
						if attempt > maxRetries {
							log.Printf("Worker-%d 持久化重试耗尽: id=%s type=%s 最大重试=%d", id, task.ID, task.Type, maxRetries)
						}
						continue
					}
				}

				// 3. 成功拿到数据

				log.Printf("Worker-%d 任务成功: id=%s type=%s 耗时=%dms", id, task.ID, task.Type, time.Since(start).Milliseconds())
				
			}
		}
	}
}
