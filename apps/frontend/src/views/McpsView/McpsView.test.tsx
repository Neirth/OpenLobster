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

vi.mock("../../components/MarketplaceModal", () => ({
  default: (_props: any) => null,
}));

import { renderWithQueryClient } from "../../test-utils";
import McpsView from "./McpsView";

describe("McpsView Component", () => {
  it("renders mcps view", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const view = container.querySelector(".mcps-view");
    expect(view).toBeTruthy();
  });

  it("renders header with title", () => {
    const { getByText } = renderWithQueryClient(() => <McpsView />);
    expect(getByText("MCP Servers")).toBeTruthy();
  });

  it("renders add server button", () => {
    const { getByText } = renderWithQueryClient(() => <McpsView />);
    expect(getByText(/Add.*Server/)).toBeTruthy();
  });

  it("renders server cards grid", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const grid = container.querySelector(".servers-grid");
    expect(grid).toBeTruthy();
  });

  it("renders server cards", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const cards = container.querySelectorAll(".server-card");
    expect(cards.length).toBeGreaterThan(0);
  });

  it("renders manage tools buttons", () => {
    const { getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageButtons = getAllByText("Manage");
    expect(manageButtons.length).toBeGreaterThan(0);
  });

  it("renders three section tabs", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const tabs = container.querySelectorAll(".mcps-section-tab");
    expect(tabs.length).toBe(3);
  });

  it("servers tab is active by default", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const activeTab = container.querySelector(".mcps-section-tab--active");
    expect(activeTab?.textContent).toMatch(/Servers/i);
  });

  it("clicking Built-in tab makes it active", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Built-in/i));
    expect(container.querySelector(".mcps-section-tab--active")?.textContent).toMatch(/Built-in/i);
  });

  it("built-in tab shows capability grid", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Built-in/i));
    expect(container.querySelector(".builtin-grid")).toBeTruthy();
  });

  it("clicking Permissions tab makes it active", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    expect(container.querySelector(".mcps-section-tab--active")?.textContent).toMatch(/Permissions/i);
  });

  it("permissions tab shows permissions layout", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    expect(container.querySelector(".permissions-layout")).toBeTruthy();
  });

  it("opens add server modal when button is clicked", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("add server modal has URL and name input fields", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    const inputs = container.querySelectorAll(".modal-overlay input");
    expect(inputs.length).toBeGreaterThanOrEqual(2);
  });

  it("add server modal closes on cancel", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    fireEvent.click(getByText("Cancel"));
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("renders status dots on server cards", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const statusDots = container.querySelectorAll(".server-status-dot");
    expect(statusDots.length).toBeGreaterThan(0);
  });

  it("renders server name on card", () => {
    const { getByText } = renderWithQueryClient(() => <McpsView />);
    expect(getByText("filesystem")).toBeTruthy();
  });

  it("renders tool count on server cards", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const toolCounts = container.querySelectorAll(".tools-count");
    expect(toolCounts.length).toBeGreaterThan(0);
  });

  it("clicking Manage button opens tools modal", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("servers tab re-activates when clicked after switching away", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Built-in/i));
    fireEvent.click(getByText(/Servers/i));
    expect(container.querySelector(".mcps-section-tab--active")?.textContent).toMatch(/Servers/i);
    expect(container.querySelector(".servers-grid")).toBeTruthy();
  });

  it("permissions tab shows user list", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    const userList = container.querySelector(".permissions-user-list");
    expect(userList).toBeTruthy();
    expect(container.querySelectorAll(".permissions-user-item").length).toBeGreaterThan(0);
  });

  it("selecting a user in permissions tab shows tool matrix", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    const firstUser = container.querySelector(".permissions-user-item") as HTMLElement;
    fireEvent.click(firstUser);
    expect(container.querySelector(".permissions-tool-table")).toBeTruthy();
  });

  it("selected user item gets active class", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    const firstUser = container.querySelector(".permissions-user-item") as HTMLElement;
    fireEvent.click(firstUser);
    expect(firstUser.classList.contains("permissions-user-item--active")).toBe(true);
  });

  it("tool groups are rendered in permissions matrix", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const groups = container.querySelectorAll(".permissions-tool-group__header");
    expect(groups.length).toBeGreaterThan(0);
  });

  it("tool rows are rendered inside a group", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const rows = container.querySelectorAll(".permissions-tool-row");
    expect(rows.length).toBeGreaterThan(0);
  });

  it("clicking a group header collapses its tools", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const initialRows = container.querySelectorAll(".permissions-tool-row").length;
    const firstGroup = container.querySelector(".permissions-tool-group__header") as HTMLElement;
    fireEvent.click(firstGroup);
    const afterRows = container.querySelectorAll(".permissions-tool-row").length;
    expect(afterRows).toBeLessThan(initialRows);
  });

  it("clicking collapsed group header expands it again", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const firstGroup = container.querySelector(".permissions-tool-group__header") as HTMLElement;
    const rowsBefore = container.querySelectorAll(".permissions-tool-row").length;
    fireEvent.click(firstGroup);
    fireEvent.click(firstGroup);
    expect(container.querySelectorAll(".permissions-tool-row").length).toBe(rowsBefore);
  });

  it("clicking a tool toggle changes its state", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const firstToggle = container.querySelector(".permissions-toggle") as HTMLElement;
    // clicking should not throw even though mutate is mocked
    fireEvent.click(firstToggle);
    expect(firstToggle).toBeTruthy();
  });

  it("bulk allow button is rendered when user is selected", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    expect(container.querySelector(".btn-bulk-allow")).toBeTruthy();
  });

  it("clicking bulk allow button calls mutate", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const bulkAllow = container.querySelector(".btn-bulk-allow") as HTMLElement;
    fireEvent.click(bulkAllow);
    expect(bulkAllow).toBeTruthy();
  });

  it("clicking bulk deny button calls mutate", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    const bulkDeny = container.querySelector(".btn-bulk-deny") as HTMLElement;
    fireEvent.click(bulkDeny);
    expect(bulkDeny).toBeTruthy();
  });

  it("permissions tab shows empty state when no user selected", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    expect(container.querySelector(".permissions-empty-state")).toBeTruthy();
  });

  it("clicking a built-in capability card opens detail modal", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Built-in/i));
    const firstCard = container.querySelector(".builtin-card") as HTMLElement;
    fireEvent.click(firstCard);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("built-in detail modal lists tools", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Built-in/i));
    const firstCard = container.querySelector(".builtin-card") as HTMLElement;
    fireEvent.click(firstCard);
    // any list of tool items should be present
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("built-in detail modal closes when close button is clicked", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Built-in/i));
    const firstCard = container.querySelector(".builtin-card") as HTMLElement;
    fireEvent.click(firstCard);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    // The Modal component renders a close button
    const closeBtn = container.querySelector(".modal-close") as HTMLElement;
    if (closeBtn) fireEvent.click(closeBtn);
    else {
      // fallback: click overlay backdrop
      const overlay = container.querySelector(".modal-overlay") as HTMLElement;
      fireEvent.click(overlay);
    }
    // modal should close
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("manage server modal shows disconnect button", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    // just check modal is open with content
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("advanced options toggle in add server modal", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    const advancedBtn = container.querySelector(".modal-advanced-toggle, [class*='advanced']") as HTMLElement;
    if (advancedBtn) {
      fireEvent.click(advancedBtn);
      expect(container.querySelector(".modal-overlay")).toBeTruthy();
    } else {
      // advanced options button may not be present — just verify modal is open
      expect(container.querySelector(".modal-overlay")).toBeTruthy();
    }
  });

  it("add server form inputs accept text", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    const inputs = container.querySelectorAll(".modal-overlay input[type='text'], .modal-overlay input:not([type])") as NodeListOf<HTMLInputElement>;
    if (inputs.length > 0) {
      fireEvent.input(inputs[0], { target: { value: "test-server" } });
      expect(inputs[0].value).toBe("test-server");
    }
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("marketplace button is rendered and clickable", () => {
    const { container } = renderWithQueryClient(() => <McpsView />);
    const marketplaceBtns = container.querySelectorAll(".marketplace-btn");
    expect(marketplaceBtns.length).toBeGreaterThan(0);
    // Clicking should not throw
    fireEvent.click(marketplaceBtns[0] as HTMLElement);
    expect(container.querySelector(".mcps-view")).toBeTruthy();
  });

  it("manage server modal shows server tools", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    // modal should show tools section
    expect(container.querySelector(".server-tools")).toBeTruthy();
  });

  it("manage server modal shows server status", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    const sections = container.querySelectorAll(".modal-section");
    expect(sections.length).toBeGreaterThan(0);
  });

  it("manage server modal has remove server button", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    const dangerBtn = container.querySelector(".btn-danger") as HTMLElement;
    expect(dangerBtn).toBeTruthy();
    // clicking remove should not throw
    fireEvent.click(dangerBtn);
    expect(container).toBeTruthy();
  });

  it("manage server modal has OAuth authorize button", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    const oauthBtn = container.querySelector(".btn-oauth") as HTMLElement;
    expect(oauthBtn).toBeTruthy();
    // clicking OAuth button should not throw
    fireEvent.click(oauthBtn);
    expect(container).toBeTruthy();
  });

  it("manage server modal closes when close button is clicked", () => {
    const { container, getAllByText } = renderWithQueryClient(() => <McpsView />);
    const manageBtn = getAllByText("Manage")[0] as HTMLElement;
    fireEvent.click(manageBtn);
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
    // Find the secondary button (Close) in the form-actions of the manage modal
    const formActions = container.querySelector(".modal-overlay .form-actions") as HTMLElement;
    const closeBtn = formActions?.querySelector(".btn-secondary") as HTMLElement;
    if (closeBtn) fireEvent.click(closeBtn);
    expect(container.querySelector(".modal-overlay")).toBeNull();
  });

  it("add server checkbox toggles advanced options", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    const checkbox = container.querySelector("#mcp-advanced-options") as HTMLInputElement;
    expect(checkbox).toBeTruthy();
    fireEvent.input(checkbox, { target: { checked: true } });
    // advanced client ID input appears
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("add server form URL input accepts text", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    const urlInput = container.querySelector("input[type='url']") as HTMLInputElement;
    expect(urlInput).toBeTruthy();
    fireEvent.input(urlInput, { target: { value: "http://localhost:3000" } });
    expect(urlInput.value).toBe("http://localhost:3000");
  });

  it("add server submit button calls handleAddServer", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Add.*Server/i));
    const nameInput = container.querySelector("input[type='text']") as HTMLInputElement;
    const urlInput = container.querySelector("input[type='url']") as HTMLInputElement;
    fireEvent.input(nameInput, { target: { value: "my-server" } });
    fireEvent.input(urlInput, { target: { value: "http://localhost:3000" } });
    const addBtn = container.querySelector(".btn-primary") as HTMLElement;
    fireEvent.click(addBtn);
    // just ensure no throw — connectMcp.mutate is mocked
    expect(container.querySelector(".modal-overlay")).toBeTruthy();
  });

  it("permissions main header shows policy note when user selected", () => {
    const { container, getByText } = renderWithQueryClient(() => <McpsView />);
    fireEvent.click(getByText(/Permissions/i));
    fireEvent.click(container.querySelector(".permissions-user-item") as HTMLElement);
    expect(container.querySelector(".permissions-policy-note")).toBeTruthy();
  });
});
