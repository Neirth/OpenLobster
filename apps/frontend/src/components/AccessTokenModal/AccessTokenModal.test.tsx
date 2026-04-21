// Copyright (c) OpenLobster contributors. See LICENSE for details.
 

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, waitFor } from "@solidjs/testing-library";

const mockSaveToken = vi.fn();
const mockRecheckConfig = vi.fn(() => Promise.resolve());
const mockValidateTokenOnServer = vi.fn((_token: string) => Promise.resolve(true));

vi.mock("../../App", () => ({
  t: (key: string) => key,
  recheckConfig: () => mockRecheckConfig(),
}));

vi.mock("../../stores/authStore", () => ({
  saveToken: (v: string) => mockSaveToken(v),
  validateTokenOnServer: (v: string) => mockValidateTokenOnServer(v),
  getStoredToken: () => null,
  syncNeedsAuthFromStorage: () => {},
  clearToken: () => {},
  needsAuth: () => false,
  setNeedsAuth: (_v: any) => {},
  isValidating: () => false,
  setIsValidating: (_v: any) => {},
}));

import AccessTokenModal from "./AccessTokenModal";

beforeEach(() => {
  mockSaveToken.mockClear();
  mockRecheckConfig.mockClear();
  mockValidateTokenOnServer.mockClear();
  mockValidateTokenOnServer.mockResolvedValue(true);
});

describe("AccessTokenModal Component", () => {
  it("renders the overlay", () => {
    const { container } = render(() => <AccessTokenModal />);
    expect(container.querySelector(".access-token-overlay")).toBeTruthy();
  });

  it("renders the modal card", () => {
    const { container } = render(() => <AccessTokenModal />);
    expect(container.querySelector(".access-token-modal")).toBeTruthy();
  });

  it("renders the lock icon", () => {
    const { container } = render(() => <AccessTokenModal />);
    expect(container.querySelector(".access-token-icon")).toBeTruthy();
  });

  it("renders the title translation key", () => {
    const { getByText } = render(() => <AccessTokenModal />);
    expect(getByText("accessToken.title")).toBeTruthy();
  });

  it("renders the password input", () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]');
    expect(input).toBeTruthy();
  });

  it("renders the submit button", () => {
    const { container } = render(() => <AccessTokenModal />);
    expect(container.querySelector(".access-token-submit")).toBeTruthy();
  });

  it("does not show error initially", () => {
    const { container } = render(() => <AccessTokenModal />);
    expect(container.querySelector(".access-token-error")).toBeNull();
  });

  it("shows error when submitting empty token", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(container.querySelector(".access-token-error")).toBeTruthy());
  });

  it("shows error when submitting whitespace-only token", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "   " } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(container.querySelector(".access-token-error")).toBeTruthy());
  });

  it("does not call saveToken when token is empty", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(mockSaveToken).not.toHaveBeenCalled();
  });

  it("calls saveToken with trimmed token on valid submit", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "  mytoken123  " } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(mockSaveToken).toHaveBeenCalledWith("mytoken123"));
  });

  it("calls recheckConfig after valid submit", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "validtoken" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(mockRecheckConfig).toHaveBeenCalled());
  });

  it("clears input value after valid submit", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "validtoken" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(input.value).toBe(""));
  });

  it("shows invalid token error when validation fails", async () => {
    mockValidateTokenOnServer.mockResolvedValue(false);
    const { container, getByText } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "wrongtoken" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(getByText("accessToken.errorInvalid")).toBeTruthy());
  });

  it("dismisses error when user types after error", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => expect(container.querySelector(".access-token-error")).toBeTruthy());

    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "a" } });
    expect(container.querySelector(".access-token-error")).toBeNull();
  });

  it("input has error class when error is shown", async () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    await waitFor(() => {
      const input = container.querySelector('input[type="password"]') as HTMLInputElement;
      expect(input.classList.contains("access-token-input--error")).toBe(true);
    });
  });

  it("input does not have error class initially", () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    expect(input.classList.contains("access-token-input--error")).toBe(false);
  });

  it("renders description with code elements", () => {
    const { container } = render(() => <AccessTokenModal />);
    const codes = container.querySelectorAll(".access-token-description code");
    expect(codes.length).toBe(2);
  });

  it("shows spinner and disables submit button during validation", async () => {
    let resolveValidation: (v: boolean) => void = () => {};
    mockValidateTokenOnServer.mockImplementation(() => new Promise((resolve) => {
      resolveValidation = resolve;
    }));

    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "validating_token" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);

    await waitFor(() => {
      const submitBtn = container.querySelector(".access-token-submit") as HTMLButtonElement;
      expect(submitBtn.disabled).toBe(true);
      expect(container.querySelector(".access-token-spinner")).toBeTruthy();
    });

    resolveValidation(true);
    await waitFor(() => {
      const submitBtn = container.querySelector(".access-token-submit") as HTMLButtonElement;
      expect(submitBtn.disabled).toBe(false);
      expect(container.querySelector(".access-token-spinner")).toBeNull();
    });
  });

  it("saves token even if validateTokenOnServer returns true (optimistic check)", async () => {
    // This simulates the network error behavior where validateTokenOnServer returns true.
    mockValidateTokenOnServer.mockResolvedValue(true);
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "optimistic_tok" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    
    await waitFor(() => expect(mockSaveToken).toHaveBeenCalledWith("optimistic_tok"));
    expect(container.querySelector(".access-token-error")).toBeNull();
  });
});

