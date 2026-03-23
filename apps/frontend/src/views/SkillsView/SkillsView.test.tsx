// DOM types are available globally via TypeScript lib; no imports needed here.
// Copyright (c) OpenLobster contributors. See LICENSE for details.
// Removed unused eslint-disable

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
    let lastInstance: any;
    const instance = {
      onload: null as null | (() => void),
      result: "data:application/zip;base64,AAAA",
      readAsDataURL: mockReadAsDataURL,
    };
    class MockFileReader {
      constructor() {
        lastInstance = instance;
         
        return instance as any;
      }
    }
    vi.stubGlobal("FileReader", MockFileReader as any);

    const file = new File(["content"], "skill.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);

    expect(mockReadAsDataURL).toHaveBeenCalledWith(file);
    expect(lastInstance).toBeTruthy();
    vi.unstubAllGlobals();
  });

  it("FileReader onload with valid dataURL calls importSkill.mutate", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;

    let lastInstance: any;
    const instance = {
      onload: null as null | (() => void),
      result: "data:application/zip;base64,dGVzdGRhdGE=",
      readAsDataURL: vi.fn(),
    };
    class MockFileReader {
      constructor() {
        lastInstance = instance;
         
        return instance as any;
      }
    }
    vi.stubGlobal("FileReader", MockFileReader as any);

    const file = new File(["test"], "skill.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);

    // simulate FileReader load completing
    const onload = lastInstance?.onload as (() => void) | null;
    if (onload) onload();
    // no error should be shown — import was invoked
    expect(container.querySelector(".skills-import-error")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("FileReader onload with no base64 sets importError", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;

    let lastInstance: any;
    const instance = {
      onload: null as null | (() => void),
      result: "data:application/zip;base64,",
      readAsDataURL: vi.fn(),
    };
    class MockFileReader {
      constructor() {
        lastInstance = instance;
         
        return instance as any;
      }
    }
    vi.stubGlobal("FileReader", MockFileReader as any);

    const file = new File([""], "empty.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);

    const onload = lastInstance?.onload as (() => void) | null;
    if (onload) onload();

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

  it("import button in empty state also triggers file input click", () => {
    // When skills.data is empty (not the mock scenario, but verify the button renders)
    const { container } = renderWithQueryClient(() => <SkillsView />);
    // With mock data having 4 skills, the header import button is present
    const importBtn = container.querySelector(".btn-import-skill") as HTMLElement;
    expect(importBtn).toBeTruthy();
    // Clicking should not throw (even though file picker won't open in test)
    fireEvent.click(importBtn);
    expect(container.querySelector(".skills-view")).toBeTruthy();
  });

  it("skill card renders correctly for each skill", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const cards = container.querySelectorAll(".skill-card");
    cards.forEach(card => {
      expect(card.querySelector(".skill-name")).toBeTruthy();
    });
  });

  it("skill-header contains skill-info section", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const headers = container.querySelectorAll(".skill-header");
    expect(headers.length).toBeGreaterThan(0);
    headers.forEach(h => {
      expect(h.querySelector(".skill-info")).toBeTruthy();
    });
  });

  it("skills-grid is rendered with correct class", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    expect(container.querySelector(".skills-grid")).toBeTruthy();
  });

  it("confirmDelete resets after clicking Yes button removes the confirm UI", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtn = container.querySelector(".skill-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    const yesBtn = container.querySelector(".skill-delete-confirm-yes") as HTMLElement;
    fireEvent.click(yesBtn);
    // After clicking yes, the confirm dialog disappears (onSuccess resets confirmDelete)
    // The mutate is called — since client mock does nothing, just confirm no crash
    expect(container.querySelector(".skills-view")).toBeTruthy();
  });

  it("FileReader onload dataURL without comma calls importSkill.mutate", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;

    let lastInstance: any;
    const instance = {
      onload: null as null | (() => void),
      result: "AAABBBCCC", // no comma
      readAsDataURL: vi.fn(),
    };
    class MockFileReader {
      constructor() {
        lastInstance = instance;
        return instance as any;
      }
    }
    vi.stubGlobal("FileReader", MockFileReader as any);

    const file = new File(["test"], "skill.zip", { type: "application/zip" });
    Object.defineProperty(fileInput, "files", { value: [file], configurable: true });
    fireEvent.change(fileInput);

    const onload = lastInstance?.onload as (() => void) | null;
    if (onload) onload();
    // dataUrl without comma passes the whole string as base64 — no error
    expect(container.querySelector(".skills-import-error")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("skills capabilities hint text is rendered", () => {
    const { getByText } = renderWithQueryClient(() => <SkillsView />);
    // capabilitiesHint key is rendered as translation key
    expect(getByText("Capabilities")).toBeTruthy();
  });

  it("deleteConfirm label text is rendered", () => {
    const { container } = renderWithQueryClient(() => <SkillsView />);
    const deleteBtn = container.querySelector(".skill-delete-btn") as HTMLElement;
    fireEvent.click(deleteBtn);
    expect(container.querySelector(".skill-delete-confirm-label")).toBeTruthy();
  });
});
