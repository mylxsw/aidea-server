package queue

import (
	"context"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"time"

	"github.com/mylxsw/glacier/log"
)

func ClearExpiredTaskJob(ctx context.Context, queueRepo *repo2.QueueRepo) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// 清理过期的 PendingTasks
	if err := queueRepo.RemovePendingTasks(ctx, time.Now().AddDate(0, 0, -3)); err != nil {
		log.Errorf("清理过期的 PendingTasks 失败: %v", err)
	}

	// 清理过期的 QueueTasks
	if err := queueRepo.RemoveQueueTasks(ctx, time.Now().AddDate(0, 0, -3)); err != nil {
		log.Errorf("清理过期的 QueueTasks 失败: %v", err)
	}

	return nil
}

func ClearExpiredCacheJob(ctx context.Context, cacheRepo *repo2.CacheRepo) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := cacheRepo.GC(ctx); err != nil {
		log.Errorf("清理过期的缓存失败: %v", err)
	}

	return nil
}
