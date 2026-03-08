// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { waitFor } from "@solidjs/testing-library";

// Mock Router first
vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

// Mock AppShell
vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

// Mock GraphQL client
vi.mock("../../graphql/client", () => ({
  client: {},
  GRAPHQL_ENDPOINT: "http://127.0.0.1:8080/graphql",
}));

// Mock mutations hook
vi.mock("../../hooks/mutations", () => ({
  useUpdateConfig: () => ({
    mutate: vi.fn(),
    isPending: false,
    isSuccess: false,
  }),
}));

vi.mock("../../stores/authStore", () => ({
  getStoredToken: () => null,
  setNeedsAuth: () => {},
}));

const mockFetch = vi.fn();
beforeEach(() => {
  mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
    const body = options?.body ? JSON.parse(options.body) : {};
    const query = body.query || "";
    const isConfig = query.includes("config") || query.includes("Config");
    return Promise.resolve({
      ok: true,
      status: 200,
      json: async () =>
        isConfig
          ? {
              data: {
                config: {
                  agent: { name: "TestAgent", provider: "ollama", model: "llama3.2:latest" },
                  capabilities: {},
                  database: {},
                  memory: {},
                  graphql: {},
                  logging: {},
                  secrets: {},
                  scheduler: {},
                  channelSecrets: {},
                },
              },
            }
          : { data: { systemFiles: [] } },
    });
  });
  global.fetch = mockFetch;
});

import { renderWithQueryClient } from "../../test-utils";
import SettingsView from "./SettingsView";

describe("SettingsView Component", () => {
  it("renders settings view", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      const view = container.querySelector(".settings-view");
      expect(view).toBeTruthy();
    });
  });

  it("renders settings header", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    expect(await findByText("Settings")).toBeTruthy();
  });

  it("renders general configuration section", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    expect(await findByText("GENERAL CONFIGURATION")).toBeTruthy();
  });

  it("renders agent capabilities section", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    expect(await findByText("AGENT CAPABILITIES")).toBeTruthy();
  });

  it("renders scheduler configuration section", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    expect(await findByText("SCHEDULER CONFIGURATION")).toBeTruthy();
  });

  it("renders documentation links section", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    expect(await findByText("DOCUMENTATION FOR CREATING BOTS", {}, { timeout: 3000 })).toBeTruthy();
  });

  it("renders schema fields", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      const fields = container.querySelectorAll(".schema-field");
      expect(fields.length).toBeGreaterThan(0);
    });
  });

  it("renders toggle switches in capabilities", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      const toggles = container.querySelectorAll(".toggle-switch");
      expect(toggles.length).toBeGreaterThan(0);
    });
  });

  it("renders save button", async () => {
    const { container, findByRole } = renderWithQueryClient(() => <SettingsView />);
    const saveBtn = await findByRole("button", { name: /save changes/i }, { timeout: 3000 });
    expect(saveBtn).toBeTruthy();
    expect(container.querySelector(".settings-actions button")).toBeTruthy();
  });
});
