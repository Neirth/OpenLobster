// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";
import { render } from "@solidjs/testing-library";

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("@tanstack/solid-query", () => ({
  createMutation: () => ({ mutate: vi.fn(), isPending: false }),
  useQueryClient: () => ({
    invalidateQueries: vi.fn(),
    setQueryData: vi.fn(),
    getQueryData: vi.fn(),
  }),
}));

vi.mock("@openlobster/ui/graphql/mutations", () => ({
  SEND_MESSAGE_MUTATION: "SEND_MESSAGE_MUTATION",
  DELETE_USER_MUTATION: "DELETE_USER_MUTATION",
}));

vi.mock("@openlobster/ui/graphql/queries", () => ({
  MESSAGES_QUERY: "MESSAGES_QUERY",
}));

vi.mock("@openlobster/ui/hooks", () => ({
  useConversations: () => ({ data: [], isLoading: false, error: null }),
  useSubscriptions: () => ({
    isConnected: () => false,
    connect: () => {},
    disconnect: () => {},
    sendResponse: () => {},
  }),
  useConfig: () => ({ data: undefined, isLoading: false }),
}));

vi.mock("../../graphql/client", () => ({
  GRAPHQL_ENDPOINT: "/graphql",
  client: { request: vi.fn(() => Promise.resolve({})) },
}));

vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

import ChatView from "./ChatView";

describe("ChatView Component — empty conversations state", () => {
  it("shows chat-empty when conversations list is empty", () => {
    const { container } = render(() => <ChatView />);
    expect(container.querySelector(".chat-empty")).toBeTruthy();
  });

  it("does not render chat-layout when conversations list is empty", () => {
    const { container } = render(() => <ChatView />);
    expect(container.querySelector(".chat-layout")).toBeNull();
  });
});
