package queue

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/fromston"
	"github.com/mylxsw/aidea-server/pkg/ai/leap"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
	"github.com/redis/go-redis/v9"
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
		rep *repo2.Repository,
		userSvc *service.UserService,
		rds *redis.Client,
	) {
		// 注册异步 PendingTask 任务处理器
		manager.Register(TypeLeapAICompletion, leapAsyncJobProcesser(leapClient, up, rep))
		manager.Register(TypeFromStonCompletion, fromStonAsyncJobProcesser(queue, fromstonClient, up, rep))
		manager.Register(TypeDashscopeImageCompletion, dashscopeImageAsyncJobProcesser(queue, dashscopeClient, up, rep))

		// 注册创作岛更新后，自动释放冻结的智慧果任务
		rep.Creative.RegisterRecordStatusUpdateCallback(func(taskID string, userID int64, status repo2.CreativeStatus) {
			key := fmt.Sprintf("creative-island:%d:task:%s:quota-freeze", userID, taskID)
			if status == repo2.CreativeStatusSuccess || status == repo2.CreativeStatusFailed {
				freezedValue, err := rds.Get(context.TODO(), key).Int64()
				if err != nil {
					log.F(log.M{"task_id": taskID, "user_id": userID, "status": status}).Errorf("获取创作岛任务冻结的智慧果数量失败：%s", err)
					return
				}

				if freezedValue > 0 {
					if err := userSvc.UnfreezeUserQuota(context.TODO(), userID, freezedValue); err != nil {
						log.F(log.M{"task_id": taskID, "user_id": userID, "status": status}).Errorf("释放创作岛任务冻结的智慧果失败：%s", err)
						return
					}
				}
			}
		})
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
	TypeDalleCompletion          = "dalle:completion"
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
	TypeArtisticTextCompletion   = "artistic_text:completion"
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
	case "dalle":
		return TypeDalleCompletion
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
	queueRepo *repo2.QueueRepo
}

// NewQueue 创建一个任务队列
func NewQueue(client *asynq.Client, queueRepo *repo2.QueueRepo) *Queue {
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
