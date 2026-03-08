// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Header renders the 50px top bar shared across all views.
 *
 * Layout:
 *   Left  — smart_toy icon + OPENLOBSTER wordmark
 *   Center — tab navigation (absolutely centered)
 *   Right  — status dot + agent name + version + avatar
 *
 * @param activeTab - The currently active tab identifier
 */

import { For, createMemo } from "solid-js";
import type { Component } from "solid-js";
import type { GraphQLClient } from "graphql-request";
import { A } from "@solidjs/router";
import { useAgent } from "@openlobster/ui/hooks";
import { t } from "../../App";
import { wsConnected } from "../../stores/wsStore";
import "./Header.css";

export type TabId =
  | "overview"
  | "chat"
  | "tasks"
  | "memory"
  | "mcps"
  | "skills"
  | "settings";

interface Tab {
  id: TabId;
  labelKey: string;
  path: string;
}

const getTabs = (): Tab[] => [
  { id: "overview", labelKey: "dashboard.title", path: "/" },
  { id: "chat", labelKey: "chat.title", path: "/chat" },
  { id: "tasks", labelKey: "tasks.title", path: "/tasks" },
  { id: "memory", labelKey: "memory.title", path: "/memory" },
  { id: "mcps", labelKey: "mcps.title", path: "/mcps" },
  { id: "skills", labelKey: "skills.title", path: "/skills" },
  { id: "settings", labelKey: "settings.title", path: "/settings" },
];

interface HeaderProps {
  activeTab: TabId;
  graphqlClient?: GraphQLClient;
}

const Header: Component<HeaderProps> = (props) => {
  const graphqlClient = props.graphqlClient;
  const agent = useAgent(graphqlClient as GraphQLClient);
  const tabs = createMemo(() => getTabs());

  return (
    <header class="header">
      {/* Left — logo */}
      <div class="header__left">
        <span class="material-symbols-outlined header__logo-icon">
          smart_toy
        </span>
        <span class="header__wordmark">{t("header.brand")}</span>
      </div>

      {/* Center — tab navigation (absolutely centered) */}
      <nav class="header__nav" aria-label="Main navigation">
        <For each={tabs()}>
          {(tab) => (
            <A
              href={tab.path}
              class="header__tab"
              classList={{ "header__tab--active": props.activeTab === tab.id }}
              end={tab.path === "/"}
            >
              {t(tab.labelKey)}
            </A>
          )}
        </For>
      </nav>

      {/* Right — agent status */}
      <div class="header__right">
        <span
          class="header__status-dot"
          style={{
            background: wsConnected()
              ? "var(--color-success)"
              : "var(--color-error)",
          }}
        />
        <span class="header__agent-name">{agent.data?.name ?? t("header.defaultAgentName")}</span>
        <span class="header__version">v{agent.data?.version ?? "0.1.0"}</span>
        <span class="header__avatar">
          <span
            class="material-symbols-outlined"
            style={{ "font-size": "16px", color: "var(--color-text-muted)" }}
          >
            person
          </span>
        </span>
      </div>
    </header>
  );
};

export default Header;
