// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

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

import TasksView from "./TasksView";

function renderWithClient(ui: () => any) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>{ui()}</QueryClientProvider>
  ));
}

describe("TasksView Component", () => {
  it("renders tasks view", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-view")).toBeTruthy();
  });

  it("renders header with title", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    expect(getByText("Scheduled Tasks")).toBeTruthy();
  });

  it("renders header subtitle stacked below title", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-header__text")).toBeTruthy();
  });

  it("renders new task button", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    expect(getByText("+ New Task")).toBeTruthy();
  });

  it("renders tasks table", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".tasks-table")).toBeTruthy();
  });

  it("renders table headers including # and ACTIONS", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    expect(getByText("#")).toBeTruthy();
    expect(getByText("NAME")).toBeTruthy();
    expect(getByText("SCHEDULE")).toBeTruthy();
    expect(getByText("STATUS")).toBeTruthy();
    expect(getByText("ACTIONS")).toBeTruthy();
  });

  it("renders task rows from hook data", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const rows = container.querySelectorAll("tbody tr");
    expect(rows.length).toBe(3);
  });

  it("renders task prompts from hook data", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    expect(getByText("Morning brief")).toBeTruthy();
    expect(getByText("Health check")).toBeTruthy();
    expect(getByText("Memory cleanup")).toBeTruthy();
  });

  it("renders linear #N IDs in task rows", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const idCells = container.querySelectorAll(".task-id");
    expect(idCells[0].textContent).toBe("#1");
    expect(idCells[1].textContent).toBe("#2");
    expect(idCells[2].textContent).toBe("#3");
  });

  it("renders toggle switches for each task", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const toggles = container.querySelectorAll(".toggle-switch");
    expect(toggles.length).toBe(3);
  });

  it("renders delete buttons for each task", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const deleteBtns = container.querySelectorAll(".task-delete-btn");
    expect(deleteBtns.length).toBe(3);
  });

  it("does not show new task form initially", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".new-task-form")).toBeNull();
  });

  it("shows new task form when button is clicked", () => {
    const { container, getByText } = renderWithClient(() => <TasksView />);
    const btn = getByText("+ New Task");
    fireEvent.click(btn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    expect(getByText("New Task")).toBeTruthy();
  });

  it("new task form contains required fields", () => {
    const { container, getByText } = renderWithClient(() => <TasksView />);
    fireEvent.click(getByText("+ New Task"));
    // Check for prompt textarea
    expect(container.querySelector("textarea")).toBeTruthy();
    // Check for task type selector (buttons for one-shot / cyclic)
    expect(container.querySelector(".task-type-selector")).toBeTruthy();
    // Default task type is one-shot, which renders a datetime-local input
    expect(container.querySelector('input[type="datetime-local"]')).toBeTruthy();
  });

  it("new task form has cancel and create buttons", () => {
    const { getByText } = renderWithClient(() => <TasksView />);
    fireEvent.click(getByText("+ New Task"));
    expect(getByText("Cancel")).toBeTruthy();
    expect(getByText("Create Task")).toBeTruthy();
  });

  it("hides form when cancel is clicked", () => {
    const { container, getByText } = renderWithClient(() => <TasksView />);
    fireEvent.click(getByText("+ New Task"));
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    fireEvent.click(getByText("Cancel"));
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("does not show delete modal initially", () => {
    const { container } = renderWithClient(() => <TasksView />);
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("shows delete modal when delete button is clicked", () => {
    const { container, getByText } = renderWithClient(() => <TasksView />);
    const deleteBtn = container.querySelector(".task-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    expect(getByText("Delete Task?")).toBeTruthy();
    expect(getByText("This action cannot be undone.")).toBeTruthy();
  });

  it("closes delete modal when cancel is clicked", () => {
    const { container, getAllByText } = renderWithClient(() => <TasksView />);
    const deleteBtn = container.querySelector(".task-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    // Find the Cancel button inside the modal
    const cancelBtns = getAllByText("Cancel");
    fireEvent.click(cancelBtns[cancelBtns.length - 1]);
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("renders em-dash fallback when nextRunAt is null", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const cells = container.querySelectorAll(".task-next-run");
    const dashCell = Array.from(cells).find((el) => el.textContent === "—");
    expect(dashCell).toBeTruthy();
  });

  it("renders edit buttons for each task", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const editBtns = container.querySelectorAll(".task-edit-btn");
    expect(editBtns.length).toBe(3);
  });

  it("opens edit modal when edit button is clicked", () => {
    const { container, getByText } = renderWithClient(() => <TasksView />);
    const editBtn = container.querySelector(".task-edit-btn") as HTMLElement;
    fireEvent.click(editBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    expect(getByText("Edit Task")).toBeTruthy();
  });

  it("edit modal pre-fills prompt from task data", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const editBtn = container.querySelector(".task-edit-btn") as HTMLElement;
    fireEvent.click(editBtn);
    const textarea = container.querySelector(".modal-overlay textarea") as HTMLTextAreaElement;
    expect(textarea.value).toBe("Morning brief");
  });

  it("edit modal has task-type-selector", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const editBtn = container.querySelector(".task-edit-btn") as HTMLElement;
    fireEvent.click(editBtn);
    expect(container.querySelector(".modal-overlay .task-type-selector")).toBeTruthy();
  });

  it("edit modal closes when cancel is clicked", () => {
    const { container, getAllByText } = renderWithClient(() => <TasksView />);
    const editBtn = container.querySelector(".task-edit-btn") as HTMLElement;
    fireEvent.click(editBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    const cancelBtns = getAllByText("Cancel");
    fireEvent.click(cancelBtns[cancelBtns.length - 1]);
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("switching to cyclic task type shows text input for schedule", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const newTaskBtn = Array.from(container.querySelectorAll("button")).find(
      (b) => b.textContent?.trim() === "+ New Task"
    ) as HTMLElement;
    fireEvent.click(newTaskBtn);
    // default is one-shot → datetime-local
    const modal = container.querySelector(".modal-overlay") as HTMLElement;
    expect(modal.querySelector('input[type="datetime-local"]')).toBeTruthy();
    // click Cyclic button inside modal
    const cyclicBtn = Array.from(modal.querySelectorAll(".task-type-selector button")).find(
      (b) => b.textContent?.trim() === "Cyclic"
    ) as HTMLElement;
    fireEvent.click(cyclicBtn);
    expect(modal.querySelector('input[type="text"]')).toBeTruthy();
    expect(modal.querySelector('input[type="datetime-local"]')).toBeNull();
  });

  it("switching back to one-shot shows datetime-local input", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const newTaskBtn = Array.from(container.querySelectorAll("button")).find(
      (b) => b.textContent?.trim() === "+ New Task"
    ) as HTMLElement;
    fireEvent.click(newTaskBtn);
    const modal = container.querySelector(".modal-overlay") as HTMLElement;
    const selectorBtns = modal.querySelectorAll(".task-type-selector button");
    const cyclicBtn = Array.from(selectorBtns).find((b) => b.textContent?.trim() === "Cyclic") as HTMLElement;
    const oneshotBtn = Array.from(selectorBtns).find((b) => b.textContent?.trim() === "One-shot") as HTMLElement;
    // switch to cyclic
    fireEvent.click(cyclicBtn);
    expect(modal.querySelector('input[type="text"]')).toBeTruthy();
    // switch back to one-shot
    fireEvent.click(oneshotBtn);
    expect(modal.querySelector('input[type="datetime-local"]')).toBeTruthy();
    expect(modal.querySelector('input[type="text"]')).toBeNull();
  });

  it("renders cyclic task type badge for cyclic tasks", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const cyclicBadge = container.querySelector(".task-type-badge--cyclic");
    expect(cyclicBadge).toBeTruthy();
  });

  it("renders one-shot task type badge for one-shot tasks", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const oneshotBadge = container.querySelector(".task-type-badge--oneshot");
    expect(oneshotBadge).toBeTruthy();
  });

  it("renders em-dash fallback when schedule is null", () => {
    const { container } = renderWithClient(() => <TasksView />);
    const cells = container.querySelectorAll(".task-schedule");
    const dashCell = Array.from(cells).find((el) => el.textContent === "—");
    expect(dashCell).toBeTruthy();
  });
});

function getByText_helper(container: HTMLElement, text: string): HTMLElement {
  const all = Array.from(container.querySelectorAll("*")) as HTMLElement[];
  const el = all.find((el) => el.textContent?.trim() === text && el.children.length === 0);
  if (!el) throw new Error(`Text "${text}" not found`);
  return el;
}
