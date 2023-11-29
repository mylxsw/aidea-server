package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/rate"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/token"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/gorilla/mux"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers"
	"github.com/mylxsw/aidea-server/server/controllers/admin"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/aidea-server/server/controllers/interapi"
	v2 "github.com/mylxsw/aidea-server/server/controllers/v2"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/listener"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/str"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var ErrUserDestroyed = errors.New("user is destroyed")

type Provider struct{}

// Aggregates 实现 infra.ProviderAggregate 接口
func (Provider) Aggregates() []infra.Provider {
	return []infra.Provider{
		web.Provider(
			listener.FlagContext("listen"),
			web.SetRouteHandlerOption(routes),
			web.SetMuxRouteHandlerOption(muxRoutes),
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

	// 需要鉴权的 URLs
	needAuthPrefix := []string{
		"/v1/chat",            // OpenAI chat
		"/v1/audio",           // OpenAI audio to text
		"/v1/group-chat",      // 群聊
		"/v1/users",           // 用户管理
		"/v1/api-keys",        // API Key 管理
		"/v1/translate",       // 翻译 API
		"/v1/storage",         // 存储 API
		"/v1/creative-island", // 创作岛
		"/v1/tasks",           // 任务管理
		"/v1/payment/apple",   // Apple 支付管理
		"/v1/payment/alipay",  // 支付宝支付管理 @deprecated(since 1.0.8)
		"/v1/payment/others",  // 支付宝支付管理
		"/v1/payment/status",  // 支付状态查询
		"/v1/auth/bind-phone", // 绑定手机号码
		"/v1/rooms",           // 数字人管理
		"/v1/room-galleries",  // 数字人 Gallery
		"/v1/voice",           // 语音合成
		"/v1/admin",           // 管理员接口

		// v2 版本
		"/v2/creative-island", // 创作岛
		"/v2/rooms",           // 数字人管理
	}

	// Prometheus 监控指标
	reqCounterMetric := BuildCounterVec(
		"aidea",
		"http_request_count",
		"http request counts",
		[]string{"method", "path", "code", "platform"},
	)

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
				path, _ := cal.Context.CurrentRoute().GetPathTemplate()
				reqCounterMetric.WithLabelValues(
					cal.Method,
					path,
					strconv.Itoa(cal.ResponseCode),
					platform,
				).Inc()

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
			authHandler(
				func(webCtx web.Context, credential string) error {
					urlPath := webCtx.Request().Raw().URL.Path
					needAuth := str.HasPrefixes(urlPath, needAuthPrefix)

					claims, err := tk.ParseToken(credential)
					if needAuth && err != nil {
						return errors.New("invalid auth credential")
					}

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					// 查询用户信息
					var user *auth.User
					if u, err := userSrv.GetUserByID(ctx, claims.Int64Value("id"), false); err != nil {
						if needAuth {
							if errors.Is(err, repo2.ErrNotFound) {
								return errors.New("invalid auth credential, user not found")
							}

							return err
						}
					} else {
						if u.Status == repo2.UserStatusDeleted {
							if needAuth {
								return ErrUserDestroyed
							}

							u = nil
						}

						user = auth.CreateAuthUserFromModel(u)
					}

					if needAuth {
						if user == nil {
							return errors.New("invalid auth credential, user not found")
						}

						// // 请求限流(基于用户 ID)
						// ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
						// defer cancel()

						// m, err := limiter.Allow(ctx, fmt.Sprintf("request:%d:freq", claims.Int64Value("id")), rate.MaxRequestsInPeriod(10, 1*time.Minute))
						// if err != nil {
						// 	return errors.New("rate-limiter: interapi server error")
						// }

						// if m.Remaining <= 0 {
						// 	return errors.New("request frequency is too high, please try again later")
						// }

						// 管理员接口，只对内部用户开放
						if strings.HasPrefix(urlPath, "/v1/admin/") && !user.InternalUser() {
							return errors.New("permission denied")
						}

						webCtx.Provide(func() *auth.User { return user })
						webCtx.Provide(func() *auth.UserOptional {
							return &auth.UserOptional{User: user}
						})
					} else {
						webCtx.Provide(func() *auth.UserOptional { return &auth.UserOptional{User: user} })
					}

					return nil
				},
				func(ctx web.Context) bool {
					// 注入客户端信息
					ctx.Provide(func() *auth.ClientInfo {
						return &auth.ClientInfo{
							Version:         readFromWebContext(ctx, "client-version"),
							Platform:        readFromWebContext(ctx, "platform"),
							PlatformVersion: readFromWebContext(ctx, "platform-version"),
							Language:        readFromWebContext(ctx, "language"),
							IP:              ctx.Header("X-Real-IP"),
						}
					})

					// 必须要鉴权的 URL
					needAuth := str.HasPrefixes(ctx.Request().Raw().URL.Path, needAuthPrefix)
					if needAuth {
						return false
					}

					authHeader := strings.ToLower(readFromWebContext(ctx, "authorization"))
					// 如果有 Authorization 头，且 Authorization 头以 Bearer 开头，则需要鉴权
					if strings.HasPrefix(authHeader, "bearer ") {
						return false
					}

					ctx.Provide(func() *auth.UserOptional { return &auth.UserOptional{User: nil} })
					return true
				},
			),
		)
	})

	// 注册控制器，所有的控制器 API 都以 `/server` 作为接口前缀
	r := router.WithMiddleware(mws...)
	r.Controllers(
		"/v1",
		controllers.NewPromptController(resolver),
		controllers.NewExampleController(resolver),
		controllers.NewProxiesController(conf),
		controllers.NewModelController(conf),
		controllers.NewCreativeIslandController(resolver, conf),
		controllers.NewCreativeController(resolver, conf),
		controllers.NewImageController(resolver),
		controllers.NewDiagnosisController(resolver),

		controllers.NewTranslateController(resolver, conf),
		controllers.NewOpenAIController(resolver, conf, false),
		controllers.NewGroupChatController(resolver),

		controllers.NewAuthController(resolver, conf),
		controllers.NewUserController(resolver),
		controllers.NewAPIKeyController(resolver),
		controllers.NewUploadController(resolver, conf),
		controllers.NewTaskController(resolver, conf),
		controllers.NewAppleAuthController(resolver, conf),
		controllers.NewPaymentController(resolver),
		controllers.NewRoomController(resolver),
		controllers.NewVoiceController(resolver),
		controllers.NewNotificationController(resolver),
		controllers.NewArticleController(resolver),
	)

	r.Controllers(
		"/v2",
		v2.NewCreativeIslandController(resolver, conf),
		v2.NewModelController(conf),
		v2.NewRoomController(resolver),
	)

	// 内部给管理接口
	r.Controllers(
		"/internal",
		interapi.NewManagerController(resolver),
	)

	// 管理员接口
	r.Controllers(
		"/v1/admin",
		admin.NewCreativeIslandController(resolver),
	)

	// 公开访问信息
	r.Controllers(
		"/public",
		controllers.NewInfoController(resolver),
		controllers.NewPaymentPublicController(resolver),
	)
}

func muxRoutes(resolver infra.Resolver, router *mux.Router) {
	resolver.MustResolve(func(conf *config.Config) {
		// 添加 prometheus metrics 支持
		router.PathPrefix("/metrics").Handler(PrometheusHandler{token: conf.PrometheusToken})
		// 添加健康检查接口支持
		router.PathPrefix("/health").Handler(HealthCheck{})
		// Universal Links
		router.PathPrefix("/.well-known/apple-app-site-association").HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Add("Content-Type", "application/json")

			data := `{"applinks":{"apps":[],"details":[{"appID":"N95437SZ2A.cc.aicode.flutter.askaide.askaide","paths":["/wechat-login/*","/wechat-links/*"]}]}}`
			if conf.UniversalLinkConfig != "" {
				data = conf.UniversalLinkConfig
			}

			writer.Write([]byte(data))
		})
	})
}

type PrometheusHandler struct {
	token string
}

func (h PrometheusHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	authHeader := request.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if h.token != "" && tokenStr != h.token {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	promhttp.Handler().ServeHTTP(writer, request)
}

type HealthCheck struct{}

func (h HealthCheck) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(`{"status": "UP"}`))
}

var counterVecs = make(map[string]*prometheus.CounterVec)
var lock sync.Mutex

func BuildCounterVec(namespace, name, help string, tags []string) *prometheus.CounterVec {
	lock.Lock()
	defer lock.Unlock()

	cacheKey := fmt.Sprintf("%s:%s:%s", namespace, name, help)
	if sv, ok := counterVecs[cacheKey]; ok {
		return sv
	}
	// prometheus metric
	counterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      name,
		Help:      help,
	}, tags)

	if err := prometheus.Register(counterVec); err != nil {
		log.Errorf("register prometheus metric failed: %v", err)
	}

	counterVecs[cacheKey] = counterVec

	return counterVec
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

func authHandler(cb func(ctx web.Context, credential string) error, skip func(ctx web.Context) bool) web.HandlerDecorator {
	return func(handler web.WebHandler) web.WebHandler {
		return func(ctx web.Context) (resp web.Response) {
			if !skip(ctx) {
				authHeader := readFromWebContext(ctx, "authorization")
				segs := strings.SplitN(authHeader, " ", 2)

				var authToken string
				if len(segs) >= 2 {
					if segs[0] != "Bearer" {
						return ctx.JSONError("auth failed: invalid auth type", http.StatusUnauthorized)
					}
					authToken = segs[1]
				} else {
					authToken = segs[0]
				}

				if err := cb(ctx, authToken); err != nil {
					return ctx.JSONError(fmt.Sprintf("auth failed: %s", err), http.StatusUnauthorized)
				}
			}

			return handler(ctx)
		}
	}
}
