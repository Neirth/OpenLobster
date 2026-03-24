// Copyright (c) OpenLobster contributors. See LICENSE for details.
 

import { describe, it, expect, vi, beforeEach } from "vitest";
import { waitFor } from "@solidjs/testing-library";
import { configGroups, configSchema } from "../../schemas/config.schema";
import { CONFIG_QUERY } from "@openlobster/ui/graphql/queries";

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
                  capabilities: {
                    browser: false,
                    terminal: false,
                    subagents: true,
                    memory: true,
                    mcp: true,
                    filesystem: true,
                    sessions: true,
                  },
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

// ─── Regression: editable agent config field coverage ────────────────────────
//
// CANONICAL LIST — add new frontend-editable agent config fields here.
// Every entry is automatically checked against:
//   1. configSchema.properties  – the field must have a JSON Schema definition
//   2. configGroups.general     – the field must be registered for UI rendering
//   3. the save mutation input  – the field must be sent to the backend on save
//
// To add a new field: append it to EDITABLE_AGENT_FIELDS below.
// If any layer is missing the tests will fail and point you to the gap.
// ─────────────────────────────────────────────────────────────────────────────

const EDITABLE_AGENT_FIELDS = [
  "agentName",
  "provider",
  "model",
  "apiKey",
  "baseURL",
  "ollamaHost",
  "ollamaApiKey",
  "anthropicApiKey",
  "dockerModelRunnerEndpoint",
  "reasoningLevel",
] as const;

// Some form field names differ from their GraphQL field names in CONFIG_QUERY.
// List only the exceptions; everything else uses the form field name as-is.
const QUERY_FIELD_NAME: Partial<Record<(typeof EDITABLE_AGENT_FIELDS)[number], string>> = {
  agentName: "name", // queried as `agent { name }`, stored in formValues as `agentName`
};

describe("SettingsView — editable agent config field coverage", () => {
  it.each(EDITABLE_AGENT_FIELDS)(
    "field '%s' is in configSchema.properties and configGroups.general.fields",
    (field) => {
      const generalGroup = configGroups.find((g) => g.id === "general");
      expect(generalGroup, "group 'general' must exist in configGroups").toBeTruthy();
      expect(
        generalGroup!.fields,
        `'${field}' must be listed in configGroups.general.fields`
      ).toContain(field);
      expect(
        configSchema.properties[field],
        `'${field}' must be defined in configSchema.properties`
      ).toBeTruthy();
    }
  );

  it("save mutation sends all editable agent fields to the backend", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig =
        (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isConfig
            ? {
                data: {
                  config: {
                    agent: {
                      name: "Bot",
                      provider: "anthropic",
                      model: "claude-sonnet-4-6",
                      anthropicApiKey: "sk-ant-test",
                      reasoningLevel: "high",
                    },
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
            ? { data: { updateConfig: { agentName: "Bot" } } }
            : { data: { systemFiles: [] } },
      });
    });

    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );

    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);

    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;
      for (const field of EDITABLE_AGENT_FIELDS) {
        expect(field in input, `field '${field}' must be present in the save mutation input`).toBe(
          true
        );
      }
    });
  });

  // ── Gap 1: CONFIG_QUERY must request every field ──────────────────────────
  // If a field is missing from the query the server never sends it and the
  // form always shows the default value regardless of what was saved.
  it.each(EDITABLE_AGENT_FIELDS)(
    "CONFIG_QUERY requests field '%s' from the server",
    (field) => {
      const queryField = QUERY_FIELD_NAME[field] ?? field;
      // Ensure the field appears specifically inside the `agent { ... }` selection
      // block to avoid false positives from matching common tokens elsewhere.
      const regex = new RegExp(`agent\\s*{[\\s\\S]*?\\b${queryField}\\b`);
      expect(
        CONFIG_QUERY.match(regex),
        `CONFIG_QUERY must include field '${queryField}' inside the 'agent { ... }' block (form field '${field}') — otherwise it is never loaded from the server`
      ).toBeTruthy();
    }
  );

  // ── Gap 2: server values must survive the full round-trip ─────────────────
  // Verifies that onMount / setFormValues actually loads the server values
  // (not just default values) and that they are forwarded in the save mutation.
  it("server-loaded values are forwarded unchanged in the save mutation", async () => {
    // Distinct sentinel values — different from any hard-coded default.
    const serverAgent = {
      name: "ServerBot",
      provider: "anthropic",
      model: "claude-opus-4",
      apiKey: "",
      baseURL: "",
      ollamaHost: "",
      ollamaApiKey: "",
      anthropicApiKey: "sk-ant-server",
      dockerModelRunnerEndpoint: "http://dmr.server:9999",
      reasoningLevel: "low",
    };

    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig =
        (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isConfig
            ? { data: { config: { agent: serverAgent, capabilities: {}, database: {}, memory: {}, graphql: {}, logging: {}, secrets: {}, scheduler: {}, channelSecrets: {} } } }
            : query.includes("updateConfig")
            ? { data: { updateConfig: { agentName: serverAgent.name } } }
            : { data: { systemFiles: [] } },
      });
    });

    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );

    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);

    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;

      // Fields whose sentinel value differs from the hard-coded default.
      const nonDefaultFields: Partial<Record<(typeof EDITABLE_AGENT_FIELDS)[number], string>> = {
        agentName: serverAgent.name,
        provider: serverAgent.provider,
        model: serverAgent.model,
        anthropicApiKey: serverAgent.anthropicApiKey,
        dockerModelRunnerEndpoint: serverAgent.dockerModelRunnerEndpoint,
        reasoningLevel: serverAgent.reasoningLevel,
      };
      for (const [field, expected] of Object.entries(nonDefaultFields)) {
        expect(
          input[field],
          `field '${field}' must reflect the server value '${expected}', not a hard-coded default`
        ).toBe(expected);
      }
    });
  });
});

// ─── Regression: editable channel config field coverage ──────────────────────
//
// CANONICAL LIST — add new frontend-editable channel fields here.
// Every entry is checked against:
//   1. configSchema.properties  – field must have a JSON Schema definition
//   2. configGroups.channels    – field must be registered for UI rendering
//   3. CONFIG_QUERY             – field must be requested from the server
//   4. save mutation input      – field must be sent to the backend on save
// ─────────────────────────────────────────────────────────────────────────────

const EDITABLE_CHANNEL_FIELDS = [
  "channelTelegramEnabled",
  "channelTelegramToken",
  "channelDiscordEnabled",
  "channelDiscordToken",
  "channelWhatsAppEnabled",
  "channelWhatsAppPhoneId",
  "channelWhatsAppApiToken",
  "channelTwilioEnabled",
  "channelTwilioAccountSid",
  "channelTwilioAuthToken",
  "channelTwilioFromNumber",
  "channelSlackEnabled",
  "channelSlackBotToken",
  "channelSlackAppToken",
] as const;

// Maps each channelXxx form key to its name inside the CONFIG_QUERY
// channelSecrets block (shorter GraphQL field names without the "channel" prefix).
const CHANNEL_QUERY_FIELD_NAME: Partial<Record<(typeof EDITABLE_CHANNEL_FIELDS)[number], string>> =
  {
    channelTelegramEnabled: "telegramEnabled",
    channelTelegramToken: "telegramToken",
    channelDiscordEnabled: "discordEnabled",
    channelDiscordToken: "discordToken",
    channelWhatsAppEnabled: "whatsAppEnabled",
    channelWhatsAppPhoneId: "whatsAppPhoneId",
    channelWhatsAppApiToken: "whatsAppApiToken",
    channelTwilioEnabled: "twilioEnabled",
    channelTwilioAccountSid: "twilioAccountSid",
    channelTwilioAuthToken: "twilioAuthToken",
    channelTwilioFromNumber: "twilioFromNumber",
    channelSlackEnabled: "slackEnabled",
    channelSlackBotToken: "slackBotToken",
    channelSlackAppToken: "slackAppToken",
  };

describe("SettingsView — editable channel config field coverage", () => {
  it.each(EDITABLE_CHANNEL_FIELDS)(
    "field '%s' is in configSchema.properties and configGroups.channels.fields",
    (field) => {
      const channelsGroup = configGroups.find((g) => g.id === "channels");
      expect(channelsGroup, "group 'channels' must exist in configGroups").toBeTruthy();
      expect(
        channelsGroup!.fields,
        `'${field}' must be listed in configGroups.channels.fields`
      ).toContain(field);
      expect(
        configSchema.properties[field],
        `'${field}' must be defined in configSchema.properties`
      ).toBeTruthy();
    }
  );

  it.each(EDITABLE_CHANNEL_FIELDS)(
    "CONFIG_QUERY requests channel field '%s' from the server",
    (field) => {
      const queryField = CHANNEL_QUERY_FIELD_NAME[field] ?? field;
      expect(
        CONFIG_QUERY,
        `CONFIG_QUERY must include '${queryField}' (form field '${field}')`
      ).toContain(queryField);
    }
  );

  it("save mutation sends all editable channel fields to the backend", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig =
        (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isConfig
            ? {
                data: {
                  config: {
                    agent: { name: "Bot", provider: "openai", model: "gpt-4o" },
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
            ? { data: { updateConfig: { agentName: "Bot" } } }
            : { data: { systemFiles: [] } },
      });
    });

    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );

    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);

    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;
      for (const field of EDITABLE_CHANNEL_FIELDS) {
        expect(
          field in input,
          `channel field '${field}' must be present in the save mutation input`
        ).toBe(true);
      }
    });
  });

  it("server-loaded Slack values are forwarded unchanged in the save mutation", async () => {
    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig =
        (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isConfig
            ? {
                data: {
                  config: {
                    agent: { name: "Bot", provider: "openai", model: "gpt-4o" },
                    capabilities: {},
                    database: {},
                    memory: {},
                    graphql: {},
                    logging: {},
                    secrets: {},
                    scheduler: {},
                    channelSecrets: {
                      slackEnabled: true,
                      slackBotToken: "xoxb-server-token",
                      slackAppToken: "xapp-server-token",
                    },
                  },
                },
              }
            : query.includes("updateConfig")
            ? { data: { updateConfig: { agentName: "Bot" } } }
            : { data: { systemFiles: [] } },
      });
    });

    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );

    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);

    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;
      expect(input.channelSlackEnabled, "Slack enabled must be loaded from server").toBe(true);
      expect(input.channelSlackBotToken, "Slack bot token must be loaded from server").toBe(
        "xoxb-server-token"
      );
      expect(input.channelSlackAppToken, "Slack app token must be loaded from server").toBe(
        "xapp-server-token"
      );
    });
  });
});

// ─── Regression: ALL remaining config groups ─────────────────────────────────
//
// Canonical lists for every group not yet covered above (database, memory,
// subagents, graphql, logging, secrets, scheduler, capabilities).
// Each group is checked against the same 4 layers:
//   1. configSchema.properties  – field must have a JSON Schema definition
//   2. configGroups[id].fields  – field must be registered for UI rendering
//   3. CONFIG_QUERY             – field name (or its alias) must appear in the query
//   4. save mutation input      – field must be present in the mutation variables
// ─────────────────────────────────────────────────────────────────────────────

// Capabilities is a single object field in configGroups (one entry, "capabilities"),
// but its sub-fields are what we care about in CONFIG_QUERY and save mutation.
const CAPABILITY_SUBFIELDS = [
  "browser",
  "terminal",
  "subagents",
  "memory",
  "mcp",
  "filesystem",
  "sessions",
] as const;

// Every other group uses flat string keys matching both the form key and the mutation key.
const EDITABLE_DATABASE_FIELDS = [
  "databaseDriver",
  "databaseDSN",
  "databaseMaxOpenConns",
  "databaseMaxIdleConns",
] as const;

const EDITABLE_MEMORY_FIELDS = [
  "memoryBackend",
  "memoryFilePath",
  "memoryNeo4jURI",
  "memoryNeo4jUser",
  "memoryNeo4jPassword",
] as const;

const EDITABLE_SUBAGENTS_FIELDS = [
  "subagentsMaxConcurrent",
  "subagentsDefaultTimeout",
] as const;

const EDITABLE_GRAPHQL_FIELDS = [
  "graphqlEnabled",
  "graphqlPort",
  "graphqlHost",
  "graphqlBaseUrl",
] as const;

const EDITABLE_LOGGING_FIELDS = [
  "loggingLevel",
  "loggingPath",
] as const;

const EDITABLE_SECRETS_FIELDS = [
  "secretsBackend",
  "secretsFilePath",
  "secretsOpenbaoURL",
  "secretsOpenbaoToken",
] as const;

const EDITABLE_SCHEDULER_FIELDS = [
  "schedulerEnabled",
  "schedulerMemoryEnabled",
  "schedulerMemoryInterval",
] as const;

// Maps form field names to the shorter names used inside CONFIG_QUERY blocks.
// Fields whose form key matches the query name exactly are omitted.
const OTHER_QUERY_FIELD_NAME: Record<string, string> = {
  databaseDriver: "driver",
  databaseDSN: "dsn",
  databaseMaxOpenConns: "maxOpenConns",
  databaseMaxIdleConns: "maxIdleConns",
  memoryBackend: "backend",
  memoryFilePath: "filePath",
  memoryNeo4jURI: "uri",
  memoryNeo4jUser: "user",
  memoryNeo4jPassword: "password",
  subagentsMaxConcurrent: "maxConcurrent",
  subagentsDefaultTimeout: "defaultTimeout",
  graphqlEnabled: "enabled",
  graphqlPort: "port",
  graphqlHost: "host",
  graphqlBaseUrl: "baseUrl",
  loggingLevel: "level",
  loggingPath: "path",
  secretsBackend: "backend",
  secretsFilePath: "path",
  secretsOpenbaoURL: "url",
  secretsOpenbaoToken: "token",
  schedulerEnabled: "enabled",
  schedulerMemoryEnabled: "memoryEnabled",
  schedulerMemoryInterval: "memoryInterval",
};

// Helper: all non-agent, non-channel editable fields in one flat array.
const ALL_OTHER_EDITABLE_FIELDS = [
  ...EDITABLE_DATABASE_FIELDS,
  ...EDITABLE_MEMORY_FIELDS,
  ...EDITABLE_SUBAGENTS_FIELDS,
  ...EDITABLE_GRAPHQL_FIELDS,
  ...EDITABLE_LOGGING_FIELDS,
  ...EDITABLE_SECRETS_FIELDS,
  ...EDITABLE_SCHEDULER_FIELDS,
] as const;

// ── 1. Capabilities ──────────────────────────────────────────────────────────

describe("SettingsView — capabilities config coverage", () => {
  it("field 'capabilities' is in configSchema.properties and configGroups.capabilities.fields", () => {
    const capGroup = configGroups.find((g) => g.id === "capabilities");
    expect(capGroup, "group 'capabilities' must exist in configGroups").toBeTruthy();
    expect(capGroup!.fields, "'capabilities' must be in configGroups.capabilities.fields").toContain(
      "capabilities"
    );
    expect(
      configSchema.properties["capabilities"],
      "'capabilities' must be defined in configSchema.properties"
    ).toBeTruthy();
  });

  it.each(CAPABILITY_SUBFIELDS)(
    "CONFIG_QUERY requests capability sub-field '%s'",
    (subField) => {
      expect(
        CONFIG_QUERY,
        `CONFIG_QUERY must include '${subField}' inside the capabilities block`
      ).toContain(subField);
    }
  );

  it("save mutation sends capabilities object with all sub-fields", async () => {
    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );
    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);
    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;
      expect("capabilities" in input, "capabilities object must be in mutation input").toBe(true);
      const caps = input.capabilities ?? {};
      for (const sub of CAPABILITY_SUBFIELDS) {
        expect(
          sub in caps,
          `capabilities.${sub} must be present in mutation input`
        ).toBe(true);
      }
    });
  });
});

// ── 2. Database, Memory, Subagents, GraphQL, Logging, Secrets, Scheduler ──────

describe("SettingsView — editable non-agent/channel config field coverage", () => {
  const GROUP_OF: Record<string, string> = {
    databaseDriver: "database",
    databaseDSN: "database",
    databaseMaxOpenConns: "database",
    databaseMaxIdleConns: "database",
    memoryBackend: "memory",
    memoryFilePath: "memory",
    memoryNeo4jURI: "memory",
    memoryNeo4jUser: "memory",
    memoryNeo4jPassword: "memory",
    subagentsMaxConcurrent: "subagents",
    subagentsDefaultTimeout: "subagents",
    graphqlEnabled: "graphql",
    graphqlPort: "graphql",
    graphqlHost: "graphql",
    graphqlBaseUrl: "graphql",
    loggingLevel: "logging",
    loggingPath: "logging",
    secretsBackend: "secrets",
    secretsFilePath: "secrets",
    secretsOpenbaoURL: "secrets",
    secretsOpenbaoToken: "secrets",
    schedulerEnabled: "scheduler",
    schedulerMemoryEnabled: "scheduler",
    schedulerMemoryInterval: "scheduler",
  };

  it.each(ALL_OTHER_EDITABLE_FIELDS)(
    "field '%s' is in configSchema.properties and its configGroup",
    (field) => {
      const groupId = GROUP_OF[field];
      const group = configGroups.find((g) => g.id === groupId);
      expect(group, `group '${groupId}' must exist in configGroups`).toBeTruthy();
      expect(
        group!.fields,
        `'${field}' must be listed in configGroups.${groupId}.fields`
      ).toContain(field);
      expect(
        configSchema.properties[field],
        `'${field}' must be defined in configSchema.properties`
      ).toBeTruthy();
    }
  );

  it.each(ALL_OTHER_EDITABLE_FIELDS)(
    "CONFIG_QUERY requests field '%s'",
    (field) => {
      const queryField = OTHER_QUERY_FIELD_NAME[field] ?? field;
      expect(
        CONFIG_QUERY,
        `CONFIG_QUERY must include '${queryField}' (form field '${field}')`
      ).toContain(queryField);
    }
  );

  it("save mutation sends all database/memory/subagents/graphql/logging/secrets/scheduler fields", async () => {
    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );
    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);
    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;
      for (const field of ALL_OTHER_EDITABLE_FIELDS) {
        expect(
          field in input,
          `field '${field}' must be present in the save mutation input`
        ).toBe(true);
      }
    });
  });
});

// ── 3. Round-trip: all groups load from server and reach the mutation ─────────

describe("SettingsView — all groups round-trip: server values reach mutation", () => {
  it("server-loaded values for all config groups are forwarded in the save mutation", async () => {
    const serverConfig = {
      agent: {
        name: "RoundTripBot",
        provider: "anthropic",
        model: "claude-opus-4",
        anthropicApiKey: "sk-ant-rt",
        reasoningLevel: "low",
      },
      capabilities: {
        browser: true,
        terminal: false,
        subagents: true,
        memory: false,
        mcp: true,
        filesystem: false,
        sessions: true,
      },
      database: {
        driver: "postgres",
        dsn: "postgres://rt:rt@localhost/rt",
        maxOpenConns: 8,
        maxIdleConns: 3,
      },
      memory: {
        backend: "neo4j",
        filePath: "./rt.gml",
        neo4j: { uri: "bolt://rt:7687", user: "rt", password: "rt-pass" },
      },
      subagents: { maxConcurrent: 6, defaultTimeout: "120s" },
      graphql: { enabled: false, port: 9999, host: "127.0.0.1", baseUrl: "https://rt.test" },
      logging: { level: "warn", path: "./rt.log" },
      secrets: {
        backend: "openbao",
        file: { path: "./rt-secrets.json" },
        openbao: { url: "https://rt-vault.test", token: "hvs.rt" },
      },
      scheduler: { enabled: false, memoryEnabled: true, memoryInterval: "2h" },
      channelSecrets: {
        slackEnabled: true,
        slackBotToken: "xoxb-rt",
        slackAppToken: "xapp-rt",
        telegramEnabled: false,
        telegramToken: "tg-rt",
        discordEnabled: true,
        discordToken: "dc-rt",
        whatsAppEnabled: false,
        whatsAppPhoneId: "+34900000001",
        whatsAppApiToken: "wa-rt",
        twilioEnabled: true,
        twilioAccountSid: "AC-rt",
        twilioAuthToken: "tw-rt",
        twilioFromNumber: "+15550000002",
      },
    };

    mockFetch.mockImplementation((_url: string, options?: { body?: string }) => {
      const body = options?.body ? JSON.parse(options.body) : {};
      const query = body.query || "";
      const isConfig =
        (query.includes("config") || query.includes("Config")) && !query.includes("mutation");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () =>
          isConfig
            ? { data: { config: serverConfig } }
            : query.includes("updateConfig")
            ? { data: { updateConfig: { agentName: "RoundTripBot" } } }
            : { data: { systemFiles: [] } },
      });
    });

    mockFetch.mockClear();
    const { container } = renderWithQueryClient(() => <SettingsView />);
    await waitFor(() =>
      expect(container.querySelector(".settings-actions .save-btn")).toBeTruthy()
    );

    fireEvent.click(container.querySelector(".settings-actions .save-btn") as HTMLElement);

    await waitFor(() => {
      const saveCalls = mockFetch.mock.calls.filter(([, opts]) => {
        const b = opts?.body ? JSON.parse(opts.body) : {};
        return (b.query ?? "").includes("updateConfig") && b.variables?.input;
      });
      expect(saveCalls.length).toBeGreaterThan(0);
      const input = JSON.parse(saveCalls[0][1].body).variables.input;

      // Agent
      expect(input.agentName, "agentName from server").toBe("RoundTripBot");
      expect(input.anthropicApiKey, "anthropicApiKey from server").toBe("sk-ant-rt");
      expect(input.reasoningLevel, "reasoningLevel from server").toBe("low");

      // Capabilities
      const caps = input.capabilities ?? {};
      expect(caps.browser, "capabilities.browser from server").toBe(true);
      expect(caps.terminal, "capabilities.terminal from server").toBe(false);
      expect(caps.mcp, "capabilities.mcp from server").toBe(true);

      // Database
      expect(input.databaseDriver, "databaseDriver from server").toBe("postgres");
      expect(input.databaseDSN, "databaseDSN from server").toBe("postgres://rt:rt@localhost/rt");
      expect(input.databaseMaxOpenConns, "databaseMaxOpenConns from server").toBe(8);
      expect(input.databaseMaxIdleConns, "databaseMaxIdleConns from server").toBe(3);

      // Memory
      expect(input.memoryBackend, "memoryBackend from server").toBe("neo4j");
      expect(input.memoryFilePath, "memoryFilePath from server").toBe("./rt.gml");
      expect(input.memoryNeo4jURI, "memoryNeo4jURI from server").toBe("bolt://rt:7687");
      expect(input.memoryNeo4jUser, "memoryNeo4jUser from server").toBe("rt");
      expect(input.memoryNeo4jPassword, "memoryNeo4jPassword from server").toBe("rt-pass");

      // Subagents
      expect(input.subagentsMaxConcurrent, "subagentsMaxConcurrent from server").toBe(6);

      // GraphQL
      expect(input.graphqlEnabled, "graphqlEnabled from server").toBe(false);
      expect(input.graphqlPort, "graphqlPort from server").toBe(9999);
      expect(input.graphqlHost, "graphqlHost from server").toBe("127.0.0.1");
      expect(input.graphqlBaseUrl, "graphqlBaseUrl from server").toBe("https://rt.test");

      // Logging
      expect(input.loggingLevel, "loggingLevel from server").toBe("warn");
      expect(input.loggingPath, "loggingPath from server").toBe("./rt.log");

      // Secrets
      expect(input.secretsBackend, "secretsBackend from server").toBe("openbao");
      expect(input.secretsFilePath, "secretsFilePath from server").toBe("./rt-secrets.json");
      expect(input.secretsOpenbaoURL, "secretsOpenbaoURL from server").toBe("https://rt-vault.test");
      expect(input.secretsOpenbaoToken, "secretsOpenbaoToken from server").toBe("hvs.rt");

      // Scheduler
      expect(input.schedulerEnabled, "schedulerEnabled from server").toBe(false);
      expect(input.schedulerMemoryEnabled, "schedulerMemoryEnabled from server").toBe(true);

      // Channels
      expect(input.channelSlackEnabled, "channelSlackEnabled from server").toBe(true);
      expect(input.channelSlackBotToken, "channelSlackBotToken from server").toBe("xoxb-rt");
      expect(input.channelSlackAppToken, "channelSlackAppToken from server").toBe("xapp-rt");
    });
  });
});

// ── Sentinel: total editable field count ──────────────────────────────────────
//
// Fails when a field is added to or removed from a canonical array without
// updating the round-trip test coverage.
//
// If you are an AI agent and this test fails, you must:
//   1. Find the new/removed field in its canonical array
//      (EDITABLE_AGENT_FIELDS, EDITABLE_CHANNEL_FIELDS, CAPABILITY_SUBFIELDS,
//      or one of the arrays inside ALL_OTHER_EDITABLE_FIELDS).
//   2. Add/remove it in the mock server response inside the
//      "all groups round-trip" describe block above.
//   3. Add/remove a corresponding expect() assertion in that same waitFor block.
//   4. Update EXPECTED_EDITABLE_FIELD_COUNT below to the new total.

// agent(10) + channel(14) + capabilities(7) + database(4) + memory(5)
// + subagents(2) + graphql(4) + logging(2) + secrets(4) + scheduler(3) = 55
const EXPECTED_EDITABLE_FIELD_COUNT = 55;

describe("SettingsView — editable field count sentinel", () => {
  it(`canonical arrays cover exactly ${EXPECTED_EDITABLE_FIELD_COUNT} editable fields`, () => {
    const total =
      EDITABLE_AGENT_FIELDS.length +
      EDITABLE_CHANNEL_FIELDS.length +
      CAPABILITY_SUBFIELDS.length +
      ALL_OTHER_EDITABLE_FIELDS.length;

    expect(total).toBe(
      EXPECTED_EDITABLE_FIELD_COUNT
    );
    // Si falla, muestra instrucciones detalladas:
    if (total !== EXPECTED_EDITABLE_FIELD_COUNT) {
      console.error(
        `Found ${total} fields across all canonical arrays but expected ${EXPECTED_EDITABLE_FIELD_COUNT}.\n\n` +
        `A config field was added or removed from a canonical array without updating the tests.\n` +
        `If you are an AI agent, you must:\n` +
        `  1. Identify the new/removed field in its canonical array.\n` +
        `  2. Add/remove it in the mock server response in the "all groups round-trip" describe block.\n` +
        `  3. Add/remove the corresponding expect() assertion in that waitFor block.\n` +
        `  4. Update EXPECTED_EDITABLE_FIELD_COUNT in this file to ${total}.`
      );
    }
  });
});
