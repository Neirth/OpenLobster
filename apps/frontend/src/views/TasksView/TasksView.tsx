// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from 'solid-js';
import { For, Show, createEffect } from 'solid-js';
import { createSignal } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useTasks } from '@openlobster/ui/hooks';
import type { Task } from '@openlobster/ui/types';
import { ADD_TASK_MUTATION, REMOVE_TASK_MUTATION, TOGGLE_TASK_MUTATION, UPDATE_TASK_MUTATION } from '@openlobster/ui/graphql/mutations';
import { client } from '../../graphql/client';
import AppShell from '../../components/AppShell';
import Modal from '../../components/Modal';
import { t } from '../../App';
import './TasksView.css';

const TasksView: Component = () => {
  const tasks = useTasks(client);
  const [showNewTaskForm, setShowNewTaskForm] = createSignal(false);
  const [taskToDelete, setTaskToDelete] = createSignal<string | null>(null);
  const [taskToEdit, setTaskToEdit] = createSignal<Task | null>(null);
  const [editPrompt, setEditPrompt] = createSignal('');
  const [editSchedule, setEditSchedule] = createSignal('');
  const [editTaskType, setEditTaskType] = createSignal<'one-shot' | 'cyclic'>('one-shot');
  const [newPrompt, setNewPrompt] = createSignal('');
  const [newSchedule, setNewSchedule] = createSignal('');
  const [newTaskType, setNewTaskType] = createSignal<'one-shot' | 'cyclic'>('one-shot');

  const queryClient = useQueryClient();

  // Optimistic local state for task enabled toggles.
  // Keeps UI responsive without waiting for query refetch.
  const [enabledMap, setEnabledMap] = createSignal<Record<string, boolean>>({});

  createEffect(() => {
    const data = tasks.data;
    if (!data) return;
    const next: Record<string, boolean> = {};
    data.forEach((t) => { next[t.id] = t.enabled; });
    setEnabledMap(next);
  });

  const handleToggle = (taskId: string, newEnabled: boolean) => {
    setEnabledMap((prev) => ({ ...prev, [taskId]: newEnabled }));
    toggleTask.mutate({ id: taskId, enabled: newEnabled });
  };

  const removeTask = createMutation(() => ({
    mutationFn: (vars: { taskId: string }) => client.request(REMOVE_TASK_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['tasks'] });
      setTaskToDelete(null);
    },
  }));

  const addTask = createMutation(() => ({
    mutationFn: (vars: { prompt: string; schedule: string }) =>
      client.request(ADD_TASK_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['tasks'] });
      setShowNewTaskForm(false);
      setNewPrompt('');
      setNewSchedule('');
      setNewTaskType('one-shot');
    },
  }));

  const handleNewTaskSubmit = (e: Event) => {
    e.preventDefault();
    if (!newPrompt()) return;
    // cyclic: schedule is the cron string; one-shot: schedule is ISO datetime or empty
    addTask.mutate({ prompt: newPrompt(), schedule: newSchedule() });
  };

  const updateTask = createMutation(() => ({
    mutationFn: (vars: { id: string; prompt: string; schedule: string }) =>
      client.request(UPDATE_TASK_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['tasks'] });
      setTaskToEdit(null);
    },
  }));

  const toggleTask = createMutation(() => ({
    mutationFn: (vars: { id: string; enabled: boolean }) =>
      client.request(TOGGLE_TASK_MUTATION, vars),
    onError: (_err: unknown, vars: { id: string; enabled: boolean }) => {
      // Revert optimistic update on failure.
      setEnabledMap((prev) => ({ ...prev, [vars.id]: !vars.enabled }));
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['tasks'] });
    },
  }));

  const openEditModal = (task: Task) => {
    setEditPrompt(task.prompt);
    setEditSchedule(task.schedule ?? '');
    setEditTaskType(task.taskType ?? (task.schedule && !task.schedule.includes('T') ? 'cyclic' : 'one-shot'));
    setTaskToEdit(task);
  };

  const handleEditSubmit = (e: Event) => {
    e.preventDefault();
    const task = taskToEdit();
    if (!task) return;
    updateTask.mutate({ id: task.id, prompt: editPrompt(), schedule: editSchedule() });
  };

  const formatSchedule = (schedule: string | null | undefined): string => {
    if (!schedule) return '—';
    // ISO 8601 datetime: shorten to readable format
    const d = new Date(schedule);
    if (!isNaN(d.getTime()) && schedule.includes('T')) {
      return d.toLocaleString(undefined, {
        month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit', hour12: false,
      });
    }
    return schedule;
  };

  const formatNextRun = (iso: string | null | undefined): string => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    return d.toLocaleString(undefined, {
      month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit', hour12: false,
    });
  };

  return (
    <AppShell activeTab="tasks">
      <div class="tasks-view">
        <Show when={tasks.data && tasks.data.length > 0}>
          <div class="tasks-header">
            <div class="tasks-header__text">
              <h1>{t('tasks.scheduledTasks')}</h1>
              <p>{t('tasks.managingHint')}</p>
            </div>
            <button class="new-task-btn" onClick={() => setShowNewTaskForm(!showNewTaskForm())}>
              + {t('tasks.newTask')}
            </button>
          </div>
        </Show>

        <Show when={!tasks.isLoading && (!tasks.data || tasks.data.length === 0)}>
          <div class="tasks-empty">
            <span class="material-symbols-outlined tasks-empty-icon">schedule</span>
            <p class="tasks-empty-title">{t('tasks.noTasks')}</p>
            <p class="tasks-empty-hint">{t('tasks.noTasksHint')}</p>
            <button class="btn btn-md btn-primary" onClick={() => setShowNewTaskForm(true)}>
              + {t('tasks.newTask')}
            </button>
          </div>
        </Show>

        <Show when={tasks.data && tasks.data.length > 0}>
        <table class="tasks-table">
          <colgroup>
            <col style="width: 40px" />
            <col style="width: auto" />
            <col style="width: 160px" />
            <col style="width: 100px" />
            <col style="width: 90px" />
            <col style="width: 72px" />
            <col style="width: 130px" />
            <col style="width: 80px" />
          </colgroup>
          <thead>
            <tr>
              <th>#</th>
              <th>{t('tasks.colName')}</th>
              <th>{t('tasks.colSchedule')}</th>
              <th>{t('tasks.colType')}</th>
              <th>{t('tasks.colStatus')}</th>
              <th>{t('tasks.colEnabled')}</th>
              <th>{t('tasks.colNextRun')}</th>
              <th>{t('tasks.colActions')}</th>
            </tr>
          </thead>
          <tbody>
            <For each={tasks.data}>
              {(task, index) => (
                <tr>
                  <td class="task-id">#{index() + 1}</td>
                  <td class="task-name">{task.prompt}</td>
                  <td class="task-schedule">{formatSchedule(task.schedule)}</td>
                  <td class="task-type">
                    <span class={`task-type-badge ${task.taskType === 'cyclic' ? 'task-type-badge--cyclic' : 'task-type-badge--oneshot'}`}>
                      {task.taskType === 'cyclic' ? t('tasks.typeCyclic') : t('tasks.typeOneShot')}
                    </span>
                  </td>
                  <td class="task-status">{task.status}</td>
                  <td class="task-enabled">
                    <label class="toggle-switch">
                      <input
                        type="checkbox"
                        checked={enabledMap()[task.id] ?? task.enabled}
                        onChange={(e) => handleToggle(task.id, e.currentTarget.checked)}
                      />
                      <span class="toggle-slider" />
                    </label>
                  </td>
                  <td class="task-next-run">{formatNextRun(task.nextRunAt)}</td>
                  <td class="task-actions">
                    <div class="task-actions__inner">
                      <button
                        class="task-edit-btn"
                        onClick={() => openEditModal(task)}
                        aria-label={t('tasks.editTaskTitle')}
                      >
                        <span class="material-symbols-outlined">edit</span>
                      </button>
                      <button
                        class="task-delete-btn"
                        onClick={() => setTaskToDelete(task.id)}
                        aria-label={t('tasks.deleteTaskTitle')}
                      >
                        <span class="material-symbols-outlined">delete</span>
                      </button>
                    </div>
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
        </Show>

        {/* New Task Modal */}
        <Modal
          isOpen={showNewTaskForm()}
          onClose={() => setShowNewTaskForm(false)}
          title={t('tasks.newTask')}
        >
          <form class="modal-form" onSubmit={handleNewTaskSubmit}>
            <div class="form-group">
              <label>{t('tasks.prompt')}</label>
              <textarea
                placeholder={t('tasks.promptPlaceholder')}
                rows={4}
                value={newPrompt()}
                onInput={(e) => setNewPrompt(e.currentTarget.value)}
                required
              />
            </div>
            <div class="form-group">
              <label>{t('tasks.taskType')}</label>
              <div class="task-type-selector">
                <button
                  type="button"
                  class={newTaskType() === 'one-shot' ? 'active' : ''}
                  onClick={() => { setNewTaskType('one-shot'); setNewSchedule(''); }}
                >
                  {t('tasks.typeOneShot')}
                </button>
                <button
                  type="button"
                  class={newTaskType() === 'cyclic' ? 'active' : ''}
                  onClick={() => { setNewTaskType('cyclic'); setNewSchedule(''); }}
                >
                  {t('tasks.typeCyclic')}
                </button>
              </div>
            </div>
            <div class="form-group">
              <label>{t('tasks.schedule')}</label>
              {newTaskType() === 'cyclic' ? (
                <input
                  type="text"
                  placeholder="0 8 * * *"
                  value={newSchedule()}
                  onInput={(e) => setNewSchedule(e.currentTarget.value)}
                  required
                />
              ) : (
                <input
                  type="text"
                  placeholder="2026-04-01T09:00:00Z"
                  value={newSchedule()}
                  onInput={(e) => setNewSchedule(e.currentTarget.value)}
                />
              )}
              <small class="form-hint">
                {newTaskType() === 'cyclic'
                  ? t('tasks.scheduleHintCron')
                  : t('tasks.scheduleHintOneShot')}
              </small>
            </div>
            <div class="form-actions">
              <button type="button" class="btn btn-md btn-secondary" onClick={() => setShowNewTaskForm(false)}>
                {t('common.cancel')}
              </button>
              <button
                type="submit"
                class="btn btn-md btn-primary"
                disabled={addTask.isPending}
              >
                {addTask.isPending ? '…' : t('tasks.createTask')}
              </button>
            </div>
          </form>
        </Modal>

        {/* Edit Task Modal */}
        <Modal
          isOpen={taskToEdit() !== null}
          onClose={() => setTaskToEdit(null)}
          title={t('tasks.editTask')}
        >
          <form class="modal-form" onSubmit={handleEditSubmit}>
            <div class="form-group">
              <label>{t('tasks.prompt')}</label>
              <textarea
                placeholder={t('tasks.promptPlaceholder')}
                rows={4}
                value={editPrompt()}
                onInput={(e) => setEditPrompt(e.currentTarget.value)}
                required
              />
            </div>
            <div class="form-group">
              <label>{t('tasks.taskType')}</label>
              <div class="task-type-selector">
                <button
                  type="button"
                  class={editTaskType() === 'one-shot' ? 'active' : ''}
                  onClick={() => { setEditTaskType('one-shot'); setEditSchedule(''); }}
                >
                  {t('tasks.typeOneShot')}
                </button>
                <button
                  type="button"
                  class={editTaskType() === 'cyclic' ? 'active' : ''}
                  onClick={() => { setEditTaskType('cyclic'); setEditSchedule(''); }}
                >
                  {t('tasks.typeCyclic')}
                </button>
              </div>
            </div>
            <div class="form-group">
              <label>{t('tasks.schedule')}</label>
              {editTaskType() === 'cyclic' ? (
                <input
                  type="text"
                  placeholder="0 8 * * *"
                  value={editSchedule()}
                  onInput={(e) => setEditSchedule(e.currentTarget.value)}
                  required
                />
              ) : (
                <input
                  type="text"
                  placeholder="2026-04-01T09:00:00Z"
                  value={editSchedule()}
                  onInput={(e) => setEditSchedule(e.currentTarget.value)}
                />
              )}
              <small class="form-hint">
                {editTaskType() === 'cyclic'
                  ? t('tasks.scheduleHintCron')
                  : t('tasks.scheduleHintOneShot')}
              </small>
            </div>
            <div class="form-actions">
              <button
                type="button"
                class="btn btn-md btn-secondary"
                onClick={() => setTaskToEdit(null)}
              >
                {t('common.cancel')}
              </button>
              <button
                type="submit"
                class="btn btn-md btn-primary"
                disabled={updateTask.isPending}
              >
                {updateTask.isPending ? '…' : t('common.save')}
              </button>
            </div>
          </form>
        </Modal>

        {/* Delete Task Confirmation Modal */}
        <Modal
          isOpen={taskToDelete() !== null}
          onClose={() => setTaskToDelete(null)}
          title={t('tasks.deleteTask')}
        >
          <div class="modal-form">
            <p class="modal-text">{t('tasks.deleteConfirmation')}</p>
            <div class="form-actions">
              <button class="btn btn-md btn-secondary" onClick={() => setTaskToDelete(null)}>
                {t('common.cancel')}
              </button>
              <button
                class="btn btn-md btn-danger"
                onClick={() => {
                  const id = taskToDelete();
                  if (id) removeTask.mutate({ taskId: id });
                }}
                disabled={removeTask.isPending}
              >
                {t('common.delete')}
              </button>
            </div>
          </div>
        </Modal>
      </div>
    </AppShell>
  );
};

export default TasksView;
