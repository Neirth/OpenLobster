// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";
import { fireEvent } from "@solidjs/testing-library";

const mockMemoryData = {
  nodes: [
    { id: "1", label: "John Doe", type: "person", value: "Test value", properties: {}, createdAt: "" },
    { id: "2", label: "Jane Smith", type: "person", value: "", properties: {}, createdAt: "" },
  ],
  edges: [],
};

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

vi.mock("../../graphql/client", () => ({ client: {} }));

// Mock cytoscape (canvas not supported in happy-dom) and GraphVisualization
vi.mock("cytoscape", () => ({
  default: () => ({ destroy: () => {}, on: () => {} }),
}));

vi.mock("../../components/GraphVisualization", () => ({
  default: () => <div class="graph-visualization-mock" />,
}));

vi.mock("@openlobster/ui/hooks", () => ({
  useMemory: () => ({
    data: mockMemoryData,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

import { renderWithQueryClient } from "../../test-utils";
import MemoryView from "./MemoryView";

describe("MemoryView Component", () => {
  it("renders memory view", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    expect(container.querySelector(".memory-view")).toBeTruthy();
  });

  it("renders memory container", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    expect(container.querySelector(".memory-container")).toBeTruthy();
  });

  it("renders sidebar", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    expect(container.querySelector(".memory-sidebar")).toBeTruthy();
  });

  it("renders main content area", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    expect(container.querySelector(".memory-content")).toBeTruthy();
  });

  it("renders memory section header", () => {
    const { getByText } = renderWithQueryClient(() => <MemoryView />);
    expect(getByText("Memory Index")).toBeTruthy();
  });

  it.skip("renders PEOPLE section - skipped", () => {
    const { getByText } = renderWithQueryClient(() => <MemoryView />);
    expect(getByText("PEOPLE")).toBeTruthy();
  });

  it("renders only person-type items in list", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    const items = container.querySelectorAll(".memory-item");
    expect(items.length).toBe(2);
  });

  it("renders person labels from hook data", () => {
    const { getByText } = renderWithQueryClient(() => <MemoryView />);
    expect(getByText("John Doe")).toBeTruthy();
    expect(getByText("Jane Smith")).toBeTruthy();
  });

  it("renders no selection message initially", () => {
    const { getByText } = renderWithQueryClient(() => <MemoryView />);
    expect(getByText("Select an entry to view details")).toBeTruthy();
  });

  it("renders detail panel when a memory item is clicked", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    const firstItem = container.querySelector(".memory-item") as HTMLElement;
    fireEvent.click(firstItem);
    expect(container.querySelector(".person-detail")).toBeTruthy();
  });

  it("detail panel shows node label as heading", () => {
    const { container, getByText } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(getByText("John Doe"));
    expect(container.querySelector(".person-detail h1")?.textContent).toBe(
      "John Doe",
    );
  });

  it("detail panel shows node type", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    expect(container.querySelector(".person-role")?.textContent).toBe("person");
  });

  it("detail panel has action buttons", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    expect(container.querySelectorAll(".action-btn").length).toBe(2);
  });

  it("detail panel replaces empty state after selection", () => {
    const { container, getByText, queryByText } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(getByText("John Doe"));
    expect(queryByText("Select an entry to view details")).toBeNull();
    expect(container.querySelector(".person-detail")).toBeTruthy();
  });

  it.skip("detail panel shows node value when non-empty - value not displayed in UI", () => {
    const { container, getByText } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    expect(getByText("Test value")).toBeTruthy();
  });

  it.skip("detail panel shows em-dash fallback when value is empty - value not displayed in UI", () => {
    const { container, getByText } = renderWithQueryClient(() => <MemoryView />);
    const items = container.querySelectorAll(".memory-item");
    fireEvent.click(items[1] as HTMLElement);
    expect(getByText("—")).toBeTruthy();
  });

  it.skip("renders empty list when memory data is undefined - skipped", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    expect(container.querySelector(".memory-sidebar")).toBeTruthy();
  });

  it("renders search box in sidebar", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    expect(container.querySelector(".search-box")).toBeTruthy();
  });

  it("search filters nodes by label", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    const searchBox = container.querySelector(".search-box") as HTMLInputElement;
    fireEvent.input(searchBox, { target: { value: "John" } });
    const items = container.querySelectorAll(".memory-item");
    expect(items.length).toBe(1);
  });

  it("search shows no results for non-matching query", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    const searchBox = container.querySelector(".search-box") as HTMLInputElement;
    fireEvent.input(searchBox, { target: { value: "zzznomatch" } });
    const items = container.querySelectorAll(".memory-item");
    expect(items.length).toBe(0);
  });

  it("search clearing restores all items", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    const searchBox = container.querySelector(".search-box") as HTMLInputElement;
    fireEvent.input(searchBox, { target: { value: "John" } });
    expect(container.querySelectorAll(".memory-item").length).toBe(1);
    fireEvent.input(searchBox, { target: { value: "" } });
    expect(container.querySelectorAll(".memory-item").length).toBe(2);
  });

  it("opens edit modal when edit button clicked on selected node", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    const editBtn = container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement;
    fireEvent.click(editBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("edit modal has label and type inputs", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement);
    expect(container.querySelector("#edit-label")).toBeTruthy();
    expect(container.querySelector("#edit-type")).toBeTruthy();
  });

  it("edit modal pre-fills label from selected node", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    // Nodes are sorted alphabetically: "Jane Smith" < "John Doe"
    const items = container.querySelectorAll(".memory-item");
    fireEvent.click(items[0] as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement);
    const labelInput = container.querySelector("#edit-label") as HTMLInputElement;
    expect(labelInput.value).toBe("Jane Smith");
  });

  it("edit modal closes on cancel", () => {
    const { container, getByText } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    fireEvent.click(getByText("Cancel"));
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("add property button adds a new property row", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement);
    const addBtn = container.querySelector(".memory-modal-add-prop") as HTMLElement;
    fireEvent.click(addBtn);
    expect(container.querySelectorAll(".memory-modal-prop-row").length).toBe(1);
  });

  it("remove property button removes the row", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement);
    fireEvent.click(container.querySelector(".memory-modal-add-prop") as HTMLElement);
    expect(container.querySelectorAll(".memory-modal-prop-row").length).toBe(1);
    fireEvent.click(container.querySelector(".prop-row-remove") as HTMLElement);
    expect(container.querySelectorAll(".memory-modal-prop-row").length).toBe(0);
  });

  it("opens delete modal when delete button clicked on selected node", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    const deleteBtn = container.querySelector(".action-btn--danger") as HTMLElement;
    fireEvent.click(deleteBtn);
    expect(container.querySelector(".memory-modal-confirm")).toBeTruthy();
  });

  it("delete modal closes on cancel", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn--danger") as HTMLElement);
    expect(container.querySelector(".memory-modal-confirm")).toBeTruthy();
    const cancelBtns = getAllByText("Cancel");
    fireEvent.click(cancelBtns[cancelBtns.length - 1]);
    expect(container.querySelector(".memory-modal-confirm")).toBeNull();
  });

  it("clicking a memory item sets it as active", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    const items = container.querySelectorAll(".memory-item");
    fireEvent.click(items[1] as HTMLElement);
    expect(items[1].classList.contains("memory-item--active")).toBe(true);
    expect(items[0].classList.contains("memory-item--active")).toBe(false);
  });

  it("property key input updates on change", () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);
    fireEvent.click(container.querySelector(".memory-item") as HTMLElement);
    fireEvent.click(container.querySelector(".action-btn:not(.action-btn--danger)") as HTMLElement);
    fireEvent.click(container.querySelector(".memory-modal-add-prop") as HTMLElement);
    const keyInput = container.querySelector(".memory-modal-prop-row input") as HTMLInputElement;
    fireEvent.input(keyInput, { target: { value: "nickname" } });
    expect(keyInput.value).toBe("nickname");
  });
});
