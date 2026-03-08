// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";

vi.mock("@solidjs/router", () => ({
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

vi.mock("../../components/AppShell/AppShell", () => ({
  default: (props: any) => <div class="app-shell" {...props} />,
}));

vi.mock("../../graphql/client", () => ({ client: {} }));

import { renderWithQueryClient } from "../../test-utils";
import SkillsView from "./SkillsView";

describe("SkillsView Component", () => {
  it("renders skills view", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    expect(container.querySelector(".skills-view")).toBeTruthy();
  });

  it("renders header with title", () => {
    const { getByText } = renderWithQueryClient(() => <SkillsView />);
    expect(getByText("Capabilities")).toBeTruthy();
  });

  it("renders skills section", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    expect(container.querySelector(".skills-section")).toBeTruthy();
  });

  it("renders SKILLS section heading", () => {
    const { getByText } = renderWithQueryClient(() => <SkillsView />);
    expect(getByText("SKILLS")).toBeTruthy();
  });

  it("renders skill cards from hook data", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const cards = container.querySelectorAll(".skill-card");
    expect(cards.length).toBe(4);
  });

  it("renders skill names from hook data", () => {
    const { getByText } = renderWithQueryClient(() => <SkillsView />);
    expect(getByText("computer-science")).toBeTruthy();
    expect(getByText("general-engineering")).toBeTruthy();
  });

  it("renders skill descriptions", () => {
    const { getByText } = renderWithQueryClient(() => <SkillsView />);
    expect(getByText("Software engineering and CS expertise")).toBeTruthy();
  });

  it("renders delete buttons for each skill", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtns = container.querySelectorAll(".skill-delete-btn");
    expect(deleteBtns.length).toBe(4);
  });
});
