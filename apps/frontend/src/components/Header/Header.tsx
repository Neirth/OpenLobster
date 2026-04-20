// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Header renders the 50px top bar shared across all views.
 *
 * Layout:
 *   Left  — smart_toy icon + OPENLOBSTER wordmark
 *   Center — tab navigation (absolutely centered)
 *   Right  — status dot + agent name + version + pending pairings button + avatar
 *
 * @param activeTab - The currently active tab identifier
 */

import { For, Show, createMemo, createSignal } from "solid-js";
import type { Component } from "solid-js";
import type { GraphQLClient } from "graphql-request";
import { A } from "@solidjs/router";
import { useAgent } from "@/ui/hooks";
import { t } from "../../App";
import { wsConnected } from "../../stores/wsStore";
import { pendingPairingsQueue, openPairingRequest } from "../../stores/pairingStore";
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
  // eslint-disable-next-line solid/reactivity
  const agent = useAgent(props.graphqlClient as GraphQLClient);
  const tabs = createMemo(() => getTabs());
  const [pairingDropdownOpen, setPairingDropdownOpen] = createSignal(false);

  const pendingCount = createMemo(() => pendingPairingsQueue().length);

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

        {/* Pending pairing requests button */}
        <Show when={pendingCount() > 0}>
          <div class="header__pairing-wrap">
            <button
              type="button"
              class="header__pairing-btn"
              classList={{ "header__pairing-btn--open": pairingDropdownOpen() }}
              onClick={() => setPairingDropdownOpen((v) => !v)}
              aria-label={t("header.pendingPairings")}
              title={t("header.pendingPairings")}
            >
              <span class="material-symbols-outlined" aria-hidden={true}>link</span>
              <span class="header__pairing-badge">{pendingCount()}</span>
            </button>
            <Show when={pairingDropdownOpen()}>
              <div class="header__pairing-dropdown" role="menu">
                <p class="header__pairing-dropdown-title">{t("header.pendingPairingsTitle")}</p>
                <For each={pendingPairingsQueue()}>
                  {(req) => (
                    <button
                      type="button"
                      class="header__pairing-item"
                      role="menuitem"
                      onClick={() => {
                        setPairingDropdownOpen(false);
                        openPairingRequest(req);
                      }}
                    >
                      <span class="material-symbols-outlined header__pairing-item-icon">
                        {req.channelType === "telegram" ? "send"
                          : req.channelType === "discord" ? "forum"
                          : req.channelType === "whatsapp" ? "chat"
                          : req.channelType === "twilio" ? "phone"
                          : "devices"}
                      </span>
                      <span class="header__pairing-item-name">
                        {req.displayName || req.channelID || req.requestID}
                      </span>
                      <span class="header__pairing-item-channel">{req.channelType}</span>
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>
        </Show>

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
