package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/repo/model"
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
	_, err := model.NewQueueTasksModel(repo.db).Create(ctx, query.KV{
		model.FieldQueueTasksTitle:     helper.SubString(title, 70),
		model.FieldQueueTasksUid:       uid,
		model.FieldQueueTasksTaskId:    taskID,
		model.FieldQueueTasksTaskType:  taskType,
		model.FieldQueueTasksQueueName: queueName,
		model.FieldQueueTasksStatus:    QueueTaskStatusPending,
		model.FieldQueueTasksPayload:   null.StringFrom(string(payload)),
	})

	return err
}

func (repo *QueueRepo) Update(ctx context.Context, taskID string, status QueueTaskStatus, result any) error {
	task, err := model.NewQueueTasksModel(repo.db).First(ctx, query.Builder().Where(model.FieldQueueTasksTaskId, taskID))
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

	return task.Save(ctx, model.FieldQueueTasksStatus, model.FieldQueueTasksResult)
}

func (repo *QueueRepo) Tasks(ctx context.Context, userID int64, taskType string) ([]model.QueueTasks, error) {
	tasks, err := model.NewQueueTasksModel(repo.db).Get(ctx, query.Builder().
		Where(model.FieldQueueTasksTaskType, taskType).
		Where(model.FieldQueueTasksUid, userID).
		OrderBy(model.FieldQueueTasksCreatedAt, "DESC").Limit(10))
	if err != nil {
		return nil, err
	}

	res := make([]model.QueueTasks, len(tasks))
	for idx, task := range tasks {
		res[idx] = task.ToQueueTasks()
	}

	return res, nil
}

func (repo *QueueRepo) Task(ctx context.Context, taskID string) (*model.QueueTasks, error) {
	task, err := model.NewQueueTasksModel(repo.db).First(ctx, query.Builder().Where(model.FieldQueueTasksTaskId, taskID))
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
	_, err := model.NewQueueTasksModel(repo.db).Delete(ctx, query.Builder().Where(model.FieldQueueTasksTaskId, taskID))
	return err
}

func (repo *QueueRepo) RemoveQueueTasks(ctx context.Context, before time.Time) error {
	q := query.Builder().
		Where(model.FieldQueueTasksCreatedAt, "<", before).
		Where(model.FieldQueueTasksStatus, QueueTaskStatusSuccess)
	_, err := model.NewQueueTasksModel(repo.db).Delete(ctx, q)
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
		model.FieldQueueTasksPendingTaskType:      task.TaskType,
		model.FieldQueueTasksPendingTaskId:        task.TaskID,
		model.FieldQueueTasksPendingPayload:       string(data),
		model.FieldQueueTasksPendingStatus:        int(task.Status),
		model.FieldQueueTasksPendingNextExecuteAt: task.NextExecuteAt,
	}

	if !task.DeadlineAt.IsZero() {
		kv[model.FieldQueueTasksPendingDeadlineAt] = task.DeadlineAt
	} else {
		kv[model.FieldQueueTasksPendingDeadlineAt] = time.Now().Add(24 * time.Hour)
	}

	_, err = model.NewQueueTasksPendingModel(repo.db).Create(ctx, kv)
	return err
}

func (repo *QueueRepo) PendingTasks(ctx context.Context) ([]model.QueueTasksPending, error) {
	q := query.Builder().
		Where(model.FieldQueueTasksPendingStatus, PendingTaskStatusProcessing).
		Where(model.FieldQueueTasksPendingNextExecuteAt, "<=", time.Now())

	tasks, err := model.NewQueueTasksPendingModel(repo.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	res := make([]model.QueueTasksPending, len(tasks))
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

	data := model.QueueTasksPendingN{
		Status: null.IntFrom(int64(task.Status)),
	}

	if !task.NextExecuteAt.IsZero() {
		data.NextExecuteAt = null.TimeFrom(task.NextExecuteAt)
	}

	if task.ExecuteTimes > 0 {
		data.ExecuteTimes = null.IntFrom(task.ExecuteTimes)
	}

	q := query.Builder().Where(model.FieldQueueTasksPendingId, id)
	_, err := model.NewQueueTasksPendingModel(repo.db).Update(ctx, q, data)

	return err
}

// RemovePendingTasks 删除已经执行成功的任务
func (repo *QueueRepo) RemovePendingTasks(ctx context.Context, before time.Time) error {
	q := query.Builder().
		Where(model.FieldQueueTasksCreatedAt, "<", before).
		Where(model.FieldQueueTasksPendingStatus, PendingTaskStatusSuccess)
	_, err := model.NewQueueTasksPendingModel(repo.db).Delete(ctx, q)
	return err
}
