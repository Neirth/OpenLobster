// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, fireEvent } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import FirstBootWizard from "./FirstBootWizard";

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

vi.mock("../../stores/authStore", () => ({
  getStoredToken: () => null,
  setNeedsAuth: () => {},
}));

const mockClientRequest = vi.hoisted(() => vi.fn());
vi.mock("../../graphql/client", () => ({
  GRAPHQL_ENDPOINT: "/graphql",
  client: { request: mockClientRequest },
}));

const mockFetch = vi.fn();
global.fetch = mockFetch;

const MOCK_MARKETPLACE = [
  {
    id: "zapier",
    name: "Zapier",
    company: "Zapier",
    description: "Connect to 7000+ apps",
    url: "https://mcpserver.zapier.com/mcp",
  },
  {
    id: "linear",
    name: "Linear",
    company: "Linear",
    description: "Manage issues and projects",
    url: "https://mcp.linear.app/mcp",
  },
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
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : (input as URL).toString();
    if (url.includes("graphql") || url === "/graphql") {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({
          data: {
            config: {
              agent: { name: "TestAgent", provider: "ollama", model: "llama3.2:latest" },
              graphql: { baseUrl: "" },
              capabilities: {},
              channelSecrets: {},
            },
          },
        }),
      });
    }
    if (url.includes("marketplace.json")) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => MOCK_MARKETPLACE,
      });
    }
    return Promise.reject(new Error(`Unexpected fetch: ${url}`));
  });
}

async function navigateToStep5(container: HTMLElement) {
  for (let i = 0; i < 5; i++) {
    const nextBtn = container.querySelector(".wizard-btn-primary") as HTMLButtonElement;
    if (nextBtn && !nextBtn.disabled) {
      fireEvent.click(nextBtn);
      await new Promise((r) => setTimeout(r, 50));
    }
  }
}

describe("FirstBootWizard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    setupFetchMock();
    mockClientRequest.mockResolvedValue({ connectMcp: { success: true } });
  });

  describe("FirstBootWizard component", () => {
    it("renders wizard overlay", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        const overlay = container.querySelector(".wizard-overlay");
        expect(overlay).toBeTruthy();
      });
    });

    it("renders stepper with 7 steps", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        const dots = container.querySelectorAll(".wizard-step-dot");
        expect(dots.length).toBe(7);
      });
    });

    it("shows welcome step after loading", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        const welcome = container.querySelector(".wizard-step--welcome");
        expect(welcome).toBeTruthy();
      }, { timeout: 2000 });
    });
  });

  describe("Marketplace MCP selector (step 5)", () => {
    it("shows marketplace grid when navigating to step 5", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        expect(container.querySelector(".wizard-step--welcome")).toBeTruthy();
      }, { timeout: 2000 });

      await navigateToStep5(container);

      await waitFor(() => {
        const marketplace = container.querySelector(".wizard-step--marketplace");
        expect(marketplace).toBeTruthy();
        const grid = container.querySelector(".wizard-marketplace-grid");
        expect(grid).toBeTruthy();
      }, { timeout: 3000 });
    });

    it("displays marketplace servers from fetch", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        expect(container.querySelector(".wizard-step--welcome")).toBeTruthy();
      }, { timeout: 2000 });

      await navigateToStep5(container);

      await waitFor(() => {
        const cards = container.querySelectorAll(".wizard-marketplace-card");
        expect(cards.length).toBeGreaterThanOrEqual(1);
        expect(container.textContent).toContain("Zapier");
        expect(container.textContent).toContain("Linear");
      }, { timeout: 3000 });
    });

    it("shows detail view with name and endpoint inputs when clicking a server", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        expect(container.querySelector(".wizard-step--welcome")).toBeTruthy();
      }, { timeout: 2000 });

      await navigateToStep5(container);

      await waitFor(() => {
        const cards = container.querySelectorAll(".wizard-marketplace-card");
        expect(cards.length).toBeGreaterThanOrEqual(1);
      }, { timeout: 3000 });

      const firstCard = container.querySelector(".wizard-marketplace-card") as HTMLButtonElement;
      fireEvent.click(firstCard);

      await waitFor(() => {
        const detail = container.querySelector(".wizard-marketplace-detail");
        expect(detail).toBeTruthy();
        const form = container.querySelector(".wizard-marketplace-detail__form");
        expect(form).toBeTruthy();
        const inputs = container.querySelectorAll(".wizard-marketplace-detail__form input");
        expect(inputs.length).toBe(2);
        const connectBtn = container.querySelector(".wizard-btn-primary");
        expect(connectBtn).toBeTruthy();
        expect(container.textContent).toContain("marketplace.connect");
      }, { timeout: 1000 });
    });

    it("calls connectMcp with name and url when clicking Conectar", async () => {
      const { container } = renderWithProvider();
      await waitFor(() => {
        expect(container.querySelector(".wizard-step--welcome")).toBeTruthy();
      }, { timeout: 2000 });

      await navigateToStep5(container);

      await waitFor(() => {
        const cards = container.querySelectorAll(".wizard-marketplace-card");
        expect(cards.length).toBeGreaterThanOrEqual(1);
      }, { timeout: 3000 });

      fireEvent.click(container.querySelector(".wizard-marketplace-card") as HTMLButtonElement);

      await waitFor(() => {
        expect(container.querySelector(".wizard-marketplace-detail")).toBeTruthy();
      }, { timeout: 1000 });

      const connectBtn = container.querySelector(".wizard-btn-primary") as HTMLButtonElement;
      fireEvent.click(connectBtn);

      await waitFor(() => {
        expect(mockClientRequest).toHaveBeenCalledWith(
          expect.anything(),
          expect.objectContaining({
            name: "Zapier",
            transport: "http",
            url: "https://mcpserver.zapier.com/mcp",
          }),
        );
      }, { timeout: 2000 });
    });
  });
});
