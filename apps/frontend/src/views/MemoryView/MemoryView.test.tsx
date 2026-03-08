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
});
