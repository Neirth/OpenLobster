// Copyright (c) OpenLobster contributors. See LICENSE for details.
// Removed unused eslint-disable

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

const mockClientRequest = vi.hoisted(() => vi.fn());
vi.mock("../../graphql/client", () => ({
  client: { request: mockClientRequest },
}));

vi.mock("../../components/Modal", () => ({
  default: (props: any) => (
    <div>
      {props.isOpen && (
        <div class="modal-overlay">
          <div class="modal-box">
            <h3 class="modal-title">{props.title}</h3>
            <button class="modal-close" onClick={() => props.onClose()} />
            <div class="modal-content">{props.children}</div>
          </div>
        </div>
      )}
    </div>
  ),
}));

import PairingModal from "./PairingModal";

const makeRequest = (overrides = {}) => ({
  requestID: "req-001",
  code: "ABC123",
  channelID: "ch-001",
  channelType: "telegram",
  displayName: "John Doe",
  timestamp: "2026-01-01T00:00:00Z",
  ...overrides,
});

function renderModal(props: Parameters<typeof PairingModal>[0]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>
      <PairingModal {...props} />
    </QueryClientProvider>
  ));
}

describe("PairingModal Component", () => {
  beforeEach(() => {
    mockClientRequest.mockResolvedValue({ users: [] });
    vi.clearAllMocks();
  });

  it("does not render modal content when isOpen is false", () => {
    const { container } = renderModal({
      isOpen: false,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: null,
    });
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("renders modal overlay when isOpen is true with a request", () => {
    mockClientRequest.mockResolvedValue({ users: [] });
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("renders pairing title via t()", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(getByText("pairing.title")).toBeTruthy();
  });

  it("renders channel type", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ channelType: "discord" }),
    });
    expect(getByText("discord")).toBeTruthy();
  });

  it("renders display name", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ displayName: "Jane Smith" }),
    });
    expect(getByText("Jane Smith")).toBeTruthy();
  });

  it("renders pairing code", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ code: "XYZ789" }),
    });
    expect(getByText("XYZ789")).toBeTruthy();
  });

  it("renders deny button", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(container.querySelector(".pm-btn--deny")).toBeTruthy();
  });

  it("renders approve button", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(container.querySelector(".pm-btn--approve")).toBeTruthy();
  });

  it("approve button is disabled in existing mode when no user is selected", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    const approveBtn = container.querySelector(".pm-btn--approve") as HTMLButtonElement;
    expect(approveBtn.disabled).toBe(true);
  });

  it("renders mode tabs", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(container.querySelectorAll(".pm-tab").length).toBe(2);
  });

  it("existing tab is active by default", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    const activeTab = container.querySelector(".pm-tab--active");
    expect(activeTab?.textContent).toContain("pairing.existingUser");
  });

  it("clicking new user tab makes it active", () => {
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(getByText("pairing.newUser"));
    const activeTab = container.querySelector(".pm-tab--active");
    expect(activeTab?.textContent).toContain("pairing.newUser");
  });

  it("in new user mode, approve button is enabled", () => {
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(getByText("pairing.newUser"));
    const approveBtn = container.querySelector(".pm-btn--approve") as HTMLButtonElement;
    expect(approveBtn.disabled).toBe(false);
  });

  it("in new user mode renders display name input", () => {
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(getByText("pairing.newUser"));
    expect(container.querySelector(".pm-input")).toBeTruthy();
  });

  it("in new user mode, typing updates display name input", () => {
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ displayName: "Original" }),
    });
    fireEvent.click(getByText("pairing.newUser"));
    const input = container.querySelector(".pm-input") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "New Name" } });
    expect(input.value).toBe("New Name");
  });

  it("clicking deny calls onDeny with requestID", () => {
    const onDeny = vi.fn();
    const onClose = vi.fn();
    const { container } = renderModal({
      isOpen: true,
      onClose,
      onApprove: vi.fn(),
      onDeny,
      request: makeRequest({ requestID: "req-999" }),
    });
    fireEvent.click(container.querySelector(".pm-btn--deny") as HTMLElement);
    expect(onDeny).toHaveBeenCalledWith("req-999");
  });

  it("clicking deny calls onClose", () => {
    const onClose = vi.fn();
    const { container } = renderModal({
      isOpen: true,
      onClose,
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(container.querySelector(".pm-btn--deny") as HTMLElement);
    expect(onClose).toHaveBeenCalled();
  });

  it("clicking approve in new mode calls onApprove with requestID and displayName", () => {
    const onApprove = vi.fn();
    const onClose = vi.fn();
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose,
      onApprove,
      onDeny: vi.fn(),
      request: makeRequest({ requestID: "req-42", displayName: "Tester" }),
    });
    fireEvent.click(getByText("pairing.newUser"));
    fireEvent.click(container.querySelector(".pm-btn--approve") as HTMLElement);
    expect(onApprove).toHaveBeenCalledWith("req-42", "", "Tester");
  });

  it("clicking approve calls onClose", () => {
    const onClose = vi.fn();
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose,
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(getByText("pairing.newUser"));
    fireEvent.click(container.querySelector(".pm-btn--approve") as HTMLElement);
    expect(onClose).toHaveBeenCalled();
  });

  it("renders select dropdown in existing user mode", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(container.querySelector(".pm-select")).toBeTruthy();
  });

  it("switching from new to existing mode clears selected user", () => {
    const { container, getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(getByText("pairing.newUser"));
    fireEvent.click(getByText("pairing.existingUser"));
    // Back in existing mode with empty selection — approve should be disabled
    const approveBtn = container.querySelector(".pm-btn--approve") as HTMLButtonElement;
    expect(approveBtn.disabled).toBe(true);
  });

  it("renders channel label via t()", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(getByText("pairing.channel")).toBeTruthy();
  });

  it("renders user label via t()", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(getByText("pairing.user")).toBeTruthy();
  });

  it("renders code label via t()", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(getByText("pairing.code")).toBeTruthy();
  });

  it("uses channelID as display when displayName is empty", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ displayName: "", channelID: "fallback-id" }),
    });
    expect(getByText("fallback-id")).toBeTruthy();
  });

  it("telegram channel type uses send icon", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ channelType: "telegram" }),
    });
    const icon = container.querySelector(".pm-channel-icon");
    expect(icon?.textContent).toBe("send");
  });

  it("discord channel type uses forum icon", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ channelType: "discord" }),
    });
    const icon = container.querySelector(".pm-channel-icon");
    expect(icon?.textContent).toBe("forum");
  });

  it("unknown channel type uses devices icon", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ channelType: "unknown" }),
    });
    const icon = container.querySelector(".pm-channel-icon");
    expect(icon?.textContent).toBe("devices");
  });

  it("whatsapp channel type uses chat icon", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ channelType: "whatsapp" }),
    });
    const icon = container.querySelector(".pm-channel-icon");
    expect(icon?.textContent).toBe("chat");
  });

  it("twilio channel type uses phone icon", () => {
    const { container } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest({ channelType: "twilio" }),
    });
    const icon = container.querySelector(".pm-channel-icon");
    expect(icon?.textContent).toBe("phone");
  });

  it("modal close button calls onClose", () => {
    const onClose = vi.fn();
    const { container } = renderModal({
      isOpen: true,
      onClose,
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    const closeBtn = container.querySelector(".modal-close") as HTMLElement;
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalled();
  });

  it("renders hint text in new user mode", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    fireEvent.click(getByText("pairing.newUser"));
    expect(getByText("pairing.displayNameHint")).toBeTruthy();
  });

  it("select dropdown default option uses t()", () => {
    const { getByText } = renderModal({
      isOpen: true,
      onClose: vi.fn(),
      onApprove: vi.fn(),
      onDeny: vi.fn(),
      request: makeRequest(),
    });
    expect(getByText("pairing.selectUser")).toBeTruthy();
  });
});
