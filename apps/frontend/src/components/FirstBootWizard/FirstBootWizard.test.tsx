// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, fireEvent } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import FirstBootWizard from "./FirstBootWizard";

vi.mock("../../App", () => ({
  t: (key: string) => key,
  getStoredToken: () => "mock-token",
}));

const mockClientRequest = vi.hoisted(() => vi.fn());
vi.mock("../../graphql/client", () => ({
  client: { request: mockClientRequest },
}));

vi.mock("../../graphql/config", () => ({
  GRAPHQL_ENDPOINT: "/graphql",
}));


const mockFetch = vi.fn();
global.fetch = mockFetch;

const MOCK_PLUGINS = [
  {
    id: "ollama",
    name: "Ollama",
    pluginType: "ai",
    available: true,
    schemaJson: JSON.stringify({
      properties: {
        model: { type: "string", title: "Model Name" }
      }
    })
  },
  {
    id: "telegram",
    name: "Telegram Messenger",
    pluginType: "messaging",
    available: true,
    schemaJson: JSON.stringify({
      properties: {
        token: { type: "string", title: "Bot Token" }
      }
    })
  },
  {
    id: "json-memory",
    name: "Local Memory",
    pluginType: "memory",
    available: true,
    schemaJson: "{}"
  }
];

const MOCK_MARKETPLACE = [
  {
    id: "zapier",
    name: "Zapier",
    company: "Zapier",
    description: "Connect to 7000+ apps",
    url: "https://mcpserver.zapier.com/mcp",
  }
];

const renderWithProvider = (onComplete = () => {}) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>
      <FirstBootWizard onComplete={onComplete} />
    </QueryClientProvider>
  ));
};

function setupFetchMock() {
  mockFetch.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
    const body = init?.body ? JSON.parse(init.body as string) : {};
    const query = body.query || "";

    if (query.includes("MARKETPLACE_QUERY") || query.includes("marketplaceServers")) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ data: { marketplaceServers: MOCK_MARKETPLACE } }),
      });
    }

    if (query.includes("PLUGINS_QUERY") || query.includes("plugins {")) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ data: { plugins: MOCK_PLUGINS } }),
      });
    }

    if (query.includes("CONFIG_QUERY") || query.includes("config {")) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({
          data: {
            config: {
              agent: { name: "TestAgent", model: "llama3.2:latest" },
              plugins: { defaultAi: "ollama" }
            },
          },
        }),
      });
    }

    return Promise.resolve({
      ok: true,
      status: 200,
      json: async () => ({ data: { success: true } }),
    });
  });
}

async function navigateToStep(container: HTMLElement, targetStep: number) {
  for (let i = 0; i < targetStep; i++) {
    const nextBtn = Array.from(container.querySelectorAll(".wizard-btn-primary")).find(
      (b) => b.textContent?.includes("wizard.next")
    ) as HTMLButtonElement;
    if (nextBtn && !nextBtn.disabled) {
      fireEvent.click(nextBtn);
      await new Promise((r) => setTimeout(r, 10)); // tiny wait for state updates
    }
  }
}

describe("FirstBootWizard (Categorized)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    setupFetchMock();
    mockClientRequest.mockResolvedValue({ connectMcp: { success: true } });
  });

  it("renders correctly and navigates to Identity step", async () => {
    const { getByText, container } = renderWithProvider();
    await waitFor(() => expect(getByText("wizard.welcome.title")).toBeTruthy());

    const nextBtn = container.querySelector(".wizard-btn-primary") as HTMLButtonElement;
    fireEvent.click(nextBtn);

    await waitFor(() => expect(getByText("wizard.agentConfig.title")).toBeTruthy());
  });

  it("handles dynamic AI brain configuration in Step 2", async () => {
    const { getByText, container } = renderWithProvider();
    await waitFor(() => expect(getByText("wizard.welcome.title")).toBeTruthy());
    
    await navigateToStep(container, 2);
    
    await waitFor(() => expect(getByText("wizard.aiProvider.title")).toBeTruthy());
    // Verify provider dropdown has Ollama
    const select = container.querySelector("select") as HTMLSelectElement;
    expect(select.innerHTML).toContain("Ollama");
  });

  it("handles dynamic messaging channels in Step 6", async () => {
    const { getByText, findByText, container } = renderWithProvider();
    await findByText("wizard.welcome.title");
    
    await navigateToStep(container, 6);
    
    await findByText("wizard.connectivity.title");
    // Should see dynamic Telegram plugin
    const pluginLabel = await findByText("Telegram Messenger");
    expect(pluginLabel).toBeTruthy();
    
    // Toggle Telegram - find the checkbox in the same row
    const row = pluginLabel.closest(".wizard-channel-row");
    const checkbox = row?.querySelector("input[type='checkbox']") as HTMLInputElement;
    expect(checkbox).toBeTruthy();
    
    fireEvent.click(checkbox);
    
    // Should show dynamic schema field (Bot Token) from MOCK_PLUGINS
    // Use a longer timeout or just findByText which defaults to 1000ms
    const fieldLabel = await findByText("Bot Token");
    expect(fieldLabel).toBeTruthy();
  });

  it("completes the wizard flow at Step 7", async () => {
    const onComplete = vi.fn();
    const { getByText, container } = renderWithProvider(onComplete);
    
    await navigateToStep(container, 7);
    
    await waitFor(() => expect(getByText("wizard.done.title")).toBeTruthy());
    
    const finishBtn = Array.from(container.querySelectorAll(".wizard-btn-primary")).find(
      (b) => b.textContent?.includes("wizard.finish")
    ) as HTMLButtonElement;
    
    fireEvent.click(finishBtn);
    
    await waitFor(() => expect(onComplete).toHaveBeenCalled());
    expect(localStorage.getItem("openlobster_wizard_completed")).toBe("true");
  });
});
