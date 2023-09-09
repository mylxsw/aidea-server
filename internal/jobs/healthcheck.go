package jobs

import (
	"context"

	"github.com/mylxsw/aidea-server/internal/dingding"
)

func HealthCheckJob(ctx context.Context, ding *dingding.Dingding) error {
	// TODO  这里添加一些检查任务，如接口的余额查询，通知
	return nil
}
