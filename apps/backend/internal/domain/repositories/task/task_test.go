package task

import (
	"context"
	"testing"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTaskDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db, context.Background()
}

func TestNewTaskRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewTaskRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_AddAndListAll(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("Test task", "")
	require.NoError(t, repo.Add(ctx, task))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "Test task", all[0].Prompt)
	assert.Equal(t, "pending", all[0].Status)
}

func TestNewDashboardTaskRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	inner := NewTaskRepository(db.GormDB())
	wrapper := NewDashboardTaskRepository(inner)
	require.NotNil(t, wrapper)
}

func TestRepository_GetPending(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("pending task", "")
	require.NoError(t, repo.Add(ctx, task))

	pending, err := repo.GetPending(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "pending task", pending[0].Prompt)
	assert.True(t, pending[0].Enabled)
}

func TestRepository_MarkDone(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("done task", "")
	require.NoError(t, repo.Add(ctx, task))

	require.NoError(t, repo.MarkDone(ctx, task.ID))
	all, _ := repo.ListAll(ctx)
	assert.Equal(t, "done", all[0].Status)
	assert.NotNil(t, all[0].FinishedAt)
}

func TestRepository_Done(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("done via Done", "")
	require.NoError(t, repo.Add(ctx, task))

	require.NoError(t, repo.Done(ctx, task.ID))
	all, _ := repo.ListAll(ctx)
	assert.Equal(t, "done", all[0].Status)
}

func TestRepository_Update(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("original", "")
	require.NoError(t, repo.Add(ctx, task))

	task.Prompt = "updated prompt"
	task.Schedule = "0 9 * * *"
	require.NoError(t, repo.Update(ctx, task))

	all, _ := repo.ListAll(ctx)
	assert.Equal(t, "updated prompt", all[0].Prompt)
	assert.Equal(t, "0 9 * * *", all[0].Schedule)
}

func TestRepository_SetEnabled(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("enable test", "")
	require.NoError(t, repo.Add(ctx, task))

	require.NoError(t, repo.SetEnabled(ctx, task.ID, false))
	pending, _ := repo.GetPending(ctx)
	assert.Len(t, pending, 0)
}

func TestRepository_Delete(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("to delete", "")
	require.NoError(t, repo.Add(ctx, task))

	require.NoError(t, repo.Delete(ctx, task.ID))
	all, _ := repo.ListAll(ctx)
	assert.Len(t, all, 0)
}

func TestDashboardTaskRepository_AddAndList(t *testing.T) {
	db, _ := setupTaskDB(t)
	inner := NewTaskRepository(db.GormDB())
	wrapper := NewDashboardTaskRepository(inner)

	task := domainmodels.NewTask("dashboard task", "")
	require.NoError(t, wrapper.Add(task))

	list, err := wrapper.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "dashboard task", list[0].Prompt)
}

func TestDashboardTaskRepository_Done(t *testing.T) {
	db, ctx := setupTaskDB(t)
	inner := NewTaskRepository(db.GormDB())
	wrapper := NewDashboardTaskRepository(inner)

	task := domainmodels.NewTask("dashboard done", "")
	require.NoError(t, inner.Add(ctx, task))

	require.NoError(t, wrapper.Done(task.ID))
	list, _ := wrapper.List()
	assert.Equal(t, "done", list[0].Status)
}
