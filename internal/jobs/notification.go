package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/redis/go-redis/v9"
	"gopkg.in/guregu/null.v3"
)

// UserSignupNotificationJob 用户注册通知任务
func UserSignupNotificationJob(ctx context.Context, db *sql.DB, rds *redis.Client, ding *dingding.Dingding) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	lastID, err := rds.Get(ctx, "user-signup-notification:last-id").Int64()
	if err != nil && err != redis.Nil {
		log.Errorf("获取上次通知的最大用户 ID 失败: %v", err)
		return err
	}

	row := db.QueryRow("SELECT max(id), count(*), min(created_at) FROM users WHERE id > ?", lastID)
	var maxID, signupCount null.Int
	var firstSignupTime null.Time
	if err := row.Scan(&maxID, &signupCount, &firstSignupTime); err != nil {
		log.Errorf("获取注册用户数量失败: %v", err)
		return err
	}

	if signupCount.ValueOrZero() > 0 {
		content := fmt.Sprintf(
			`%s 至今已有 %d 位用户注册，最新注册用户 ID 为 %d`,
			firstSignupTime.ValueOrZero().Format("2006-01-02 15:04:05"),
			signupCount.ValueOrZero(),
			maxID.ValueOrZero(),
		)

		if ding != nil {
			if err := ding.Send(dingding.NewMarkdownMessage(content, content, []string{})); err != nil {
				log.WithFields(log.Fields{"content": content}).Errorf("send dingding message failed: %s", err)
			}
		} else {
			log.WithFields(log.Fields{"content": content}).Debugf("dingding client is nil, skip send message")
		}

		return rds.Set(ctx, "user-signup-notification:last-id", maxID.ValueOrZero(), 0).Err()
	}

	return nil
}
