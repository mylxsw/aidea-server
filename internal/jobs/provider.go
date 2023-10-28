package jobs

import (
	"time"

	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/scheduler"
	"github.com/redis/go-redis/v9"
	cronV3 "github.com/robfig/cron/v3"
)

type Provider struct{}

func (p Provider) Aggregates() []infra.Provider {
	return []infra.Provider{
		scheduler.Provider(
			p.Jobs,
			scheduler.SetLockManagerOption(func(resolver infra.Resolver) scheduler.LockManagerBuilder {
				redisClient := resolver.MustGet(&redis.Client{}).(*redis.Client)
				return func(name string) scheduler.LockManager {
					return New(redisClient, name, 1*time.Minute)
				}
			}),
		),
	}
}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func() *cronV3.Cron {
		log.Debugf("初始化定时任务管理器, Location=%s", time.Local.String())
		return cronV3.New(
			cronV3.WithSeconds(),
			cronV3.WithLogger(cronLogger{}),
			cronV3.WithLocation(time.Local),
		)
	})
}

func (Provider) Jobs(cc infra.Resolver, creator scheduler.JobCreator) {
	// 每天凌晨 0:10 执行一次配额使用统计
	if err := creator.Add(
		"quota-usage-statistics",
		"0 10 0 * * *",
		scheduler.WithoutOverlap(QuotaUsageStatisticsJob),
	); err != nil {
		log.Errorf("注册定时任务 quota-usage-statistics 失败: %v", err)
	}

	// 每 5s 执行一次 PendingTask 任务
	if err := creator.Add(
		"pending-task",
		"*/5 * * * * *",
		scheduler.WithoutOverlap(queue.PendingTaskJob).SkipCallback(func() {
			log.Debugf("上一次 pending-task 任务还未执行完毕，本次任务将被跳过")
		}),
	); err != nil {
		log.Errorf("注册定时任务 pending-task 失败: %v", err)
	}

	// 每 60 分钟 执行一次 HealthCheck 任务
	if err := creator.Add(
		"healthcheck-task",
		"0 */60 * * * *",
		scheduler.WithoutOverlap(HealthCheckJob),
	); err != nil {
		log.Errorf("注册定时任务 healthcheck-task 失败: %v", err)
	}

	// 注册 Gallery 自动随机排序任务
	if err := creator.Add(
		"gallery-sort-task",
		"0 */60 * * * *",
		scheduler.WithoutOverlap(GallerySortJob),
	); err != nil {
		log.Errorf("注册定时任务 gallery-sort-task 失败: %v", err)
	}

	// 清理过期任务
	if err := creator.Add(
		"clear-expired-task",
		"0 0 0 * * *",
		scheduler.WithoutOverlap(queue.ClearExpiredTaskJob),
	); err != nil {
		log.Errorf("注册定时任务 clear-expired-task 失败: %v", err)
	}

	// 清理过期缓存
	if err := creator.Add(
		"clear-expired-cache",
		"0 0 0 * * *",
		scheduler.WithoutOverlap(queue.ClearExpiredCacheJob),
	); err != nil {
		log.Errorf("注册定时任务 clear-expired-cache 失败: %v", err)
	}

	// 用户注册通知（管理）
	if err := creator.Add(
		"user-signup-notification",
		"@daily",
		scheduler.WithoutOverlap(UserSignupNotificationJob),
	); err != nil {
		log.Errorf("注册定时任务 user-signup-notification 失败: %v", err)
	}
}

func (Provider) ShouldLoad(c infra.FlagContext) bool {
	return c.Bool("enable-scheduler")
}

type cronLogger struct {
}

func (l cronLogger) Info(msg string, keysAndValues ...interface{}) {
	// Just drop it, we don't care
}

func (l cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Errorf("[glacier] %s: %v", msg, err)
}
