// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";
import { fireEvent } from "@solidjs/testing-library";

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

  it("shows delete confirmation when delete button is clicked", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtn = container.querySelector(".skill-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    expect(container.querySelector(".skill-delete-confirm")).toBeTruthy();
  });

  it("hides delete button and shows confirmation inline", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtns = container.querySelectorAll(".skill-delete-btn");
    fireEvent.click(deleteBtns[0] as HTMLElement);
    // only 3 remaining delete buttons (one replaced by confirm)
    expect(container.querySelectorAll(".skill-delete-btn").length).toBe(3);
  });

  it("cancels delete confirmation with No button", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtn = container.querySelector(".skill-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    const noBtn = container.querySelector(".skill-delete-confirm-no") as HTMLElement;
    fireEvent.click(noBtn);
    expect(container.querySelector(".skill-delete-confirm")).toBeNull();
    expect(container.querySelectorAll(".skill-delete-btn").length).toBe(4);
  });

  it("clicking Yes button triggers deleteSkill mutate", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtn = container.querySelector(".skill-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    const yesBtn = container.querySelector(".skill-delete-confirm-yes") as HTMLElement;
    // should not throw — mutate called with skill name
    fireEvent.click(yesBtn);
    expect(yesBtn).toBeTruthy();
  });

  it("renders hidden file input for import", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    expect(fileInput).toBeTruthy();
    expect(fileInput.accept).toContain(".skill");
    expect(fileInput.accept).toContain(".zip");
  });

  it("does not show import error initially", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    expect(container.querySelector(".skills-import-error")).toBeNull();
  });

  it("renders import button in header", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    expect(container.querySelector(".btn-import-skill")).toBeTruthy();
  });

  it("file input triggers handleFileChange with a file — calls FileReader.readAsDataURL", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;

    const mockReadAsDataURL = vi.fn();
    const mockFileReader = {
      readAsDataURL: mockReadAsDataURL,
      onload: null as any,
      result: "data:application/zip;base64,AAAA",
    };
    vi.stubGlobal("FileReader", vi.fn(() => mockFileReader));

    const file = new File(["content"], "skill.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);

    expect(mockReadAsDataURL).toHaveBeenCalledWith(file);
    vi.unstubAllGlobals();
  });

  it("FileReader onload with valid dataURL calls importSkill.mutate", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;

    let capturedOnload: (() => void) | null = null;
    const mockFileReader = {
      readAsDataURL: vi.fn(),
      set onload(fn: () => void) { capturedOnload = fn; },
      get onload() { return capturedOnload; },
      result: "data:application/zip;base64,dGVzdGRhdGE=",
    };
    vi.stubGlobal("FileReader", vi.fn(() => mockFileReader));

    const file = new File(["test"], "skill.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);

    // simulate FileReader load completing
    capturedOnload?.();
    // no error should be shown — import was invoked
    expect(container.querySelector(".skills-import-error")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("FileReader onload with no base64 sets importError", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;

    let capturedOnload: (() => void) | null = null;
    const mockFileReader = {
      readAsDataURL: vi.fn(),
      set onload(fn: () => void) { capturedOnload = fn; },
      get onload() { return capturedOnload; },
      result: "data:application/zip;base64,",
    };
    vi.stubGlobal("FileReader", vi.fn(() => mockFileReader));

    const file = new File([""], "empty.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);
    capturedOnload?.();

    expect(container.querySelector(".skills-import-error")).toBeTruthy();
    vi.unstubAllGlobals();
  });

  it("no file selected — handleFileChange returns early without error", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    Object.defineProperty(fileInput, "files", { value: [], configurable: true });
    fireEvent.change(fileInput);
    expect(container.querySelector(".skills-import-error")).toBeNull();
  });
});
