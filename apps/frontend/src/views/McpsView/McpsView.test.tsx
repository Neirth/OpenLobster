// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

vi.mock("../../graphql/client", () => ({ client: {} }));

import { renderWithQueryClient } from "../../test-utils";
import McpsView from "./McpsView";

describe("McpsView Component", () => {
  it("renders mcps view", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const view = container.querySelector(".mcps-view");
    expect(view).toBeTruthy();
  });

  it("renders header with title", () => {
    const { getByText } = renderWithQueryClient(() => <McpsView />);
    expect(getByText("MCP Servers")).toBeTruthy();
  });

  it("renders add server button", () => {
    const { getByText } = renderWithQueryClient(() => <McpsView />);
    expect(getByText(/Add.*Server/)).toBeTruthy();
  });

  it("renders server cards grid", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const grid = container.querySelector(".servers-grid");
    expect(grid).toBeTruthy();
  });

  it("renders server cards", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const cards = container.querySelectorAll(".server-card");
    expect(cards.length).toBeGreaterThan(0);
  });

  it("renders manage tools buttons", () => {
    const { getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageButtons = getAllByText("Manage");
    expect(manageButtons.length).toBeGreaterThan(0);
  });
});
