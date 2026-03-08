// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * DashboardView — the Overview tab.
 *
 * Displays:
 *  - 4 stat cards (Health, Active Sessions, MCP Servers, Agent Version)
 *  - Two-column grid: Channels + Open Sessions  |  System Status + MCP Servers
 *  - Recent logs section
 */

import type { Component } from "solid-js";
import { For, Show, Suspense, createSignal, onMount, onCleanup } from "solid-js";
import { t } from "../../App";
import { getStoredToken } from "../../stores/authStore";
import {
  useMetrics,
  useLogs,
  useChannels,
  useConversations,
  useMcpServers,
  useAgent,
  useConfig,
} from "@openlobster/ui/hooks";
import { client } from "../../graphql/client";
import AppShell from "../../components/AppShell/AppShell";
import "./DashboardView.css";

/**
 * Returns the favicon URL for a given server URL via the Google favicon service.
 *
 * @param serverUrl - The full server URL.
 * @returns A Google S2 favicon service URL for the root domain.
 */
function faviconUrl(serverUrl: string): string {
  try {
    const { hostname } = new URL(serverUrl);
    const parts = hostname.split('.');
    const rootDomain = parts.length > 2 ? parts.slice(-2).join('.') : hostname;
    return `https://www.google.com/s2/favicons?domain=${rootDomain}&sz=32`;
  } catch {
    return '';
  }
}

const CHANNEL_ICONS: Record<string, string> = {
  discord: "chat",
  telegram: "send",
  whatsapp: "chat",
  twilio: "phone",
  stdio: "terminal",
  http: "http",
};

/** Backend puede enviar "online" o "active" para canales conectados. */
function isChannelOnline(status: string): boolean {
  return status === "online" || status === "active";
}

const DashboardView: Component = () => {
  const metrics = useMetrics(client);
  const logs = useLogs(client, getStoredToken);
  const channels = useChannels(client);
  const conversations = useConversations(client);
  const mcpServers = useMcpServers(client);
  const agent = useAgent(client);
  const config = useConfig(client);

  // Health polling: polls /health every 10 s and reports OK / ERROR / KO.
  type HealthStatus = "OK" | "ERROR" | "KO";
  const [healthStatus, setHealthStatus] = createSignal<HealthStatus>("KO");

  onMount(() => {
    const checkHealth = async () => {
      try {
        const resp = await fetch("/health");
        setHealthStatus(resp.ok ? "OK" : "ERROR");
      } catch {
        setHealthStatus("KO");
      }
    };
    checkHealth();
    const timer = setInterval(checkHealth, 10_000);
    onCleanup(() => clearInterval(timer));
  });

  const healthClass = () =>
    healthStatus() === "OK"
      ? "stat-value--success"
      : healthStatus() === "ERROR"
        ? "stat-value--error"
        : "stat-value--warning";

  const healthIcon = () =>
    healthStatus() === "OK"
      ? "check_circle"
      : healthStatus() === "ERROR"
        ? "error"
        : "warning";

  const healthIconClass = () =>
    healthStatus() === "OK"
      ? "stat-value__icon"
      : healthStatus() === "ERROR"
        ? "stat-value__icon--error"
        : "stat-value__icon--warning";

  return (
    <AppShell activeTab="overview">
      <div class="dashboard-overview">
        {/* Stat cards row */}
        <div class="stat-grid">
          <div class="stat-card">
            <span class="stat-label">Health</span>
            <span class={`stat-value ${healthClass()}`}>
              <span class={`material-symbols-outlined ${healthIconClass()}`}>
                {healthIcon()}
              </span>
              {healthStatus()}
            </span>
          </div>
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.activeSessions")}</span>
            <span class="stat-value">
              <Show when={metrics.data} fallback="—">
                {(m) => m().activeSessions}
              </Show>
            </span>
          </div>
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.servers")}</span>
            <span class="stat-value">{mcpServers.data?.length ?? 0}</span>
          </div>
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.agentVersion")}</span>
            <span class="stat-value stat-value--with-icon">
              {agent.data?.version ?? "0.1.0"}
              <span class="material-symbols-outlined stat-value__info">
                info
              </span>
            </span>
          </div>
        </div>

        {/* Second stat cards row - tasks and messages */}
        <div class="stat-grid">
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.tasksPending")}</span>
            <span class="stat-value">
              <Show when={metrics.data} fallback="—">
                {(m) => m().tasksPending}
              </Show>
            </span>
          </div>
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.tasksDone")}</span>
            <span class="stat-value">
              <Show when={metrics.data} fallback="—">
                {(m) => m().tasksDone}
              </Show>
            </span>
          </div>
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.messagesReceived")}</span>
            <span class="stat-value">
              <Show when={metrics.data} fallback="—">
                {(m) => m().messagesReceived}
              </Show>
            </span>
          </div>
          <div class="stat-card">
            <span class="stat-label">{t("dashboard.messagesSent")}</span>
            <span class="stat-value">
              <Show when={metrics.data} fallback="—">
                {(m) => m().messagesSent}
              </Show>
            </span>
          </div>
        </div>

        {/* 2×2 grid */}
        <div class="dashboard-grid">
          {/* Top-left: Channels */}
          <section class="dashboard-panel">
            <h2 class="section-header">{t("dashboard.channels")}</h2>
              <Suspense>
                <For
                  each={channels.data}
                  fallback={<p class="empty-text">{t("dashboard.noChannels")}</p>}
                >
                  {(channel) => (
                    <div class="list-row">
                      <div class="list-row__left">
                        <span class="channel-icon">
                          <span
                            class="material-symbols-outlined"
                            style={{ "font-size": "14px" }}
                          >
                            {CHANNEL_ICONS[channel.type.toLowerCase()] ?? "hub"}
                          </span>
                        </span>
                        <span
                          class="list-row__name"
                          classList={{
                            "list-row__name--muted":
                              !isChannelOnline(channel.status),
                          }}
                        >
                          {channel.name}
                        </span>
                      </div>
                      <div class="list-row__right">
                        <span class="list-row__sub">
                          {isChannelOnline(channel.status)
                            ? t("status.online")
                            : channel.status === "degraded"
                              ? t("status.degraded")
                              : t("status.offline")}
                        </span>
                        <span
                          class="status-dot"
                          style={{
                            background:
                              isChannelOnline(channel.status)
                                ? "var(--color-success)"
                                : channel.status === "degraded"
                                  ? "var(--color-warning)"
                                  : "var(--color-error)",
                          }}
                        />
                      </div>
                    </div>
                  )}
                </For>
              </Suspense>
            </section>

          {/* Top-right: System Status */}
          <section class="dashboard-panel">
            <h2 class="section-header">{t("dashboard.systemStatus")}</h2>
            <div class="list-row">
              <span class="list-row__name">{t("dashboard.memoryBackend")}</span>
              <div class="list-row__right">
                <span class="list-row__sub">
                  {config.data?.memory?.backend ?? "—"}
                </span>
                <span
                  class="status-dot"
                  style={{ background: "var(--color-success)" }}
                />
              </div>
            </div>
            <div class="list-row">
              <span class="list-row__name">{t("dashboard.uptime")}</span>
              <div class="list-row__right">
                <span class="list-row__mono">
                  {metrics.data
                    ? `${Math.floor(metrics.data.uptime / 3600)}h ${Math.floor((metrics.data.uptime % 3600) / 60)}m`
                    : "—"}
                </span>
              </div>
            </div>
            <div class="list-row">
              <span class="list-row__name">{t("dashboard.secretsBackend")}</span>
              <div class="list-row__right">
                <span class="list-row__sub">
                  {config.data?.secrets?.backend ?? "—"}
                </span>
                <span
                  class="status-dot"
                  style={{ background: "var(--color-success)" }}
                />
              </div>
            </div>
          </section>

          {/* Bottom-left: Recent Conversations */}
          <section class="dashboard-panel">
            <h2 class="section-header">{t("dashboard.recentConversations")}</h2>
              <Suspense>
                <For
                  each={conversations.data?.slice(0, 5)}
                  fallback={<p class="empty-text">{t("dashboard.noActiveSessions")}</p>}
                >
                  {(conv) => (
                    <div class="list-row">
                      <div class="list-row__left">
                        <span class="avatar-circle">
                          {conv.participantName.charAt(0).toUpperCase()}
                        </span>
                        <span class="list-row__name">
                          {conv.isGroup && conv.groupName ? conv.groupName : conv.participantName}
                        </span>
                      </div>
                      <div class="list-row__right">
                        <span
                          class="badge badge--success"
                          classList={{ "badge--success": conv.unreadCount > 0 }}
                        >
                          {conv.unreadCount > 0 ? t("dashboard.active") : t("dashboard.idle")}
                        </span>
                      </div>
                    </div>
                  )}
                </For>
                <Show when={(conversations.data?.length ?? 0) > 5}>
                  <div class="list-row list-row--ellipsis">•••</div>
                </Show>
              </Suspense>
          </section>

          {/* Bottom-right: MCP Servers */}
          <section class="dashboard-panel">
            <h2 class="section-header">{t("dashboard.servers")}</h2>
              <Suspense>
                <For
                  each={mcpServers.data?.slice(0, 5)}
                  fallback={<p class="empty-text">{t("mcps.noServers")}</p>}
                >
                  {(server) => (
                    <div class="list-row">
                      <div class="list-row__left">
                        <Show
                          when={server.url}
                          fallback={
                            <span class="channel-icon">
                              <span
                                class="material-symbols-outlined"
                                style={{ "font-size": "14px" }}
                              >
                                extension
                              </span>
                            </span>
                          }
                        >
                          <img
                            class="dash-server-favicon"
                            src={faviconUrl(server.url!)}
                            alt=""
                          />
                        </Show>
                        <span class="list-row__name">{server.name}</span>
                        <span class="badge">{server.transport}</span>
                        <span class="list-row__mono">
                          {server.toolCount} {t("mcps.tools")}
                        </span>
                      </div>
                      <div class="list-row__right">
                        <span
                          class="status-dot"
                          style={{
                            background:
                              server.status === "online"
                                ? "var(--color-success)"
                                : server.status === "degraded"
                                  ? "var(--color-warning)"
                                  : "var(--color-error)",
                          }}
                        />
                      </div>
                    </div>
                  )}
                </For>
                <Show when={(mcpServers.data?.length ?? 0) > 5}>
                  <div class="list-row list-row--ellipsis">•••</div>
                </Show>
              </Suspense>
          </section>
        </div>

        {/* Recent logs */}
        <section class="logs-section">
          <div class="logs-header">
            <h2 class="section-header" style={{ margin: 0 }}>
              {t("dashboard.recentLogs")}
            </h2>
          </div>
          <Show when={!logs.data}>
            <div class="logs-empty">
              <p class="empty-text">{t("dashboard.waitingLogs")}</p>
            </div>
          </Show>
          <div class="logs-scroll">
            <pre class="logs-content">{logs.data ?? t("dashboard.noLogsAvailable")}</pre>
          </div>
        </section>
      </div>
      <footer class="app-shell__footer">Made with &lt;3 by <a href="https://linkedin.com/in/neirth" target="_blank" rel="noopener noreferrer">@Neirth</a></footer>
    </AppShell>
  );
};

export default DashboardView;
