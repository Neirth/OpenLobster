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

  it("renders workspace files editor after loading", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor")).toBeTruthy();
    });
  });

  it("renders workspace file tabs (AGENTS.md, SOUL.md, IDENTITY.md)", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    expect(await findByText("AGENTS.md")).toBeTruthy();
    expect(await findByText("SOUL.md")).toBeTruthy();
    expect(await findByText("IDENTITY.md")).toBeTruthy();
  });

  it("renders workspace textarea", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor__textarea")).toBeTruthy();
    });
  });

  it("renders file save button", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      const saveButtons = container.querySelectorAll(".save-btn");
      expect(saveButtons.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("switching workspace tabs updates active tab", async () => {
    const { findByText } = renderWithQueryClient(() => <SettingsView />);
    const soulTab = await findByText("SOUL.md");
    const agentsTab = await findByText("AGENTS.md");
    // AGENTS.md is active by default
    expect(agentsTab.classList.contains("active")).toBe(true);
    fireEvent.click(soulTab);
    expect(soulTab.classList.contains("active")).toBe(true);
    expect(agentsTab.classList.contains("active")).toBe(false);
  });

  it("typing in workspace textarea updates content", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor__textarea")).toBeTruthy();
    });
    const textarea = container.querySelector(".workspace-editor__textarea") as HTMLTextAreaElement;
    fireEvent.input(textarea, { target: { value: "# My agents content" } });
    expect(textarea.value).toBe("# My agents content");
  });

  it("clicking save triggers fetch call", async () => {
    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".save-btn")).toBeTruthy();
    });
    const saveBtn = container.querySelector(".settings-actions .save-btn") as HTMLElement;
    fireEvent.click(saveBtn);
    await waitFor(() => {
      // fetch called at least once for save (beyond initial load calls)
      expect(mockFetch.mock.calls.length).toBeGreaterThan(2);
    });
  });

  it("shows save error when response has errors", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".save-btn")).toBeTruthy();
    });
    // Override fetch to return errors on save mutation
    mockFetch.mockImplementationOnce(() =>
      Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ errors: [{ message: "Save failed" }] }),
      })
    );
    const saveBtn = container.querySelector(".settings-actions .save-btn") as HTMLElement;
    fireEvent.click(saveBtn);
    await waitFor(() => {
      expect(container.querySelector(".save-error")).toBeTruthy();
    });
  });

  it("shows loading state while config is fetching", () => {
    // Delay the fetch to keep isLoading true
    mockFetch.mockImplementation(() => new Promise(() => {}));
    const { container } = renderWithQueryClient(() => <SettingsView />);
    expect(container.querySelector(".settings-loading")).toBeTruthy();
  });

  it("shows 401 error — settings-view never mounts on 401 response", async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 401, json: async () => ({}) })
    );
    const { queryByText } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(queryByText("Settings")).toBeNull();
    }, { timeout: 3000 });
  });

  it("renders doc links section with external links", async () => {
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      const links = container.querySelectorAll(".settings-doc-links__cell");
      expect(links.length).toBeGreaterThan(0);
    });
  });

  it("workspace file save shows success message on success response", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isWriteFile = query.includes("writeSystemFile") || query.includes("WriteSystemFile");
      const isConfig = query.includes("config") || query.includes("Config");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isWriteFile
            ? { data: { writeSystemFile: { success: true } } }
            : isConfig
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
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-btn")).toBeTruthy();
    });
    const workspaceSaveBtn = container.querySelector(".workspace-editor .save-btn") as HTMLElement;
    fireEvent.click(workspaceSaveBtn);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-success")).toBeTruthy();
    });
  });

  it("workspace file save shows error message on failure response", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isWriteFile = query.includes("writeSystemFile") || query.includes("WriteSystemFile");
      const isConfig = query.includes("config") || query.includes("Config");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isWriteFile
            ? { data: { writeSystemFile: { success: false, error: "Write failed" } } }
            : isConfig
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
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-btn")).toBeTruthy();
    });
    const workspaceSaveBtn = container.querySelector(".workspace-editor .save-btn") as HTMLElement;
    fireEvent.click(workspaceSaveBtn);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-error")).toBeTruthy();
    });
  });

  it("clicking save shows save-success when response is successful", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig = (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
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
            : query.includes("updateConfig")
            ? { data: { updateConfig: { agentName: "TestAgent" } } }
            : { data: { systemFiles: [] } },
      });
    });
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy();
    });
    const saveBtn = container.querySelector(".settings-actions .save-btn") as HTMLElement;
    fireEvent.click(saveBtn);
    await waitFor(() => {
      expect(container.querySelector(".save-success")).toBeTruthy();
    });
  });

  it("handleSave handles 401 response", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig = (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      if (isConfig) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
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
          }),
        });
      }
      // Return 401 for mutation
      return Promise.resolve({ ok: false, status: 401, json: async () => ({}) });
    });
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy();
    });
    const saveBtn = container.querySelector(".settings-actions .save-btn") as HTMLElement;
    fireEvent.click(saveBtn);
    // After 401 the view may redirect; just ensure no crash
    await waitFor(() => {
      expect(container).toBeTruthy();
    });
  });

  it("handleSave handles fetch network error", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig = (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      if (isConfig) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
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
          }),
        });
      }
      return Promise.reject(new Error("Network error"));
    });
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy();
    });
    const saveBtn = container.querySelector(".settings-actions .save-btn") as HTMLElement;
    fireEvent.click(saveBtn);
    await waitFor(() => {
      expect(container.querySelector(".save-error")).toBeTruthy();
    });
  });

  it("workspace file save handles 401 response", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isWriteFile = query.includes("writeSystemFile") || query.includes("WriteSystemFile");
      const isConfig = query.includes("config") || query.includes("Config");
      return Promise.resolve({
        ok: false,
        status: isWriteFile ? 401 : 200,
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
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-btn")).toBeTruthy();
    });
    const workspaceSaveBtn = container.querySelector(".workspace-editor .save-btn") as HTMLElement;
    // Clicking should not throw even if 401 triggers setNeedsAuth
    fireEvent.click(workspaceSaveBtn);
    await waitFor(() => {
      // After 401 the settings view may unmount; just ensure no crash
      expect(container).toBeTruthy();
    });
  });

  it("workspace file save handles fetch network error", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isWriteFile = query.includes("writeSystemFile") || query.includes("WriteSystemFile");
      const isConfig = query.includes("config") || query.includes("Config");
      if (isWriteFile) return Promise.reject(new Error("Network failure"));
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
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-btn")).toBeTruthy();
    });
    const workspaceSaveBtn = container.querySelector(".workspace-editor .save-btn") as HTMLElement;
    fireEvent.click(workspaceSaveBtn);
    await waitFor(() => {
      expect(container.querySelector(".workspace-editor .save-error")).toBeTruthy();
    });
  });
});

// Need fireEvent imported for the new tests
import { fireEvent } from "@solidjs/testing-library";
