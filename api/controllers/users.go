package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/internal/service"

	"github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/go-redis/redis_rate/v10"
	"github.com/hashicorp/go-uuid"
	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/rate"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"github.com/redis/go-redis/v9"
	passwordvalidator "github.com/wagslane/go-password-validator"
)

// UserController 用户控制器
type UserController struct {
	translater youdao.Translater        `autowire:"@"`
	rds        *redis.Client            `autowire:"@"`
	limiter    *rate.RateLimiter        `autowire:"@"`
	queue      *queue.Queue             `autowire:"@"`
	userRepo   *repo.UserRepo           `autowire:"@"`
	conf       *config.Config           `autowire:"@"`
	userSrv    *service.UserService     `autowire:"@"`
	secSrv     *service.SecurityService `autowire:"@"`
}

// NewUserController 创建用户控制器
func NewUserController(resolver infra.Resolver) web.Controller {
	ctl := UserController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *UserController) Register(router web.Router) {
	router.Group("/users", func(router web.Router) {
		// 获取当前用户信息
		router.Get("/current", ctl.CurrentUser)
		router.Post("/current/avatar", ctl.UpdateAvatar)
		router.Post("/current/realname", ctl.UpdateRealname)

		// 获取当前用户配额详情
		router.Get("/quota", ctl.UserQuota)
		// 获取当前用户配额情况统计
		router.Get("/quota/usage-stat", ctl.UserQuotaUsageStatistics)

		// 重置密码
		router.Post("/reset-password/sms-code", ctl.SendResetPasswordSMSCode)
		router.Post("/reset-password", ctl.ResetPassword)
		// 账号销毁
		router.Delete("/destroy", ctl.Destroy)
		router.Post("/destroy/sms-code", ctl.SendResetPasswordSMSCode)
	})
}

// UpdateAvatar 更新用户头像
func (ctl *UserController) UpdateAvatar(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	avatarURL := webCtx.Input("avatar_url")
	if !strings.HasPrefix(avatarURL, ctl.conf.StorageDomain) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "非法的头像地址"), http.StatusBadRequest)
	}

	if err := ctl.userRepo.UpdateAvatarURL(ctx, user.ID, avatarURL); err != nil {
		log.WithFields(log.Fields{
			"user_id": user.ID,
		}).Errorf("failed to update user avatar: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	u, err := ctl.userSrv.GetUserByID(ctx, user.ID, true)
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"user": auth.CreateAuthUserFromModel(u),
	})
}

// UpdateRealname 更新用户真实姓名
func (ctl *UserController) UpdateRealname(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	realname := webCtx.Input("realname")
	if realname == "" || len(realname) > 50 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "昵称无效，请重新设置"), http.StatusBadRequest)
	}

	checkRes := ctl.secSrv.NicknameDetect(realname)
	if !checkRes.Safe {
		log.WithFields(log.Fields{
			"user_id": user.ID,
			"details": checkRes,
			"content": realname,
		}).Warningf("用户 %d 违规，违规内容：%s", user.ID, checkRes.Reason)
		return webCtx.JSONError("内容违规，已被系统拦截，如有疑问邮件联系：support@aicode.cc", http.StatusNotAcceptable)
	}

	if err := ctl.userRepo.UpdateRealname(ctx, user.ID, realname); err != nil {
		log.WithFields(log.Fields{
			"user_id": user.ID,
		}).Errorf("failed to update user realname: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	u, err := ctl.userSrv.GetUserByID(ctx, user.ID, true)
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"user": auth.CreateAuthUserFromModel(u),
	})
}

// Destroy 销毁账号
func (ctl *UserController) Destroy(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	verifyCodeId := strings.TrimSpace(webCtx.Input("verify_code_id"))
	if verifyCodeId == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码 ID 不能为空"), http.StatusBadRequest)
	}

	verifyCode := strings.TrimSpace(webCtx.Input("verify_code"))
	if verifyCode == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码不能为空"), http.StatusBadRequest)
	}

	// 流控：每个用户每 60 分钟只能重置密码 5 次
	err := ctl.limiter.Allow(ctx, fmt.Sprintf("auth:reset-password:%s:limit", user.Phone), rate.MaxRequestsInPeriod(5, 60*time.Minute))
	if err != nil {
		if err == rate.ErrRateLimitExceeded {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "操作频率过高，请稍后再试"), http.StatusTooManyRequests)
		}

		log.WithFields(log.Fields{
			"username": user.Phone,
		}).Errorf("failed to check verify code rate limit: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	// 检查验证码是否正确
	realVerifyCode, err := ctl.rds.Get(ctx, fmt.Sprintf("auth:verify-code:%s:%s", verifyCodeId, user.Phone)).Result()
	if err != nil {
		if err != redis.Nil {
			log.WithFields(log.Fields{
				"username": user.Phone,
				"id":       verifyCodeId,
				"code":     verifyCode,
			}).Errorf("failed to get verify code: %s", err)
		}
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码已过期，请重新获取"), http.StatusBadRequest)
	}

	if realVerifyCode != verifyCode {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码错误"), http.StatusBadRequest)
	}

	_ = ctl.rds.Del(ctx, fmt.Sprintf("auth:verify-code:%s:%s", verifyCodeId, user.Phone)).Err()

	if err := ctl.userRepo.UpdateStatus(ctx, user.ID, repo.UserStatusDeleted); err != nil {
		log.With(user).Errorf("failed to update user status: %s", err)
		return webCtx.JSONError("内部错误，请稍后再试", http.StatusInternalServerError)
	}

	// 撤销 Apple 账号绑定
	if user.AppleUID != "" {
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("revoke apple account token panic: %v", err)
				}
			}()

			client := apple.New()
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			secret, err := apple.GenerateClientSecret(
				ctl.conf.AppleSignIn.Secret,
				ctl.conf.AppleSignIn.TeamID,
				"cc.aicode.flutter.askaide.askaide",
				ctl.conf.AppleSignIn.KeyID,
			)
			if err != nil {
				log.Errorf("generate client secret for revoke apple account failed: %v", err)
			} else {
				req := apple.RevokeAccessTokenRequest{
					ClientID:     "cc.aicode.flutter.askaide.askaide",
					ClientSecret: secret,
					AccessToken:  user.AppleUID,
				}
				var resp apple.RevokeResponse
				if err := client.RevokeAccessToken(ctx, req, &resp); err != nil && err != io.EOF {
					log.Errorf("revoke apple access token failed: %v", err)
				}
			}
		}()
	}

	return webCtx.JSON(web.M{})
}

// SendResetPasswordSMSCode 发送重置密码的短信验证码
func (ctl *UserController) SendResetPasswordSMSCode(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	username := user.Phone
	if username == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "请退出后重新登录，绑定手机号后再进行操作"), http.StatusBadRequest)
	}

	// 流控：每个用户每分钟只能发送一次短信
	smsCodeRateLimitPerMinute := fmt.Sprintf("auth:sms-code:limit:%s:min", username)
	optCountPerMin, err := ctl.limiter.OperationCount(ctx, smsCodeRateLimitPerMinute)
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
		}).Errorf("failed to check sms code rate limit: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	if optCountPerMin > 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "发送短信验证码过于频繁，请稍后再试"), http.StatusTooManyRequests)
	}

	// 流控：每个用户每天只能发送 5 次短信
	smsCodeRateLimitPerDay := fmt.Sprintf("auth:sms-code:limit:%s:day", username)
	optCountPerDay, err := ctl.limiter.OperationCount(ctx, smsCodeRateLimitPerDay)
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
		}).Errorf("failed to check sms code rate limit: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	if optCountPerDay >= 5 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "当前账号今日发送验证码次数已达上限，请 24 小时后再试"), http.StatusTooManyRequests)
	}

	// 业务检查
	if _, err := ctl.userRepo.GetUserByPhone(ctx, username); err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "用户不存在"), http.StatusBadRequest)
		}

		log.WithFields(log.Fields{
			"username": username,
		}).Errorf("failed to get user: %s", err)

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	// 生成验证码
	id, _ := uuid.GenerateUUID()
	code := verifyCodeGenerator()

	smsPayload := &queue.SMSVerifyCodePayload{
		Receiver:  username,
		Code:      code,
		CreatedAt: time.Now(),
	}

	taskId, err := ctl.queue.Enqueue(smsPayload, queue.NewSMSVerifyCodeTask, asynq.Queue("mail"))
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"id":       id,
			"code":     code,
		}).Errorf("failed to enqueue mail task: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	if err := ctl.rds.SetNX(ctx, fmt.Sprintf("auth:verify-code:%s:%s", id, username), code, 15*time.Minute).Err(); err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"id":       id,
			"code":     code,
			"task_id":  taskId,
		}).Errorf("failed to set email code: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	// 设置流控
	if err := ctl.limiter.OperationIncr(ctx, smsCodeRateLimitPerMinute, 50*time.Second); err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"id":       id,
			"code":     code,
			"task_id":  taskId,
		}).Errorf("failed to set email code rate limit: %s", err)
	}

	if err := ctl.limiter.OperationIncr(ctx, smsCodeRateLimitPerDay, 24*time.Hour); err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"id":       id,
			"code":     code,
			"task_id":  taskId,
		}).Errorf("failed to set email code rate limit: %s", err)
	}

	return webCtx.JSON(web.M{
		"id": id,
	})
}

// ResetPassword 重置密码
func (ctl *UserController) ResetPassword(ctx context.Context, webCtx web.Context, user *auth.User, userRepo *repo.UserRepo, limiter *redis_rate.Limiter) web.Response {
	password := strings.TrimSpace(webCtx.Input("password"))
	if len(password) < 8 || len(password) > 20 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "密码长度必须在 8-20 位之间"), http.StatusBadRequest)
	}

	if err := passwordvalidator.Validate(password, minEntropyBits); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "密码强度不够，建议使用字母、数字、特殊符号组合"), http.StatusBadRequest)
	}

	verifyCodeId := strings.TrimSpace(webCtx.Input("verify_code_id"))
	if verifyCodeId == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码 ID 不能为空"), http.StatusBadRequest)
	}

	verifyCode := strings.TrimSpace(webCtx.Input("verify_code"))
	if verifyCode == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码不能为空"), http.StatusBadRequest)
	}

	// 流控：每个用户每 60 分钟只能重置密码 5 次
	err := ctl.limiter.Allow(ctx, fmt.Sprintf("auth:reset-password:%s:limit", user.Phone), rate.MaxRequestsInPeriod(5, 60*time.Minute))
	if err != nil {
		if err == rate.ErrRateLimitExceeded {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "操作频率过高，请稍后再试"), http.StatusTooManyRequests)
		}

		log.WithFields(log.Fields{
			"username": user.Phone,
		}).Errorf("failed to check verify code rate limit: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	// 检查验证码是否正确
	realVerifyCode, err := ctl.rds.Get(ctx, fmt.Sprintf("auth:verify-code:%s:%s", verifyCodeId, user.Phone)).Result()
	if err != nil {
		if err != redis.Nil {
			log.WithFields(log.Fields{
				"username": user.Phone,
				"id":       verifyCodeId,
				"code":     verifyCode,
			}).Errorf("failed to get verify code: %s", err)
		}
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码已过期，请重新获取"), http.StatusBadRequest)
	}

	if realVerifyCode != verifyCode {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "验证码错误"), http.StatusBadRequest)
	}

	_ = ctl.rds.Del(ctx, fmt.Sprintf("auth:verify-code:%s:%s", verifyCodeId, user.Phone)).Err()

	if err := userRepo.UpdatePassword(ctx, user.ID, password); err != nil {
		log.WithFields(log.Fields{
			"username": user.Phone,
		}).Errorf("failed to update password: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// CurrentUser 获取当前用户信息
func (ctl *UserController) CurrentUser(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
	}

	u, err := ctl.userSrv.GetUserByID(ctx, user.ID, true)
	if err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "用户不存在"), http.StatusNotFound)
		}

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "内部错误，请稍后再试"), http.StatusInternalServerError)
	}

	if u.Status == repo.UserStatusDeleted {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "用户不存在"), http.StatusNotFound)
	}

	user = auth.CreateAuthUserFromModel(u)
	if user.Phone != "" {
		user.Phone = helper.MaskPhoneNumber(user.Phone)
	}

	return webCtx.JSON(web.M{
		"user":  user,
		"quota": quota,
		"control": web.M{
			"is_set_pwd":         user.IsSetPassword,
			"enable_invite":      user.InviteCode != "",
			"invite_message":     fmt.Sprintf("【AIdea】玩转 GPT，实在太有趣啦！\n\n用我的专属邀请码 %s 注册，不仅免费用，还有额外奖励！\n\n快去下载 aidea.aicode.cc ，我在未来世界等你！", user.InviteCode),
			"user_card_bg":       "https://ssl.aicode.cc/ai-server/assets/quota-card-bg.webp-thumb1000",
			"invite_card_bg":     "https://ssl.aicode.cc/ai-server/assets/invite-card-bg.webp-thumb1000",
			"invite_card_color":  "FF000000",
			"invite_card_slogan": fmt.Sprintf("你与好友均可获得 %d 个智慧果\n好友充值享佣金\n成功邀请多人奖励可累积", coins.InvitedGiftCoins),
			"with_lab":           user.WithLab,
		},
	})
}

// UserQuota 获取当前用户配额详情
func (ctl *UserController) UserQuota(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	quotas, err := quotaRepo.GetUserQuotaDetails(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	var rest int64
	for _, quota := range quotas {
		if quota.Expired || quota.Rest <= 0 {
			continue
		}

		rest += quota.Rest
	}

	return webCtx.JSON(web.M{
		"details": quotas,
		"total":   rest,
	})
}

type QuotaUsageStatistics struct {
	Date string `json:"date"`
	Used int64  `json:"used"`
}

// UserQuotaUsageStatistics 获取当前用户配额使用情况统计
func (ctl *UserController) UserQuotaUsageStatistics(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	usages, err := quotaRepo.GetQuotaStatisticsRecently(ctx, user.ID, 30)
	if err != nil {
		log.WithFields(log.Fields{"user_id": user.ID}).Debugf("get quota statistics failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	usagesMap := array.ToMap(usages, func(item model.QuotaStatistics, _ int) string {
		return item.CalDate.Format("2006-01-02")
	})

	// 生成当前日期以及前 30 天的日期列表
	results := make([]QuotaUsageStatistics, 0)
	for i := 0; i < 30; i++ {
		// 最多统计到用户注册日期
		if time.Now().AddDate(0, 0, -i).Before(user.CreatedAt) {
			break
		}

		curDate := time.Now().AddDate(0, 0, -i-1).Format("2006-01-02")
		if usage, ok := usagesMap[curDate]; ok {
			results = append(results, QuotaUsageStatistics{
				Date: curDate,
				Used: usage.Used,
			})
		} else {
			results = append(results, QuotaUsageStatistics{
				Date: curDate,
				Used: 0,
			})
		}
	}

	return webCtx.JSON(web.M{
		"usages": results,
	})
}
