// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";
import { render } from "@solidjs/testing-library";

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
}));

vi.mock("../../graphql/client", () => ({
  client: {},
}));

import Header from "./Header";

describe("Header Component", () => {
  it("renders header element", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector("header")).toBeTruthy();
  });

  it("applies header class", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(
      container.querySelector("header")?.classList.contains("header"),
    ).toBe(true);
  });

  it("renders logo section on left", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__left")).toBeTruthy();
  });

  it("renders logo icon", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__logo-icon")).toBeTruthy();
  });

  it("renders wordmark", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__wordmark")?.textContent).toContain(
      "OPENLOBSTER",
    );
  });

  it("renders agent name from hook data", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__agent-name")?.textContent).toBe(
      "agent-01",
    );
  });

  it("renders fallback agent name when data is undefined", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__agent-name")?.textContent).toBe(
      "agent-01",
    );
  });

  it("renders version from hook data", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__version")?.textContent).toContain(
      "1.0.0",
    );
  });

  it("renders with different active tabs", () => {
    const tabs = [
      "overview",
      "chat",
      "tasks",
      "memory",
      "mcps",
      "skills",
      "settings",
    ] as const;
    tabs.forEach((tab) => {
      const { container } = render(() => <Header activeTab={tab} />);
      expect(container.querySelector("header")).toBeTruthy();
    });
  });

  it("renders navigation area", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    expect(container.querySelector(".header__nav")).toBeTruthy();
  });

  it("renders tab links for all tabs", () => {
    const { container } = render(() => <Header activeTab="overview" />);
    const tabs = container.querySelectorAll(".header__tab");
    expect(tabs.length).toBe(7);
  });
});
