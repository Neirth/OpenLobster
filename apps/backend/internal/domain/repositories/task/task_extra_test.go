// Copyright (c) OpenLobster contributors. See LICENSE for details.

package task

import (
	"context"
	"testing"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_SetStatus(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("status task", "")
	require.NoError(t, repo.Add(ctx, task))

	require.NoError(t, repo.SetStatus(ctx, task.ID, "running"))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "running", all[0].Status)
}

func TestRepository_SetStatus_EmptyStatus(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("empty status", "")
	require.NoError(t, repo.Add(ctx, task))

	// Empty status should be a no-op.
	require.NoError(t, repo.SetStatus(ctx, task.ID, ""))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "pending", all[0].Status) // unchanged
}

func TestRepository_SetStatus_Whitespace(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("ws status", "")
	require.NoError(t, repo.Add(ctx, task))

	// Whitespace-only status should be a no-op.
	require.NoError(t, repo.SetStatus(ctx, task.ID, "  "))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, "pending", all[0].Status)
}

func TestRepository_List(t *testing.T) {
	db, _ := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("list task", "0 * * * *")
	require.NoError(t, repo.Add(context.Background(), task))

	list, err := repo.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "list task", list[0].Prompt)
}

func TestRepository_GetPending_OnlyEnabled(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	enabled := domainmodels.NewTask("enabled", "")
	disabled := domainmodels.NewTask("disabled", "")
	require.NoError(t, repo.Add(ctx, enabled))
	require.NoError(t, repo.Add(ctx, disabled))
	require.NoError(t, repo.SetEnabled(ctx, disabled.ID, false))

	pending, err := repo.GetPending(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "enabled", pending[0].Prompt)
}

func TestRepository_Update_TaskType(t *testing.T) {
	db, ctx := setupTaskDB(t)
	repo := NewTaskRepository(db.GormDB())

	task := domainmodels.NewTask("original prompt", "")
	require.NoError(t, repo.Add(ctx, task))

	task.TaskType = "cyclic"
	task.Schedule = "*/5 * * * *"
	task.Prompt = "updated"
	require.NoError(t, repo.Update(ctx, task))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "cyclic", all[0].TaskType)
	assert.Equal(t, "*/5 * * * *", all[0].Schedule)
}
