package queue

import (
	"context"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/fromston"
	"github.com/mylxsw/aidea-server/internal/ai/leap"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *asynq.Client {
		return asynq.NewClient(asynq.RedisClientOpt{
			Addr:     conf.RedisAddr(),
			Password: conf.RedisPassword,
		})
	})

	binder.MustSingleton(NewQueue)
	binder.MustSingleton(NewPendingTaskManager)
}

func (Provider) Boot(app infra.Resolver) {
	app.MustResolve(func(
		manager *PendingTaskManager,
		leapClient *leap.LeapAI,
		fromstonClient *fromston.Fromston,
		dashscopeClient *dashscope.DashScope,
		up *uploader.Uploader,
		queue *Queue,
		rep *repo.Repository,
	) {
		// 注册异步 PendingTask 任务处理器
		manager.Register(TypeLeapAICompletion, leapAsyncJobProcesser(leapClient, up, rep))
		manager.Register(TypeFromStonCompletion, fromStonAsyncJobProcesser(queue, fromstonClient, up, rep))
		manager.Register(TypeDashscopeImageCompletion, dashscopeImageAsyncJobProcesser(queue, dashscopeClient, up, rep))
	})
}

const (
	TypeOpenAICompletion         = "openai:completion"
	TypeDeepAICompletion         = "deepai:completion"
	TypeStabilityAICompletion    = "stabilityai:completion"
	TypeLeapAICompletion         = "leapai:completion"
	TypeFromStonCompletion       = "fromston:completion"
	TypeImageGenCompletion       = "imagegen:completion"
	TypeGetimgAICompletion       = "getimgai:completion"
	TypeDashscopeImageCompletion = "dashscope-image:completion"
	TypeMailSend                 = "mail:send"
	TypeImageDownloader          = "image:downloader"
	TypeImageUpscale             = "image:upscale"
	TypeImageColorization        = "image:colorization"
	TypeSMSVerifyCodeSend        = "sms:verify_code:send"
	TypePayment                  = "payment"
	TypeSignup                   = "signup"
	TypeBindPhone                = "bind_phone"
	TypeGroupChat                = "group_chat"
)

func ResolveTaskType(category, model string) string {
	switch category {
	case "openai":
		return TypeOpenAICompletion
	case "deepai":
		return TypeDeepAICompletion
	case "stabilityai":
		return TypeStabilityAICompletion
	case "fromston":
		return TypeFromStonCompletion
	case "getimgai":
		return TypeGetimgAICompletion
	case "dashscope":
		return TypeDashscopeImageCompletion
	}

	return ""
}

// CompletionResult 任务完成后的结果
type CompletionResult struct {
	Resources   []string  `json:"resources"`
	OriginImage string    `json:"origin_image,omitempty"`
	ValidBefore time.Time `json:"valid_before,omitempty"`
}

// ErrorResult 任务失败后的结果
type ErrorResult struct {
	Errors []string `json:"errors"`
}

type EmptyResult struct{}

// TaskHandler 任务处理器
type TaskHandler func(context.Context, *asynq.Task) error

// TaskBuilder 任务构造器
type TaskBuilder func(payload any) *asynq.Task

// Payload 任务载荷接口
type Payload interface {
	GetTitle() string
	GetID() string
	GetUID() int64
	GetQuota() int64

	SetID(id string)
}

// Queue 任务队列
type Queue struct {
	client    *asynq.Client
	queueRepo *repo.QueueRepo
}

// NewQueue 创建一个任务队列
func NewQueue(client *asynq.Client, queueRepo *repo.QueueRepo) *Queue {
	return &Queue{client: client, queueRepo: queueRepo}
}

// Enqueue 将任务加入队列
func (q *Queue) Enqueue(payload Payload, taskBuilder TaskBuilder, opts ...asynq.Option) (string, error) {
	payload.SetID(must.Must(uuid.GenerateUUID()))

	task := taskBuilder(payload)
	info, err := q.client.Enqueue(task, opts...)
	if err != nil {
		return "", err
	}

	return payload.GetID(), q.queueRepo.Add(
		context.TODO(),
		payload.GetUID(),
		payload.GetID(),
		task.Type(),
		info.Queue,
		payload.GetTitle(),
		task.Payload(),
	)
}
