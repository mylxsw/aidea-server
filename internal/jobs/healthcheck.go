package jobs

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
)

func HealthCheckJob(ctx context.Context, conf *config.Config, ding *dingding.Dingding, stab *stabilityai.StabilityAI) error {
	// TODO  这里添加一些检查任务，如接口的余额查询，通知

	// Stability.ai 账号余额不足预警
	if conf.EnableStabilityAI {
		stabBalance, err := queryStabilityAIBalance(ctx, stab)
		if err != nil {
			log.Errorf("查询 Stability.ai 账号余额失败: %v", err)
		} else {
			if stabBalance < 100 {
				title := "Stability.ai 账号余额不足，请及时充值"
				content := fmt.Sprintf("Stability.ai 当前账户余额 %.2f credits，已低于预警值 100，请及时充值", stabBalance)
				if err := ding.Send(dingding.NewMarkdownMessage(title, content, []string{})); err != nil {
					log.WithFields(log.Fields{"content": content}).Errorf("send dingding message failed: %s", err)
				}
			}
		}
	}

	return nil
}

func queryStabilityAIBalance(ctx context.Context, stab *stabilityai.StabilityAI) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return stab.AccountBalance(ctx)
}
