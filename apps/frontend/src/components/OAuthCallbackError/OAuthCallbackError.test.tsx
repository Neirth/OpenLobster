// Copyright (c) OpenLobster contributors. See LICENSE for details.
// Removed unused eslint-disable

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent } from "@solidjs/testing-library";

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

import OAuthCallbackError from "./OAuthCallbackError";

describe("OAuthCallbackError Component", () => {
  beforeEach(() => {
    // Reset window.opener
    Object.defineProperty(window, "opener", {
      value: null,
      writable: true,
      configurable: true,
    });
  });

  it("renders the error container", () => {
    const { container } = render(() => (
      <OAuthCallbackError message="Access denied" onClose={() => {}} />
    ));
    expect(container.querySelector(".oauth-callback-error")).toBeTruthy();
  });

  it("renders the modal element", () => {
    const { container } = render(() => (
      <OAuthCallbackError message="Access denied" onClose={() => {}} />
    ));
    expect(container.querySelector(".oauth-callback-error__modal")).toBeTruthy();
  });

  it("renders the error icon", () => {
    const { container } = render(() => (
      <OAuthCallbackError message="Access denied" onClose={() => {}} />
    ));
    expect(container.querySelector(".oauth-callback-error__icon")).toBeTruthy();
  });

  it("renders the title via t()", () => {
    const { getByText } = render(() => (
      <OAuthCallbackError message="Access denied" onClose={() => {}} />
    ));
    expect(getByText("mcps.oauthCallbackErrorTitle")).toBeTruthy();
  });

  it("renders the provided error message", () => {
    const { getByText } = render(() => (
      <OAuthCallbackError message="Token expired" onClose={() => {}} />
    ));
    expect(getByText("Token expired")).toBeTruthy();
  });

  it("renders the close button", () => {
    const { container } = render(() => (
      <OAuthCallbackError message="Error" onClose={() => {}} />
    ));
    expect(container.querySelector(".oauth-callback-error__btn")).toBeTruthy();
  });

  it("close button label uses t()", () => {
    const { getByText } = render(() => (
      <OAuthCallbackError message="Error" onClose={() => {}} />
    ));
    expect(getByText("mcps.oauthCallbackErrorClose")).toBeTruthy();
  });

  it("calls onClose when close button is clicked", () => {
    const onClose = vi.fn();
    const { container } = render(() => (
      <OAuthCallbackError message="Error" onClose={onClose} />
    ));
    const btn = container.querySelector(".oauth-callback-error__btn") as HTMLElement;
    fireEvent.click(btn);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("calls onClose again on second click", () => {
    const onClose = vi.fn();
    const { container } = render(() => (
      <OAuthCallbackError message="Error" onClose={onClose} />
    ));
    const btn = container.querySelector(".oauth-callback-error__btn") as HTMLElement;
    fireEvent.click(btn);
    fireEvent.click(btn);
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("posts oauth_error message to window.opener on mount", () => {
    const postMessage = vi.fn();
    Object.defineProperty(window, "opener", {
      value: { postMessage },
      writable: true,
      configurable: true,
    });

    render(() => (
      <OAuthCallbackError message="Server error" onClose={() => {}} />
    ));

    expect(postMessage).toHaveBeenCalledWith(
      { type: "oauth_error", error: "Server error" },
      "*",
    );
  });

  it("does not call postMessage when window.opener is null", () => {
    const postMessage = vi.fn();
    Object.defineProperty(window, "opener", {
      value: null,
      writable: true,
      configurable: true,
    });

    // Should not throw
    render(() => (
      <OAuthCallbackError message="error" onClose={() => {}} />
    ));
    expect(postMessage).not.toHaveBeenCalled();
  });

  it("passes the correct error string to postMessage", () => {
    const postMessage = vi.fn();
    Object.defineProperty(window, "opener", {
      value: { postMessage },
      writable: true,
      configurable: true,
    });

    render(() => (
      <OAuthCallbackError message="invalid_grant" onClose={() => {}} />
    ));

    expect(postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ error: "invalid_grant" }),
      "*",
    );
  });

  it("renders different message props correctly", () => {
    const { getByText, unmount } = render(() => (
      <OAuthCallbackError message="First message" onClose={() => {}} />
    ));
    expect(getByText("First message")).toBeTruthy();
    unmount();

    const { getByText: getByText2 } = render(() => (
      <OAuthCallbackError message="Second message" onClose={() => {}} />
    ));
    expect(getByText2("Second message")).toBeTruthy();
  });
});
