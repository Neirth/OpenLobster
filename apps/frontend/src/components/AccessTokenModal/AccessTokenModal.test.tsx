// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent } from "@solidjs/testing-library";

const mockSaveToken = vi.fn();
const mockRecheckConfig = vi.fn(() => Promise.resolve());

vi.mock("../../App", () => ({
  t: (key: string) => key,
  recheckConfig: () => mockRecheckConfig(),
}));

vi.mock("../../stores/authStore", () => ({
  saveToken: (v: string) => mockSaveToken(v),
}));

import AccessTokenModal from "./AccessTokenModal";

beforeEach(() => {
  mockSaveToken.mockClear();
  mockRecheckConfig.mockClear();
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

  it("shows error when submitting empty token", () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(container.querySelector(".access-token-error")).toBeTruthy();
  });

  it("shows error when submitting whitespace-only token", () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "   " } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(container.querySelector(".access-token-error")).toBeTruthy();
  });

  it("does not call saveToken when token is empty", () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(mockSaveToken).not.toHaveBeenCalled();
  });

  it("calls saveToken with trimmed token on valid submit", () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "  mytoken123  " } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(mockSaveToken).toHaveBeenCalledWith("mytoken123");
  });

  it("calls recheckConfig after valid submit", () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "validtoken" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(mockRecheckConfig).toHaveBeenCalled();
  });

  it("clears input value after valid submit", () => {
    const { container } = render(() => <AccessTokenModal />);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "validtoken" } });
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(input.value).toBe("");
  });

  it("dismisses error when user types after error", () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    expect(container.querySelector(".access-token-error")).toBeTruthy();

    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    fireEvent.input(input, { target: { value: "a" } });
    expect(container.querySelector(".access-token-error")).toBeNull();
  });

  it("input has error class when error is shown", () => {
    const { container } = render(() => <AccessTokenModal />);
    const form = container.querySelector(".access-token-form") as HTMLFormElement;
    fireEvent.submit(form);
    const input = container.querySelector('input[type="password"]') as HTMLInputElement;
    expect(input.classList.contains("access-token-input--error")).toBe(true);
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
});
