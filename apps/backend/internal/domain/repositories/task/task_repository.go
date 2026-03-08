// Copyright (c) OpenLobster contributors. See LICENSE for details.

package task

import (
	"context"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

// TaskRepository extends ports.TaskRepositoryPort with dashboard convenience methods.
type TaskRepository interface {
	ports.TaskRepositoryPort
	Done(ctx context.Context, id string) error
	List() ([]domainmodels.Task, error)
}

type repository struct{ db *gorm.DB }

// NewTaskRepository returns a TaskRepository backed by the given *gorm.DB.
func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &repository{db: db}
}

func (r *repository) GetPending(ctx context.Context) ([]domainmodels.Task, error) {
	var models []domainmodels.TaskModel
	if err := r.db.WithContext(ctx).Where("status = 'pending' AND enabled = ?", true).Find(&models).Error; err != nil {
		return nil, err
	}
	return taskModelsToEntities(models), nil
}

func (r *repository) Add(ctx context.Context, task *domainmodels.Task) error {
	m := domainmodels.TaskModel{
		ID:       task.ID,
		Prompt:   task.Prompt,
		Schedule: task.Schedule,
		TaskType: task.TaskType,
		Status:   task.Status,
		Enabled:  task.Enabled,
		AddedAt:  task.AddedAt,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *repository) MarkDone(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domainmodels.TaskModel{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "done", "finished_at": &now}).Error
}

func (r *repository) Update(ctx context.Context, task *domainmodels.Task) error {
	return r.db.WithContext(ctx).Model(&domainmodels.TaskModel{}).Where("id = ?", task.ID).
		Updates(map[string]interface{}{"prompt": task.Prompt, "schedule": task.Schedule, "task_type": task.TaskType}).Error
}

func (r *repository) SetEnabled(ctx context.Context, id string, enabled bool) error {
	return r.db.WithContext(ctx).Model(&domainmodels.TaskModel{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (r *repository) ListAll(ctx context.Context) ([]domainmodels.Task, error) {
	var models []domainmodels.TaskModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	return taskModelsToEntities(models), nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domainmodels.TaskModel{}, "id = ?", id).Error
}

func (r *repository) Done(ctx context.Context, id string) error {
	return r.MarkDone(ctx, id)
}

func (r *repository) List() ([]domainmodels.Task, error) {
	return r.ListAll(context.Background())
}

// DashboardTaskRepository wraps TaskRepository for context-free use.
type DashboardTaskRepository struct {
	inner TaskRepository
}

// NewDashboardTaskRepository wraps a TaskRepository for use in the GraphQL dashboard.
func NewDashboardTaskRepository(repo TaskRepository) *DashboardTaskRepository {
	return &DashboardTaskRepository{inner: repo}
}

func (r *DashboardTaskRepository) Add(task *domainmodels.Task) error {
	return r.inner.Add(context.Background(), task)
}

func (r *DashboardTaskRepository) Done(id string) error {
	return r.inner.Done(context.Background(), id)
}

func (r *DashboardTaskRepository) List() ([]domainmodels.Task, error) {
	return r.inner.List()
}

func taskModelsToEntities(models []domainmodels.TaskModel) []domainmodels.Task {
	tasks := make([]domainmodels.Task, len(models))
	for i, m := range models {
		tasks[i] = domainmodels.Task{
			ID:         m.ID,
			Prompt:     m.Prompt,
			Schedule:   m.Schedule,
			TaskType:   m.TaskType,
			Status:     m.Status,
			Enabled:    m.Enabled,
			AddedAt:    m.AddedAt,
			FinishedAt: m.FinishedAt,
		}
	}
	return tasks
}
