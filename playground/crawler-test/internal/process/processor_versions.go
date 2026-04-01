package process

import (
	"context"
	"time"
	"fmt"

	"crawler-test/internal/crawler"
	"crawler-test/internal/riot"
	"crawler-test/internal/storage"
)

func (p *ResultProcessor) processVersions(ctx context.Context, task crawler.Task, dto *riot.VersionResponse) error {

	fmt.Printf("处理版本信息: task_id=%s versions=%d\n", task.ID, len(*dto))
	seeds := buildVersionSeeds(task.ID, dto, time.Now().UTC())
	fmt.Printf("生成版本信息种子: task_id=%s seeds=%d\n", task.ID, len(seeds))
	return p.store.UpsertVersions(ctx, seeds)
}

func buildVersionSeeds(taskID string, dto *riot.VersionResponse,  fetchedAt time.Time) []storage.VersionSeed {
	if dto == nil || len(*dto) == 0 {
		return nil
	}

	seeds := make([]storage.VersionSeed, 0, len(*dto))
	for _, v := range *dto {
		if !isVersion(v.Name) {
			continue
		}
		
		startTime, err := convertGMTToUnix(v.Mtime)
		if err != nil {
			fmt.Printf("处理版本信息失败: task_id=%s version=%s error=%v\n", taskID, v.Name, err)
			continue
		}
		seeds = append(seeds, storage.VersionSeed{
			Version:      v.Name,
			StartTime:    startTime,
		})
	}	

	return seeds
}

func isVersion(v string) bool {
	// 简单判断版本字符串是数字开头，排除"latest", "pbe" 等非版本字符串
	if len(v) == 0 {
		return false
	}
	return v[0] >= '0' && v[0] <= '9'
}

func convertGMTToUnix(gmt string) (time.Time, error) {
	layout := time.RFC1123 // "Mon, 02 Jan 2006 15:04:05 MST"
	t, err := time.Parse(layout, gmt)
	if err != nil {
		return time.Time{}, fmt.Errorf("解析时间字符串失败: %w", err)
	}
	return t, nil
}
