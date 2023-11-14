package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/api/billing"
	"github.com/mylxsw/aidea-server/api/openai"
	"github.com/mylxsw/aidea-server/pkg/rate"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/token"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/listener"
	"github.com/mylxsw/glacier/web"
)

var ErrUserDestroyed = errors.New("user is destroyed")

type Provider struct{}

// Aggregates 实现 infra.ProviderAggregate 接口
func (Provider) Aggregates() []infra.Provider {
	return []infra.Provider{
		web.RepeatableProvider(
			listener.FlagContext("api-listen"),
			web.SetRouteHandlerOption(routes),
			web.SetExceptionHandlerOption(exceptionHandler),
			web.SetIgnoreLastSlashOption(true),
		),
	}
}

// Register 实现 infra.Provider 接口
func (Provider) Register(binder infra.Binder) {}

// exceptionHandler 异常处理器
func exceptionHandler(ctx web.Context, err interface{}) web.Response {
	if err == ErrUserDestroyed {
		return ctx.JSONError("账号不可用：用户账号已注销", http.StatusForbidden)
	}

	debug.PrintStack()

	log.Errorf("request %s failed: %v, stack is %s", ctx.Request().Raw().URL.Path, err, string(debug.Stack()))
	return ctx.JSONWithCode(web.M{"error": fmt.Sprintf("%v", err)}, http.StatusInternalServerError)
}

// routes 注册路由规则
func routes(resolver infra.Resolver, router web.Router, mw web.RequestMiddleware) {
	conf := resolver.MustGet((*config.Config)(nil)).(*config.Config)

	mws := make([]web.HandlerDecorator, 0)
	// 跨域请求处理
	if conf.EnableCORS {
		mws = append(mws, mw.CORS("*"))
	}

	// 添加 web 中间件
	resolver.MustResolve(func(tk *token.Token, userSrv *service.UserService, limiter *redis_rate.Limiter, translater youdao.Translater) {
		mws = append(mws, mw.BeforeInterceptor(func(webCtx web.Context) web.Response {
			// 跨域请求处理，OPTIONS 请求直接返回
			if webCtx.Method() == http.MethodOptions {
				return webCtx.JSON(web.M{})
			}

			// 基于客户端 IP 的限流
			clientIP := webCtx.Header("X-Real-IP")
			if clientIP == "" {
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			m, err := limiter.Allow(ctx, fmt.Sprintf("request-ip:%s:freq", clientIP), rate.MaxRequestsInPeriod(30, 10*time.Second))
			if err != nil {
				return webCtx.JSONError("rate-limiter: interapi server error", http.StatusInternalServerError)
			}

			if m.Remaining <= 0 {
				log.WithFields(log.Fields{"ip": clientIP}).Warningf("client request too frequently")
				return webCtx.JSONError(common.Text(webCtx, translater, "请求频率过高，请稍后再试"), http.StatusTooManyRequests)
			}

			return nil
		}))

		mws = append(mws,
			mw.CustomAccessLog(func(cal web.CustomAccessLog) {
				// 记录访问日志
				platform := readFromWebContext(cal.Context, "platform")

				log.F(log.M{
					"method":   cal.Method,
					"url":      cal.URL,
					"code":     cal.ResponseCode,
					"elapse":   cal.Elapse.Milliseconds(),
					"ip":       cal.Context.Header("X-Real-IP"),
					"lang":     readFromWebContext(cal.Context, "language"),
					"ver":      readFromWebContext(cal.Context, "client-version"),
					"plat":     platform,
					"plat-ver": readFromWebContext(cal.Context, "platform-version"),
				}).Debug("request")
			}),
			mw.AuthHandler(func(webCtx web.Context, typ string, credential string) error {
				// 注入客户端信息
				webCtx.Provide(func() *auth.ClientInfo {
					return &auth.ClientInfo{
						Version:         readFromWebContext(webCtx, "client-version"),
						Platform:        readFromWebContext(webCtx, "platform"),
						PlatformVersion: readFromWebContext(webCtx, "platform-version"),
						Language:        readFromWebContext(webCtx, "language"),
						IP:              webCtx.Header("X-Real-IP"),
					}
				})

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// 查询用户信息
				var user *auth.User
				if u, err := userSrv.GetUserByAPIKey(ctx, credential); err != nil {
					if errors.Is(err, repo2.ErrNotFound) {
						return errors.New("invalid auth credential, user not found")
					}

					return err
				} else {
					if u.Status == repo2.UserStatusDeleted {
						return ErrUserDestroyed
					}

					user = auth.CreateAuthUserFromModel(u)
				}

				if user == nil {
					return errors.New("invalid auth credential, user not found")
				}

				webCtx.Provide(func() *auth.User { return user })
				webCtx.Provide(func() *auth.UserOptional {
					return &auth.UserOptional{User: user}
				})

				return nil
			}),
		)
	})

	// 注册控制器，所有的控制器 API 都以 `/server` 作为接口前缀
	r := router.WithMiddleware(mws...)
	r.Controllers(
		"/v1",
		controllers.NewOpenAIController(resolver, conf, true),
		openai.NewOpenAICompatibleController(resolver),
	)

	r.Controllers(
		"/dashboard",
		billing.NewBillingController(resolver),
	)
}

// readFromWebContext 优先读取请求参数，请求参数不存在，读取请求头
func readFromWebContext(webCtx web.Context, key string) string {
	val := webCtx.Input(key)
	if val != "" {
		return val
	}

	val = webCtx.Header(strings.ToUpper(key))
	if val != "" {
		return val
	}

	return webCtx.Header("X-" + strings.ToUpper(key))
}
