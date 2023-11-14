package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/mylxsw/aidea-server/pkg/misc"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/eloquent/query"
	"gopkg.in/guregu/null.v3"
)

type QueueTaskStatus string

const (
	QueueTaskStatusPending QueueTaskStatus = "pending"
	QueueTaskStatusRunning QueueTaskStatus = "running"
	QueueTaskStatusSuccess QueueTaskStatus = "success"
	QueueTaskStatusFailed  QueueTaskStatus = "failed"
)

type QueueRepo struct {
	db   *sql.DB
	conf *config.Config
}

func NewQueueRepo(db *sql.DB, conf *config.Config) *QueueRepo {
	return &QueueRepo{db: db, conf: conf}
}

func (repo *QueueRepo) Add(ctx context.Context, uid int64, taskID, taskType, queueName string, title string, payload []byte) error {
	_, err := model2.NewQueueTasksModel(repo.db).Create(ctx, query.KV{
		model2.FieldQueueTasksTitle:     misc.SubString(title, 70),
		model2.FieldQueueTasksUid:       uid,
		model2.FieldQueueTasksTaskId:    taskID,
		model2.FieldQueueTasksTaskType:  taskType,
		model2.FieldQueueTasksQueueName: queueName,
		model2.FieldQueueTasksStatus:    QueueTaskStatusPending,
		model2.FieldQueueTasksPayload:   null.StringFrom(string(payload)),
	})

	return err
}

func (repo *QueueRepo) Update(ctx context.Context, taskID string, status QueueTaskStatus, result any) error {
	task, err := model2.NewQueueTasksModel(repo.db).First(ctx, query.Builder().Where(model2.FieldQueueTasksTaskId, taskID))
	if err != nil {
		return err
	}

	task.Status = null.StringFrom(string(status))

	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		task.Result = null.StringFrom(string(data))
	}

	return task.Save(ctx, model2.FieldQueueTasksStatus, model2.FieldQueueTasksResult)
}

func (repo *QueueRepo) Tasks(ctx context.Context, userID int64, taskType string) ([]model2.QueueTasks, error) {
	tasks, err := model2.NewQueueTasksModel(repo.db).Get(ctx, query.Builder().
		Where(model2.FieldQueueTasksTaskType, taskType).
		Where(model2.FieldQueueTasksUid, userID).
		OrderBy(model2.FieldQueueTasksCreatedAt, "DESC").Limit(10))
	if err != nil {
		return nil, err
	}

	res := make([]model2.QueueTasks, len(tasks))
	for idx, task := range tasks {
		res[idx] = task.ToQueueTasks()
	}

	return res, nil
}

func (repo *QueueRepo) Task(ctx context.Context, taskID string) (*model2.QueueTasks, error) {
	task, err := model2.NewQueueTasksModel(repo.db).First(ctx, query.Builder().Where(model2.FieldQueueTasksTaskId, taskID))
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}
		return nil, err
	}

	res := task.ToQueueTasks()
	return &res, nil
}

func (repo *QueueRepo) Remove(ctx context.Context, taskID string) error {
	_, err := model2.NewQueueTasksModel(repo.db).Delete(ctx, query.Builder().Where(model2.FieldQueueTasksTaskId, taskID))
	return err
}

func (repo *QueueRepo) RemoveQueueTasks(ctx context.Context, before time.Time) error {
	q := query.Builder().
		Where(model2.FieldQueueTasksCreatedAt, "<", before).
		Where(model2.FieldQueueTasksStatus, QueueTaskStatusSuccess)
	_, err := model2.NewQueueTasksModel(repo.db).Delete(ctx, q)
	return err
}

type PendingTask struct {
	TaskType      string            `json:"task_type"`
	TaskID        string            `json:"task_id"`
	NextExecuteAt time.Time         `json:"next_execute_at"`
	DeadlineAt    time.Time         `json:"deadline_at"`
	Payload       any               `json:"payload"`
	Status        PendingTaskStatus `json:"status"`
}

type PendingTaskStatus int

const (
	PendingTaskStatusProcessing PendingTaskStatus = 1
	PendingTaskStatusSuccess    PendingTaskStatus = 2
	PendingTaskStatusFailed     PendingTaskStatus = 3
	PendingTaskStatusTimeout    PendingTaskStatus = 4
)

func (repo *QueueRepo) CreatePendingTask(ctx context.Context, task *PendingTask) error {
	data, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}

	kv := query.KV{
		model2.FieldQueueTasksPendingTaskType:      task.TaskType,
		model2.FieldQueueTasksPendingTaskId:        task.TaskID,
		model2.FieldQueueTasksPendingPayload:       string(data),
		model2.FieldQueueTasksPendingStatus:        int(task.Status),
		model2.FieldQueueTasksPendingNextExecuteAt: task.NextExecuteAt,
	}

	if !task.DeadlineAt.IsZero() {
		kv[model2.FieldQueueTasksPendingDeadlineAt] = task.DeadlineAt
	} else {
		kv[model2.FieldQueueTasksPendingDeadlineAt] = time.Now().Add(24 * time.Hour)
	}

	_, err = model2.NewQueueTasksPendingModel(repo.db).Create(ctx, kv)
	return err
}

func (repo *QueueRepo) PendingTasks(ctx context.Context) ([]model2.QueueTasksPending, error) {
	q := query.Builder().
		Where(model2.FieldQueueTasksPendingStatus, PendingTaskStatusProcessing).
		Where(model2.FieldQueueTasksPendingNextExecuteAt, "<=", time.Now())

	tasks, err := model2.NewQueueTasksPendingModel(repo.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	res := make([]model2.QueueTasksPending, len(tasks))
	for idx, task := range tasks {
		res[idx] = task.ToQueueTasksPending()
	}

	return res, nil
}

type PendingTaskUpdate struct {
	NextExecuteAt time.Time         `json:"next_execute_at"`
	ExecuteTimes  int64             `json:"execute_times"`
	Status        PendingTaskStatus `json:"status"`
}

func (repo *QueueRepo) UpdatePendingTask(ctx context.Context, id int64, task *PendingTaskUpdate) error {

	data := model2.QueueTasksPendingN{
		Status: null.IntFrom(int64(task.Status)),
	}

	if !task.NextExecuteAt.IsZero() {
		data.NextExecuteAt = null.TimeFrom(task.NextExecuteAt)
	}

	if task.ExecuteTimes > 0 {
		data.ExecuteTimes = null.IntFrom(task.ExecuteTimes)
	}

	q := query.Builder().Where(model2.FieldQueueTasksPendingId, id)
	_, err := model2.NewQueueTasksPendingModel(repo.db).Update(ctx, q, data)

	return err
}

// RemovePendingTasks 删除已经执行成功的任务
func (repo *QueueRepo) RemovePendingTasks(ctx context.Context, before time.Time) error {
	q := query.Builder().
		Where(model2.FieldQueueTasksCreatedAt, "<", before).
		Where(model2.FieldQueueTasksPendingStatus, PendingTaskStatusSuccess)
	_, err := model2.NewQueueTasksPendingModel(repo.db).Delete(ctx, q)
	return err
}
