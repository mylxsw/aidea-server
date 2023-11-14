package queue

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"sync"
	"time"

	"github.com/mylxsw/asteria/log"
)

type PendingTaskHandler func(task *model.QueueTasksPending) (*repo.PendingTaskUpdate, error)

type PendingTaskManager struct {
	lock     sync.RWMutex
	handlers map[string]PendingTaskHandler
}

func NewPendingTaskManager() *PendingTaskManager {
	return &PendingTaskManager{handlers: make(map[string]PendingTaskHandler)}
}

func (m *PendingTaskManager) Register(taskType string, handler PendingTaskHandler) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.handlers[taskType] = handler
}

func (m *PendingTaskManager) Get(taskType string) (PendingTaskHandler, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	handler, found := m.handlers[taskType]
	return handler, found
}

func (m *PendingTaskManager) Remove(taskType string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.handlers, taskType)
}

func PendingTaskJob(ctx context.Context, queueRepo *repo.QueueRepo, manager *PendingTaskManager) error {
	tasks, err := queueRepo.PendingTasks(ctx)
	if err != nil {
		log.Errorf("查询待处理任务失败: %v", err)
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go func(task model.QueueTasksPending) {
			defer wg.Done()
			if err := handlePendingTask(ctx, queueRepo, manager, &task); err != nil {
				log.Errorf("处理待处理任务失败: %v", err)
			}
		}(task)
	}

	wg.Wait()

	return nil
}

func handlePendingTask(ctx context.Context, queueRepo *repo.QueueRepo, manager *PendingTaskManager, task *model.QueueTasksPending) error {
	defer func() {
		if err := recover(); err != nil {
			log.WithFields(log.Fields{"task_id": task.Id}).Errorf("处理待处理任务失败: %v", err)

			// 任务处理失败，设置为失败状态
			if err := queueRepo.UpdatePendingTask(ctx, task.Id, &repo.PendingTaskUpdate{Status: repo.PendingTaskStatusFailed}); err != nil {
				log.WithFields(log.Fields{"task_id": task.Id}).Errorf("更新待处理任务状态失败: %v", err)
			}
		}
	}()

	if !task.DeadlineAt.IsZero() && task.DeadlineAt.Before(time.Now()) {
		// 任务已经超时，设置超时状态
		if err := queueRepo.UpdatePendingTask(ctx, task.Id, &repo.PendingTaskUpdate{Status: repo.PendingTaskStatusTimeout}); err != nil {
			log.WithFields(log.Fields{"task_id": task.Id}).Errorf("更新待处理任务状态失败: %v", err)
			return err
		}

		return nil
	}

	// 查找任务处理器
	handler, found := manager.Get(task.TaskType)
	if !found {
		panic(errors.New("未找到待处理任务的处理器"))
	}

	// 执行任务处理器
	u, err := handler(task)
	if err != nil {
		panic(err)
	}

	// 更新任务状态
	return queueRepo.UpdatePendingTask(ctx, task.Id, u)
}
