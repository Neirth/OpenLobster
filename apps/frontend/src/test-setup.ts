// Test setup: polyfills and global test helpers for happy-dom
// Polyfill a minimal Popover API used by ContextMenu component.

if (typeof window !== "undefined") {
  // Cast to `any` to avoid conflicts with lib.dom.d.ts, which in modern TypeScript
  // declares showPopover/hidePopover as non-optional on HTMLElement. The polyfill
  // only installs the methods when happy-dom omits them at runtime.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const proto = HTMLElement.prototype as any;

  type PopoverEl = HTMLElement & { __popoverOpen?: boolean };

  if (!proto.showPopover) {
    proto.showPopover = function (this: PopoverEl) {
      try {
        this.__popoverOpen = true;
        if (this.dataset) this.dataset["popoverOpen"] = "true";
      } catch {
        // ignore
      }
    };
  }

  if (!proto.hidePopover) {
    proto.hidePopover = function (this: PopoverEl) {
      try {
        this.__popoverOpen = false;
        if (this.dataset) this.dataset["popoverOpen"] = "false";
      } catch {
        // ignore
      }
    };
  }

  // Patch matches to understand ':popover-open' pseudo-class.
  const originalMatches = HTMLElement.prototype.matches;
  proto.matches = function (this: PopoverEl, selector: string): boolean {
    if (selector === ":popover-open") {
      try {
        if (this.__popoverOpen !== undefined) return !!this.__popoverOpen;
        if (this.dataset && this.dataset["popoverOpen"] !== undefined) {
          return this.dataset["popoverOpen"] === "true";
        }
        return false;
      } catch {
        return false;
      }
    }
    return originalMatches.call(this, selector);
  };

  // Polyfill localStorage if it's missing or incomplete in happy-dom
  if (!window.localStorage || typeof window.localStorage.clear !== "function") {
    const store = new Map<string, string>();
    const localStorageShim = {
      getItem: (key: string) => store.get(key) || null,
      setItem: (key: string, value: string) => store.set(key, value),
      removeItem: (key: string) => store.delete(key),
      clear: () => store.clear(),
      key: (index: number) => Array.from(store.keys())[index] || null,
      get length() {
        return store.size;
      },
    };
    Object.defineProperty(window, "localStorage", {
      value: localStorageShim,
      writable: true,
      configurable: true,
    });
  }
}

import { vi } from "vitest";

// Common global mocks
vi.mock("@/graphql/config", () => ({
  client: {
    request: vi.fn(async (query: string) => {
      const q = query.toLowerCase();
      // Return safe defaults based on common query names to avoid 'undefined' or empty list failures
      if (q.includes("tasks")) return { tasks: [
        { id: "1", prompt: "Morning brief", schedule: "0 8 * * *", enabled: true, status: "running" },
        { id: "2", prompt: "Cleanup logs", schedule: "0 0 * * *", enabled: true, status: "pending" },
        { id: "3", prompt: "One-time backup", schedule: "2026-12-31T23:59:00Z", enabled: true, status: "pending" }
      ] };
      if (q.includes("skills")) return { skills: [
        { id: "s1", name: "Python", description: "Scripting", enabled: true, status: "active" },
        { id: "s2", name: "Javascript", description: "Frontend", enabled: true, status: "active" }
      ]};
      if (q.includes("metric") || q.includes("dashboard")) return { metrics: {
        health: "OK", sessionCount: 5, mcpServerCount: 2, messagesReceived: 100, messagesSent: 50, uptime: "24h"
      }};

      if (q.includes("agent")) return { agent: { name: "OpenLobster", version: "0.4.1" } };
      if (q.includes("config")) return { config: { wizardCompleted: true, theme: "dark" } };
      if (q.includes("mcp") || q.includes("tool")) return { 
        mcpServers: [{ id: "m1", name: "Filesystem", status: "running" }],
        mcpTools: [{ id: "t1", name: "read_file", serverId: "m1" }]
      };
      if (q.includes("message")) return { messages: [{ id: "msg1", role: "user", content: "Hello" }] };
      if (q.includes("conversation")) return { conversations: [{ id: "c1", title: "New Chat", lastMessageAt: new Date().toISOString() }] };
      if (q.includes("channel")) return { channels: [{ id: "ch1", name: "Discord", active: true }] };
      if (q.includes("memory")) return { memoryEntries: [{ id: "mem1", content: "Fact 1", category: "personal" }] };
      
      return { data: {} };
    }),
  },
  GRAPHQL_ENDPOINT: "/graphql",
}));

vi.mock("@/graphql/config", () => {
  const mockClient = {
    request: vi.fn(async (query: string) => {
      if (query.includes("tasks")) return { tasks: [] };
      if (query.includes("agent")) return { agent: { name: "OpenLobster", version: "0.4.1" } };
      return { data: {} };
    }),
  };
  return {
    client: mockClient,
    createGraphqlClient: vi.fn(() => mockClient),
  };
});

export {};
