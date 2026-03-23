// Copyright (c) OpenLobster contributors. See LICENSE for details.
// Removed unused eslint-disable

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

// Must be hoisted before imports
const mockNeedsAuth = vi.hoisted(() => vi.fn(() => false));
const mockSetNeedsAuth = vi.hoisted(() => vi.fn());
const mockGetStoredToken = vi.hoisted(() => vi.fn((): string | null => null));
const mockSetOpenPairingRequestHandler = vi.hoisted(() => vi.fn());
const mockClientRequest = vi.hoisted(() => vi.fn(() => Promise.resolve({})));
const mockSubscribe = vi.hoisted(() => vi.fn(() => ({ disconnect: vi.fn() })));
const mockCreateSubscriptionManager = vi.hoisted(() =>
  vi.fn(() => ({ subscribe: mockSubscribe }))
);

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

vi.mock("../../stores/authStore", () => ({
  needsAuth: mockNeedsAuth,
  setNeedsAuth: mockSetNeedsAuth,
  getStoredToken: mockGetStoredToken,
}));

vi.mock("../../stores/wsStore", () => ({
  useWsConnection: () => ({
    isConnected: () => false,
    setConnected: vi.fn(),
  }),
}));

vi.mock("../../stores/pairingStore", () => ({
  pendingPairingsQueue: () => [],
  setPendingPairingsQueue: vi.fn(),
  setOpenPairingRequestHandler: mockSetOpenPairingRequestHandler,
}));

vi.mock("@openlobster/ui/hooks", () => ({
  createSubscriptionManager: mockCreateSubscriptionManager,
}));

vi.mock("../../graphql/client", () => ({
  client: { request: mockClientRequest },
  GRAPHQL_ENDPOINT: "/graphql",
}));

vi.mock("../AccessTokenModal/AccessTokenModal", () => ({
  default: () => <div class="access-token-modal-mock">AccessTokenModal</div>,
}));

vi.mock("../PairingModal/PairingModal", () => ({
  default: (props: any) => (
    <div class="pairing-modal-mock" data-open={String(props.isOpen)}>
      PairingModal
    </div>
  ),
}));

import AuthModals from "./AuthModals";

function renderAuthModals(children: any = <div class="child-content">Child</div>) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>
      <AuthModals>{children}</AuthModals>
    </QueryClientProvider>
  ));
}

describe("AuthModals Component", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNeedsAuth.mockReturnValue(false);
    mockGetStoredToken.mockReturnValue(null);
    mockClientRequest.mockResolvedValue({});
    mockSubscribe.mockReturnValue({ disconnect: vi.fn() });
    mockCreateSubscriptionManager.mockReturnValue({ subscribe: mockSubscribe });

    // Mock fetch for pending pairings
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: async () => ({ data: { pendingPairings: [] } }),
        })
      )
    );
  });

  it("renders children", () => {
    const { getByText } = renderAuthModals(<div>Child content</div>);
    expect(getByText("Child content")).toBeTruthy();
  });

  it("renders multiple children correctly", () => {
    const { getByText } = renderAuthModals(
      <>
        <div>First</div>
        <div>Second</div>
      </>
    );
    expect(getByText("First")).toBeTruthy();
    expect(getByText("Second")).toBeTruthy();
  });

  it("does not render AccessTokenModal when needsAuth is false", () => {
    mockNeedsAuth.mockReturnValue(false);
    const { container } = renderAuthModals();
    expect(container.querySelector(".access-token-modal-mock")).toBeNull();
  });

  it("renders AccessTokenModal when needsAuth is true", () => {
    mockNeedsAuth.mockReturnValue(true);
    const { container } = renderAuthModals();
    expect(container.querySelector(".access-token-modal-mock")).toBeTruthy();
  });

  it("renders PairingModal component", () => {
    const { container } = renderAuthModals();
    expect(container.querySelector(".pairing-modal-mock")).toBeTruthy();
  });

  it("PairingModal is closed by default (no active request)", () => {
    const { container } = renderAuthModals();
    const pairingModal = container.querySelector(".pairing-modal-mock");
    expect(pairingModal?.getAttribute("data-open")).toBe("false");
  });

  it("calls createSubscriptionManager on mount", () => {
    renderAuthModals();
    expect(mockCreateSubscriptionManager).toHaveBeenCalled();
  });

  it("calls subscribe to start listening for events", () => {
    renderAuthModals();
    expect(mockSubscribe).toHaveBeenCalled();
  });

  it("calls setOpenPairingRequestHandler on mount", () => {
    renderAuthModals();
    expect(mockSetOpenPairingRequestHandler).toHaveBeenCalled();
  });

  it("calls client.request to probe the backend on mount", () => {
    renderAuthModals();
    expect(mockClientRequest).toHaveBeenCalledWith("{ __typename }");
  });

  it("does not show dev connection indicator in test (DEV is false)", () => {
    // In test environment import.meta.env.DEV is false
    const { container } = renderAuthModals();
    // The dev indicator is conditional on import.meta.env.DEV
    // In vitest test mode this will be false by default
    // Just verify the component renders without error
    expect(container.querySelector(".child-content")).toBeTruthy();
  });

  it("subscribe handler options include onPairingRequest", () => {
    renderAuthModals();
    const subscribeCall = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg = subscribeCall?.[0];
    expect(typeof subscribeArg?.onPairingRequest).toBe("function");
  });

  it("subscribe handler options include onConnected", () => {
    renderAuthModals();
    const subscribeCall2 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg2 = subscribeCall2?.[0];
    expect(typeof subscribeArg2?.onConnected).toBe("function");
  });

  it("subscribe handler options include onDisconnected", () => {
    renderAuthModals();
    const subscribeCall3 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg3 = subscribeCall3?.[0];
    expect(typeof subscribeArg3?.onDisconnected).toBe("function");
  });

  it("subscribe handler options include onError", () => {
    renderAuthModals();
    const subscribeCall4 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg4 = subscribeCall4?.[0];
    expect(typeof subscribeArg4?.onError).toBe("function");
  });

  it("onError handler does not throw when called", () => {
    renderAuthModals();
    const subscribeCall5 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg5 = subscribeCall5?.[0];
    expect(() => subscribeArg5?.onError(new Error("test"))).not.toThrow();
  });

  it("disconnect is called on unmount", () => {
    const mockDisconnect = vi.fn();
    mockSubscribe.mockReturnValue({ disconnect: mockDisconnect });
    const { unmount } = renderAuthModals();
    unmount();
    expect(mockDisconnect).toHaveBeenCalled();
  });

  it("fetch is called when onConnected fires", async () => {
    renderAuthModals();
    const subscribeCall6 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg6 = subscribeCall6?.[0];
    await subscribeArg6?.onConnected?.();
    expect(vi.mocked(fetch)).toHaveBeenCalled();
  });

  it("sends Authorization header when token is stored", async () => {
    mockGetStoredToken.mockReturnValue("test-token");
    renderAuthModals();
    const subscribeCall7 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg7 = subscribeCall7?.[0];
    await subscribeArg7?.onConnected?.();
    const fetchCalls = vi.mocked(fetch).mock.calls;
    const lastCall = fetchCalls[fetchCalls.length - 1];
    const options = lastCall?.[1] as RequestInit;
    expect((options?.headers as Record<string, string>)?.Authorization).toBe("Bearer test-token");
  });

  it("does not send Authorization header when no token", async () => {
    mockGetStoredToken.mockReturnValue(null);
    renderAuthModals();
    const subscribeCall8 = mockSubscribe.mock.calls[0] as unknown as [any] | undefined;
    const subscribeArg8 = subscribeCall8?.[0];
    await subscribeArg8?.onConnected?.();
    const fetchCalls = vi.mocked(fetch).mock.calls;
    const lastCall = fetchCalls[fetchCalls.length - 1];
    const options = lastCall?.[1] as RequestInit;
    expect((options?.headers as Record<string, string>)?.Authorization).toBeUndefined();
  });
});
