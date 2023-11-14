package jobs

import (
	"context"
	"database/sql"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
	"time"

	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/glacier/log"
)

func QuotaUsageStatisticsJob(ctx context.Context, db *sql.DB) error {
	return QuotaUsageStatistics(ctx, db, time.Now())
}

func QuotaUsageStatistics(ctx context.Context, db *sql.DB, date time.Time) error {
	statisticDate := date.Add(-time.Hour * 24).Format("2006-01-02")

	startTime := statisticDate + " 00:00:00"
	endTime := date.Format("2006-01-02") + " 00:00:00"

	q := query.Builder().
		Table(model2.QuotaUsageTable()).
		Select(query.Raw("DISTINCT user_id")).
		Where(model2.FieldQuotaUsageCreatedAt, ">=", startTime).
		Where(model2.FieldQuotaUsageCreatedAt, "<", endTime)

	userIds, err := eloquent.Query(ctx, db, q, func(row eloquent.Scanner) (int64, error) {
		var userId int64
		if err := row.Scan(&userId); err != nil {
			return 0, err
		}

		return userId, nil
	})

	if err != nil {
		log.Errorf("执行配额每日统计任务失败，查询活跃用户失败: %v", err)
		return err
	}

	log.Infof("执行配额每日统计任务(%s), 查询到 %d 个活跃用户", statisticDate, len(userIds))

	for _, userId := range userIds {
		processUserQuotaUsageStatistics(ctx, db, userId, statisticDate, startTime, endTime)
	}

	log.Infof("执行配额每日统计任务(%s)成功", statisticDate)

	return nil
}

func processUserQuotaUsageStatistics(ctx context.Context, db *sql.DB, userId int64, statisticDate, startTime, endTime string) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("配额每日统计任务，统计用户 %d 失败: %v", userId, err)
		}
	}()

	q := query.Builder().
		Table(model2.QuotaUsageTable()).
		Select(query.Raw("SUM(used)")).
		Where(model2.FieldQuotaUsageCreatedAt, ">=", startTime).
		Where(model2.FieldQuotaUsageCreatedAt, "<", endTime).
		Where(model2.FieldQuotaUsageUserId, userId)

	used, err := eloquent.Query(ctx, db, q, func(row eloquent.Scanner) (int64, error) {
		var used int64
		if err := row.Scan(&used); err != nil {
			return 0, err
		}

		return used, nil
	})

	if err != nil {
		panic(err)
	}

	res := query.KV{
		model2.FieldQuotaStatisticsCalDate: statisticDate,
		model2.FieldQuotaStatisticsUserId:  userId,
		model2.FieldQuotaStatisticsUsed:    used[0],
	}
	if _, err := model2.NewQuotaStatisticsModel(db).Create(ctx, res); err != nil {
		panic(err)
	}

	log.Debugf("配额每日统计任务，统计用户 %d 成功，统计日期: %s, 统计结果: %d", userId, statisticDate, used[0])
}
