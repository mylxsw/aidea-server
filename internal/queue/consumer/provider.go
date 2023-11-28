package consumer

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/deepai"
	"github.com/mylxsw/aidea-server/pkg/ai/fromston"
	"github.com/mylxsw/aidea-server/pkg/ai/getimgai"
	"github.com/mylxsw/aidea-server/pkg/ai/leap"
	"github.com/mylxsw/aidea-server/pkg/ai/lepton"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"github.com/mylxsw/aidea-server/pkg/mail"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/sms"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *asynq.Server {
		return asynq.NewServer(
			asynq.RedisClientOpt{
				Addr:     conf.RedisAddr(),
				Password: conf.RedisPassword,
			},
			asynq.Config{
				Concurrency: conf.QueueWorkers,
				Queues: map[string]int{
					"mail":    conf.QueueWorkers / 5 * 1,
					"user":    conf.QueueWorkers / 5 * 1,
					"default": conf.QueueWorkers - conf.QueueWorkers/5*2,
					//"text":  conf.QueueWorkers / 3 * 2,
					//"image": conf.QueueWorkers - conf.QueueWorkers/3*2,
				},
				Logger: Logger{},
			},
		)
	})

	binder.MustSingleton(func(server *asynq.Server) *asynq.ServeMux {
		mux := asynq.NewServeMux()
		mux.Use(loggingMiddleware)
		return mux
	})
}

func loggingMiddleware(h asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		start := time.Now()
		log.Debugf("Start processing %q", t.Type())
		err := h.ProcessTask(ctx, t)
		if err != nil {
			log.Warningf("task process failed: %q, %v", t.Type(), err)
			// 失败后不再进行重试
			return asynq.SkipRetry
		}

		log.Debugf("finished processing %q: elapsed time = %v", t.Type(), time.Since(start))
		return nil
	})
}

func (p Provider) Boot(resolver infra.Resolver) {
	resolver.MustResolve(func(
		mux *asynq.ServeMux,
		openaiClient openai.Client,
		deepaiClient *deepai.DeepAI,
		stabaiClient *stabilityai.StabilityAI,
		getimgaiClient *getimgai.GetimgAI,
		leapClient *leap.LeapAI,
		translater youdao.Translater,
		uploader *uploader.Uploader,
		que *queue.Queue,
		mailer *mail.Sender,
		smsClient *sms.Client,
		ding *dingding.Dingding,
		fromstonClient *fromston.Fromston,
		dashscopeClient *dashscope.DashScope,
		rep *repo.Repository,
		ct chat.Chat,
		conf *config.Config,
		userSvc *service.UserService,
		dalleClient *openai.DalleImageClient,
		leptonClient *lepton.Lepton,
	) {
		log.Debugf("register all queue handlers")
		mux.HandleFunc(queue.TypeOpenAICompletion, queue.BuildOpenAICompletionHandler(openaiClient, rep))
		mux.HandleFunc(queue.TypeDeepAICompletion, queue.BuildDeepAICompletionHandler(deepaiClient, translater, uploader, rep, openaiClient))
		mux.HandleFunc(queue.TypeStabilityAICompletion, queue.BuildStabilityAICompletionHandler(stabaiClient, translater, uploader, rep, openaiClient))
		mux.HandleFunc(queue.TypeLeapAICompletion, queue.BuildLeapAICompletionHandler(leapClient, translater, uploader, rep, openaiClient))
		mux.HandleFunc(queue.TypeMailSend, queue.BuildMailSendHandler(mailer, rep))
		mux.HandleFunc(queue.TypeSMSVerifyCodeSend, queue.BuildSMSVerifyCodeSendHandler(smsClient, rep))
		mux.HandleFunc(queue.TypeSignup, queue.BuildSignupHandler(rep, mailer, ding))
		mux.HandleFunc(queue.TypePayment, queue.BuildPaymentHandler(rep, mailer, que, ding))
		mux.HandleFunc(queue.TypeBindPhone, queue.BuildBindPhoneHandler(rep, mailer))
		mux.HandleFunc(queue.TypeImageGenCompletion, queue.BuildImageCompletionHandler(leapClient, stabaiClient, deepaiClient, fromstonClient, dashscopeClient, getimgaiClient, translater, uploader, rep, openaiClient, dalleClient))
		mux.HandleFunc(queue.TypeFromStonCompletion, queue.BuildFromStonCompletionHandler(fromstonClient, uploader, rep))
		mux.HandleFunc(queue.TypeDashscopeImageCompletion, queue.BuildDashscopeImageCompletionHandler(dashscopeClient, uploader, rep, translater, openaiClient))
		mux.HandleFunc(queue.TypeGetimgAICompletion, queue.BuildGetimgAICompletionHandler(getimgaiClient, translater, uploader, rep, openaiClient))
		mux.HandleFunc(queue.TypeImageDownloader, queue.BuildImageDownloaderHandler(uploader, rep))
		mux.HandleFunc(queue.TypeImageUpscale, queue.BuildImageUpscaleHandler(deepaiClient, stabaiClient, uploader, rep))
		mux.HandleFunc(queue.TypeImageColorization, queue.BuildImageColorizationHandler(deepaiClient, uploader, rep))
		mux.HandleFunc(queue.TypeGroupChat, queue.BuildGroupChatHandler(conf, ct, rep, userSvc))
		mux.HandleFunc(queue.TypeDalleCompletion, queue.BuildDalleCompletionHandler(dalleClient, uploader, rep))
		mux.HandleFunc(queue.TypeArtisticTextCompletion, queue.BuildArtisticTextCompletionHandler(leptonClient, translater, uploader, rep, openaiClient))
	})
}

func (Provider) ShouldLoad(conf *config.Config) bool {
	return conf.QueueWorkers > 0
}

func (Provider) Daemon(ctx context.Context, resolver infra.Resolver) {
	resolver.MustResolve(func(conf *config.Config, server *asynq.Server, mux *asynq.ServeMux) error {
		log.Debugf("start queue consumer")
		return server.Run(mux)
	})
}
