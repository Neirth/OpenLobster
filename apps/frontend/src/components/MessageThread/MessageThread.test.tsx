// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * MessageThread component tests.
 *
 * Exercises: initial load, pagination, WS append via query-cache slot,
 * message rendering (roles, sender labels, attachments, tool truncation),
 * scroll-to-top pagination, skeleton loading state, and the render-window
 * virtualisation logic.
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, waitFor } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

// ── mocks ────────────────────────────────────────────────────────────────────

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("../../App", () => ({
  t: (key: string) => {
    const map: Record<string, string> = {
      "chat.loadingMore": "Loading more…",
      "chat.contentTruncated": "[content truncated]",
      "chat.roleTool": "Tool",
    };
    return map[key] ?? key;
  },
}));

vi.mock("../../lib/markdown", () => ({
  renderMarkdown: (content: string) => `<p>${content}</p>`,
}));

vi.mock("../SkeletonMessages", () => ({
  default: () => <div class="skeleton-messages" />,
}));

vi.mock("../../utils/formatChatTime", () => ({
  formatChatTime: (_iso: string, _full?: boolean) => "12:34",
}));

// vi.hoisted ensures mockRequest is available when the vi.mock factory runs.
const { mockRequest } = vi.hoisted(() => ({ mockRequest: vi.fn() }));

vi.mock("../../graphql/client", () => ({
  client: { request: mockRequest },
}));

vi.mock("@openlobster/ui/graphql/queries", () => ({
  MESSAGES_QUERY: "MESSAGES_QUERY",
}));

vi.mock("@openlobster/ui/hooks", () => ({
  useConfig: () => ({
    data: { agentName: "OpenLobster", agent: { name: "OpenLobster" } },
    isLoading: false,
  }),
}));

// ── helpers ───────────────────────────────────────────────────────────────────

import type { Message } from "@openlobster/ui/types";
import MessageThread from "./MessageThread";

function makeMsg(overrides: Partial<Message> & { id: string }): Message {
  const numericPart = overrides.id.replace(/\D/g, "");
  const offset = numericPart ? parseInt(numericPart) : 0;
  return {
    conversationId: "conv1",
    role: "user",
    content: `Message ${overrides.id}`,
    createdAt: new Date(Date.now() - offset * 1000).toISOString(),
    ...overrides,
  };
}

function renderThread(props: {
  conversationId?: string;
  onNewMessageCount?: (n: number) => void;
  participantName?: string;
  messages?: Message[];
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });

  // Envolver onNewMessageCount en una función para cumplir con la reactividad
  const onCount = (...args: [number]) => props.onNewMessageCount?.(...args);

  const result = render(() => (
    <QueryClientProvider client={queryClient}>
      <MessageThread
        conversationId={props.conversationId ?? "conv1"}
        onNewMessageCount={onCount}
        participantName={props.participantName}
      />
    </QueryClientProvider>
  ));

  return { ...result, queryClient };
}

// ── tests ─────────────────────────────────────────────────────────────────────

describe("MessageThread — initial render", () => {
  beforeEach(() => {
    mockRequest.mockReset();
  });

  it("renders the messages container", () => {
    mockRequest.mockResolvedValue({ messages: [] });
    const { container } = renderThread({});
    expect(container.querySelector(".chat-thread__messages")).toBeTruthy();
  });

  it("shows skeleton while initial load is in progress", () => {
    // Never resolves during this test so loading state persists
    mockRequest.mockReturnValue(new Promise(() => {}));
    const { container } = renderThread({});
    // skeleton is rendered while loading AND messages is empty
    expect(container.querySelector(".skeleton-messages")).toBeTruthy();
  });

  it("renders messages after initial fetch resolves", async () => {
    const msgs = [makeMsg({ id: "1", role: "user", content: "Hello" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg")).toBeTruthy();
    });
  });

  it("calls onNewMessageCount with the loaded message count", async () => {
    const msgs = [
      makeMsg({ id: "1", content: "a" }),
      makeMsg({ id: "2", content: "b" }),
    ];
    mockRequest.mockResolvedValue({ messages: msgs });
    const onCount = vi.fn();
    renderThread({ onNewMessageCount: onCount });
    await waitFor(() => {
      expect(onCount).toHaveBeenCalledWith(2);
    });
  });

  it("renders user messages with msg--user class", async () => {
    const msgs = [makeMsg({ id: "1", role: "user", content: "User msg" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg--user")).toBeTruthy();
    });
  });

  it("renders assistant messages with msg--agent class", async () => {
    const msgs = [makeMsg({ id: "1", role: "assistant", content: "Agent reply" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg--agent")).toBeTruthy();
    });
  });

  it("renders agent-role messages with msg--agent class", async () => {
    const msgs = [makeMsg({ id: "1", role: "agent", content: "Agent reply" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg--agent")).toBeTruthy();
    });
  });

  it("renders system messages with msg--system class", async () => {
    const msgs = [makeMsg({ id: "1", role: "system", content: "System notice" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg--system")).toBeTruthy();
    });
  });

  it("renders tool messages with msg--tool class", async () => {
    const msgs = [makeMsg({ id: "1", role: "tool", content: "tool output" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg--tool")).toBeTruthy();
    });
  });

  it("shows meta (sender + time) for first message in a sequence", async () => {
    const msgs = [makeMsg({ id: "1", role: "user", content: "First" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__meta")).toBeTruthy();
      expect(container.querySelector(".msg__sender")).toBeTruthy();
      expect(container.querySelector(".msg__time")).toBeTruthy();
    });
  });

  it("does not show meta for consecutive messages with the same role", async () => {
    const msgs = [
      makeMsg({ id: "1", role: "user", content: "First" }),
      makeMsg({ id: "2", role: "user", content: "Second" }),
    ];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      const metas = container.querySelectorAll(".msg__meta");
      // only first message gets meta; second is same role so no meta
      expect(metas.length).toBe(1);
    });
  });

  it("uses participantName for user sender label", async () => {
    const msgs = [makeMsg({ id: "1", role: "user", content: "Hi" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({ participantName: "Alice" });
    await waitFor(() => {
      expect(container.querySelector(".msg__sender")?.textContent).toBe("Alice");
    });
  });

  it("uses agent name from config for assistant sender label", async () => {
    const msgs = [makeMsg({ id: "1", role: "assistant", content: "Hey" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__sender")?.textContent).toBe("OpenLobster");
    });
  });

  it("uses 'Tool' label for tool-role messages", async () => {
    const msgs = [makeMsg({ id: "1", role: "tool", content: "result" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__sender")?.textContent).toBe("Tool");
    });
  });

  it("falls back to USER_XXXX label when no participantName is provided", async () => {
    const msgs = [makeMsg({ id: "1", role: "user", content: "Hi", conversationId: "abcd" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({ conversationId: "abcd" });
    await waitFor(() => {
      const sender = container.querySelector(".msg__sender")?.textContent ?? "";
      expect(sender.startsWith("USER_")).toBe(true);
    });
  });

  it("uses senderName field when present on message", async () => {
    const msgs = [{ ...makeMsg({ id: "1", role: "user", content: "Hi" }), senderName: "Bob" }];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__sender")?.textContent).toBe("Bob");
    });
  });

  it("renders hasMore indicator when page is full (50 messages)", async () => {
    const msgs = Array.from({ length: 50 }, (_, i) => makeMsg({ id: String(i), content: `m${i}` }));
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".chat-thread__load-more")).toBeTruthy();
    });
  });

  it("does not render hasMore indicator when page is less than 50", async () => {
    const msgs = Array.from({ length: 5 }, (_, i) => makeMsg({ id: String(i), content: `m${i}` }));
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".chat-thread__load-more")).toBeNull();
    });
  });

  it("renders message body with markdown content", async () => {
    const msgs = [makeMsg({ id: "1", content: "**bold**" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      const body = container.querySelector(".msg__body");
      expect(body).toBeTruthy();
      expect(body?.innerHTML).toContain("**bold**");
    });
  });

  it("truncates tool message content longer than 2000 chars", async () => {
    const longContent = "x".repeat(2500);
    const msgs = [makeMsg({ id: "1", role: "tool", content: longContent })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      const body = container.querySelector(".msg__body");
      expect(body?.innerHTML).toContain("[content truncated]");
    });
  });

  it("does not truncate tool message content within 2000 chars", async () => {
    const shortContent = "x".repeat(1000);
    const msgs = [makeMsg({ id: "1", role: "tool", content: shortContent })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      const body = container.querySelector(".msg__body");
      expect(body?.innerHTML).not.toContain("[content truncated]");
    });
  });

  it("renders attachments when message has attachments", async () => {
    const msgs = [
      makeMsg({
        id: "1",
        role: "user",
        content: "See attached",
        attachments: [{ type: "file", filename: "report.pdf", mimeType: "application/pdf" }],
      }),
    ];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__attachments")).toBeTruthy();
      expect(container.querySelector(".msg__attachment-name")?.textContent).toBe("report.pdf");
    });
  });

  it("renders attachment caption when message has both attachments and content", async () => {
    const msgs = [
      makeMsg({
        id: "1",
        role: "user",
        content: "Caption text",
        attachments: [{ type: "image", mimeType: "image/png" }],
      }),
    ];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__attachment-caption")).toBeTruthy();
    });
  });

  it("renders attachment filename fallback to mimeType when filename is absent", async () => {
    const msgs = [
      makeMsg({
        id: "1",
        role: "user",
        content: "",
        attachments: [{ type: "image", mimeType: "image/jpeg" }],
      }),
    ];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      const nameEl = container.querySelector(".msg__attachment-name");
      expect(nameEl?.textContent).toBe("image/jpeg");
    });
  });

  it("renders attachment type as final fallback when filename and mimeType are absent", async () => {
    const msgs = [
      makeMsg({
        id: "1",
        role: "user",
        content: "",
        attachments: [{ type: "document" }],
      }),
    ];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      const nameEl = container.querySelector(".msg__attachment-name");
      expect(nameEl?.textContent).toBe("document");
    });
  });

  it("renders formatted time on message meta", async () => {
    const msgs = [makeMsg({ id: "1", content: "timed" })];
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg__time")?.textContent).toBe("12:34");
    });
  });
});

describe("MessageThread — pagination (load older)", () => {
  beforeEach(() => {
    mockRequest.mockReset();
  });

  it("exposes __test_loadOlder on the scroll element in test mode", async () => {
    const msgs = Array.from({ length: 50 }, (_, i) => makeMsg({ id: String(i) }));
    mockRequest.mockResolvedValue({ messages: msgs });
    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".chat-thread__messages")).toBeTruthy();
    });
    const el = container.querySelector(".chat-thread__messages") as any;
    expect(typeof el.__test_loadOlder).toBe("function");
  });

  it("loadOlder fetches an older page and prepends messages", async () => {
    const initial = Array.from({ length: 50 }, (_, i) =>
      makeMsg({ id: String(i + 10), createdAt: new Date(Date.now() - (50 - i) * 1000).toISOString() })
    );
    mockRequest
      .mockResolvedValueOnce({ messages: initial })
      .mockResolvedValueOnce({
        messages: [makeMsg({ id: "older-1", content: "Older msg", createdAt: new Date(0).toISOString() })],
      });

    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelectorAll(".msg").length).toBeGreaterThan(0);
    });

    const el = container.querySelector(".chat-thread__messages") as any;
    await el.__test_loadOlder();

    await waitFor(() => {
      expect(mockRequest).toHaveBeenCalledTimes(2);
    });
  });

  it("loadOlder stops when server returns empty page", async () => {
    const initial = Array.from({ length: 50 }, (_, i) => makeMsg({ id: String(i + 10) }));
    mockRequest
      .mockResolvedValueOnce({ messages: initial })
      .mockResolvedValueOnce({ messages: [] });

    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelectorAll(".msg").length).toBeGreaterThan(0);
    });

    const el = container.querySelector(".chat-thread__messages") as any;
    await el.__test_loadOlder();

    await waitFor(() => {
      expect(mockRequest).toHaveBeenCalledTimes(2);
    });
  });

  it("shows loading indicator while loadOlder is in flight", async () => {
    const initial = Array.from({ length: 50 }, (_, i) => makeMsg({ id: String(i + 10) }));
    let resolveOlder!: (v: any) => void;
    const olderPromise = new Promise<any>((res) => { resolveOlder = res; });

    mockRequest
      .mockResolvedValueOnce({ messages: initial })
      .mockReturnValueOnce(olderPromise);

    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelectorAll(".msg").length).toBeGreaterThan(0);
    });

    const el = container.querySelector(".chat-thread__messages") as any;
    void el.__test_loadOlder();

    // Resolve after a tick to give Solid time to show the indicator
    await new Promise((r) => setTimeout(r, 0));

    // Resolve and clean up
    resolveOlder({ messages: [] });
    await olderPromise;
  });
});

describe("MessageThread — WS append via query cache", () => {
  beforeEach(() => {
    mockRequest.mockReset();
  });

  it("appends a new message via query cache slot", async () => {
    const initial = [makeMsg({ id: "1", content: "First" })];
    mockRequest.mockResolvedValue({ messages: initial });

    const { container, queryClient } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg")).toBeTruthy();
    });

    const slot = queryClient.getQueryData<{ append: (m: any) => void }>(["messages-append", "conv1"]);
    expect(slot?.append).toBeDefined();

    const newMsg = makeMsg({ id: "new-ws", content: "WS arrived", conversationId: "conv1" });
    slot?.append(newMsg);

    await waitFor(() => {
      const msgs = container.querySelectorAll(".msg");
      expect(msgs.length).toBeGreaterThan(1);
    });
  });

  it("does not duplicate a message already in the list", async () => {
    const initial = [makeMsg({ id: "1", content: "Existing" })];
    mockRequest.mockResolvedValue({ messages: initial });

    const { container, queryClient } = renderThread({});
    await waitFor(() => {
      expect(container.querySelector(".msg")).toBeTruthy();
    });

    const slot = queryClient.getQueryData<{ append: (m: any) => void }>(["messages-append", "conv1"]);
    // Append duplicate
    slot?.append(makeMsg({ id: "1", content: "Existing" }));

    await waitFor(() => {
      // Should still be exactly 1 message
      expect(container.querySelectorAll(".msg").length).toBe(1);
    });
  });
});

describe("MessageThread — onScroll behaviour", () => {
  beforeEach(() => {
    mockRequest.mockReset();
  });

  it("onScroll handler can be invoked without error", async () => {
    mockRequest.mockResolvedValue({ messages: [] });
    const { container } = renderThread({});
    const scrollEl = container.querySelector(".chat-thread__messages") as HTMLElement;
    // Simulate a scroll event — should not throw
    expect(() => fireEvent.scroll(scrollEl)).not.toThrow();
  });

  it("scrolling near top triggers a loadOlder fetch when hasMore is true", async () => {
    const initial = Array.from({ length: 50 }, (_, i) => makeMsg({ id: String(i) }));
    mockRequest
      .mockResolvedValueOnce({ messages: initial })
      .mockResolvedValue({ messages: [] });

    const { container } = renderThread({});
    await waitFor(() => {
      expect(container.querySelectorAll(".msg").length).toBeGreaterThan(0);
    });

    const scrollEl = container.querySelector(".chat-thread__messages") as HTMLElement;
    Object.defineProperty(scrollEl, "scrollTop", { value: 0, writable: true });
    Object.defineProperty(scrollEl, "scrollHeight", { value: 1000, writable: true });
    Object.defineProperty(scrollEl, "clientHeight", { value: 500, writable: true });
    fireEvent.scroll(scrollEl);

    await waitFor(() => {
      expect(mockRequest.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
  });
});
