// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

vi.mock("../../graphql/client", () => ({ client: {} }));

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve({ ok: true, status: 200 }))
  );
});

import DashboardView from "./DashboardView";

function renderWithClient(ui: () => any) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>{ui()}</QueryClientProvider>
  ));
}

describe("DashboardView Component", () => {
  it("renders dashboard inside app-shell", () => {
    const { container } = renderWithClient(() => <DashboardView />);
    expect(container.querySelector(".app-shell")).toBeTruthy();
  });

  it("renders stat-grid rows", () => {
    const { container } = renderWithClient(() => <DashboardView />);
    const grids = container.querySelectorAll(".stat-grid");
    expect(grids.length).toBeGreaterThanOrEqual(1);
  });

  it("renders stat cards with labels", () => {
    const { getByText, getAllByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Health")).toBeTruthy();
    expect(getByText("Active Sessions")).toBeTruthy();
    expect(getAllByText("MCP Servers").length).toBeGreaterThanOrEqual(1);
    expect(getByText("Agent Version")).toBeTruthy();
  });

  it("renders second row stat labels", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Pending Tasks")).toBeTruthy();
    expect(getByText("Completed Tasks")).toBeTruthy();
    expect(getByText("Messages Received")).toBeTruthy();
    expect(getByText("Messages Sent")).toBeTruthy();
  });

  it("renders OK heartbeat status", async () => {
    const { findByText } = renderWithClient(() => <DashboardView />);
    expect(await findByText("OK", {}, { timeout: 2000 })).toBeTruthy();
  });

  it("renders dashboard-grid with panels", () => {
    const { container } = renderWithClient(() => <DashboardView />);
    expect(container.querySelector(".dashboard-grid")).toBeTruthy();
    const panels = container.querySelectorAll(".dashboard-panel");
    expect(panels.length).toBeGreaterThanOrEqual(2);
  });

  it("renders Channels section header", () => {
    const { getAllByText } = renderWithClient(() => <DashboardView />);
    const headers = getAllByText("Channels");
    expect(headers.length).toBeGreaterThanOrEqual(1);
  });

  it("renders Recent Conversations section header", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Recent Conversations")).toBeTruthy();
  });

  it("renders System Status section header", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("System Status")).toBeTruthy();
  });

  it("renders MCP Servers section header", () => {
    const { getAllByText } = renderWithClient(() => <DashboardView />);
    const headers = getAllByText("MCP Servers");
    expect(headers.length).toBeGreaterThanOrEqual(1);
  });

  it("renders Recent Logs section", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Recent Logs")).toBeTruthy();
  });

  it("renders Memory Backend row in System Status", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Memory Backend")).toBeTruthy();
  });

  it("renders Uptime row in System Status", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Uptime")).toBeTruthy();
  });

  it("renders Secrets Backend row in System Status", () => {
    const { getByText } = renderWithClient(() => <DashboardView />);
    expect(getByText("Secrets Backend")).toBeTruthy();
  });
});
