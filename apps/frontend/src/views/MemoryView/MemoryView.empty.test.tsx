// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * MemoryView empty-state tests.
 *
 * Uses a separate module file so that vi.mock can be hoisted with an
 * empty memory graph — no nodes, no edges — to exercise the empty-state
 * branch of MemoryView that is not reachable via the main test file
 * (which uses the ui-tests hook alias returning populated mock data).
 */

import { describe, it, expect, vi } from "vitest";

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

vi.mock("cytoscape", () => ({
  default: () => ({ destroy: () => {}, on: () => {} }),
}));

vi.mock("../../components/GraphVisualization", () => ({
  default: () => <div class="graph-visualization-mock" />,
}));

vi.mock("@tanstack/solid-query", () => ({
  createMutation: () => ({ mutate: vi.fn(), isPending: false }),
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));

vi.mock("@openlobster/ui/graphql/mutations", () => ({
  UPDATE_MEMORY_NODE_MUTATION: "UPDATE_MEMORY_NODE_MUTATION",
  DELETE_MEMORY_NODE_MUTATION: "DELETE_MEMORY_NODE_MUTATION",
}));

vi.mock("@openlobster/ui/hooks", () => ({
  useMemory: () => ({
    data: { nodes: [], edges: [] },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("../../App", () => ({
  t: (key: string) => {
    const map: Record<string, string> = {
      "memory.noMemory": "No memory nodes yet",
      "memory.noMemoryHint": "Start a conversation to build the graph",
    };
    return map[key] ?? key;
  },
}));

import { render } from "@solidjs/testing-library";
import MemoryView from "./MemoryView";

describe("MemoryView Component — empty state (no nodes)", () => {
  it("renders empty state container when there are no nodes", () => {
    const { container } = render(() => <MemoryView />);
    expect(container.querySelector(".memory-empty")).toBeTruthy();
  });

  it("shows empty state title", () => {
    const { getByText } = render(() => <MemoryView />);
    expect(getByText("No memory nodes yet")).toBeTruthy();
  });

  it("shows empty state hint", () => {
    const { getByText } = render(() => <MemoryView />);
    expect(getByText("Start a conversation to build the graph")).toBeTruthy();
  });

  it("does not render memory-container when there are no nodes", () => {
    const { container } = render(() => <MemoryView />);
    expect(container.querySelector(".memory-container")).toBeNull();
  });

  it("does not render sidebar when there are no nodes", () => {
    const { container } = render(() => <MemoryView />);
    expect(container.querySelector(".memory-sidebar")).toBeNull();
  });
});
