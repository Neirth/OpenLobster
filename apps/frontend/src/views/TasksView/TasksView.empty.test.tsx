// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * TasksView empty-state tests.
 *
 * Uses a separate module file so vi.mock hoisting can override the
 * useTasks hook to return an empty task list, exercising the
 * empty-state branch that the main test file cannot reach.
 */

import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

vi.mock("../../graphql/client", () => ({
  client: { request: vi.fn(() => Promise.resolve({})) },
}));

vi.mock("@openlobster/ui/graphql/mutations", () => ({
  ADD_TASK_MUTATION: "ADD_TASK_MUTATION",
  REMOVE_TASK_MUTATION: "REMOVE_TASK_MUTATION",
  TOGGLE_TASK_MUTATION: "TOGGLE_TASK_MUTATION",
  UPDATE_TASK_MUTATION: "UPDATE_TASK_MUTATION",
}));

vi.mock("@openlobster/ui/hooks", () => ({
  useTasks: () => ({
    data: [],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("../../App", () => ({
  t: (key: string) => {
    const map: Record<string, string> = {
      "tasks.noTasks": "No tasks scheduled",
      "tasks.noTasksHint": "Create a new task to get started",
      "tasks.newTask": "New Task",
      "tasks.scheduledTasks": "Scheduled Tasks",
      "tasks.deleteTask": "Delete Task?",
      "tasks.deleteConfirmation": "This action cannot be undone.",
      "tasks.editTask": "Edit Task",
      "tasks.createTask": "Create Task",
      "tasks.prompt": "Prompt",
      "tasks.promptPlaceholder": "Describe the task...",
      "tasks.taskType": "Task Type",
      "tasks.typeOneShot": "One-shot",
      "tasks.typeCyclic": "Cyclic",
      "tasks.schedule": "Schedule",
      "tasks.scheduleHintCron": "Cron expression",
      "tasks.scheduleHintOneShot": "Leave empty to run once immediately",
      "common.cancel": "Cancel",
      "common.save": "Save",
      "common.delete": "Delete",
      "common.closeAria": "Close",
    };
    return map[key] ?? key;
  },
}));

import TasksView from "./TasksView";

function renderWithClient(ui: () => any) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>{ui()}</QueryClientProvider>
  ));
}

describe("TasksView Component — empty state (no tasks)", () => {
  it("renders the tasks view container", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-view")).toBeTruthy();
  });

  it("shows empty state when no tasks exist", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-empty")).toBeTruthy();
  });

  it("shows empty state title", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    expect(getByText("No tasks scheduled")).toBeTruthy();
  });

  it("shows empty state hint", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    expect(getByText("Create a new task to get started")).toBeTruthy();
  });

  it("renders new task button inside empty state", () => {
    const { getAllByText } = renderWithClient(() => <TasksView />);
    expect(getAllByText("+ New Task").length).toBeGreaterThan(0);
  });

  it("does not render tasks table when no tasks exist", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-table")).toBeNull();
  });

  it("does not render tasks header when no tasks exist", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-header")).toBeNull();
  });

  it("clicking new task button in empty state opens modal", () => {
    const { container, getAllByText } = renderWithClient(() => <TasksView />);
    const btns = getAllByText("+ New Task");
    fireEvent.click(btns[0] as HTMLElement);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });
});
