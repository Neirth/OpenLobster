// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from 'solid-js';
import { For, Show, Suspense, createMemo, createSignal, onCleanup } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useMcpServers, useMcpUsers, useMcpTools, useToolPermissions, useConfig } from '@openlobster/ui/hooks';
import {
  CONNECT_MCP_MUTATION,
  DISCONNECT_MCP_MUTATION,
  INITIATE_OAUTH_MUTATION,
  SET_TOOL_PERMISSION_MUTATION,
  DELETE_TOOL_PERMISSION_MUTATION,
  SET_ALL_TOOL_PERMISSIONS_MUTATION,
} from '@openlobster/ui/graphql/mutations';
import { client } from '../../graphql/client';
import AppShell from '../../components/AppShell/AppShell';
import Modal from '../../components/Modal/Modal';
import MarketplaceModal from '../../components/MarketplaceModal/MarketplaceModal';
import { t } from '../../App';
import './McpsView.css';

/** Built-in capability descriptors — ordered by relevance. MCP gateway is excluded
 * because it is managed implicitly via the Servers tab. Audio is excluded because
 * it is a model-level feature, not an agent tool. */
const BUILTIN_TOOLS: Array<{ key: 'browser' | 'terminal' | 'subagents' | 'memory' | 'filesystem' | 'sessions'; icon: string }> = [
  { key: 'browser',    icon: 'language'      },
  { key: 'terminal',   icon: 'terminal'      },
  { key: 'subagents',  icon: 'device_hub'    },
  { key: 'memory',     icon: 'memory_alt'    },
  { key: 'filesystem', icon: 'folder_open'   },
  { key: 'sessions',   icon: 'forum'         },
];

/** Real tools exposed to the LLM per built-in capability. */
const BUILTIN_TOOL_DETAILS: Record<string, Array<{ name: string; description: string }>> = {
  browser: [
    { name: 'browser_fetch',       description: 'Fetch and read the content of a web page' },
    { name: 'browser_screenshot',  description: 'Take a screenshot of the current page' },
    { name: 'browser_click',       description: 'Click a DOM element by CSS selector' },
    { name: 'browser_fill_input',  description: 'Fill a form input field' },
  ],
  terminal: [
    { name: 'terminal_exec',   description: 'Execute a command synchronously (blocks until complete)' },
    { name: 'terminal_spawn',  description: 'Launch a process in background (non-blocking, master agent only)' },
  ],
  memory: [
    { name: 'add_memory',     description: "Add a fact or piece of information to the agent's memory about the user" },
    { name: 'search_memory',  description: "Search the agent's memory for similar information" },
  ],
  subagents: [
    { name: 'subagent_spawn',  description: 'Spawn a sub-agent with a specific task (master agent only)' },
    { name: 'task_add',        description: 'Add a task to the heartbeat task queue' },
    { name: 'task_done',       description: 'Mark a task as completed' },
    { name: 'task_list',       description: 'List pending tasks in the heartbeat queue' },
  ],
  filesystem: [
    { name: 'read_file',     description: 'Read the contents of a file from the filesystem' },
    { name: 'write_file',    description: 'Write content to a file (creates or overwrites)' },
    { name: 'edit_file',     description: 'Apply a targeted edit to a file' },
    { name: 'list_content',  description: 'List directory contents' },
  ],
  sessions: [
    { name: 'send_message',    description: 'Send a message to another channel' },
    { name: 'send_file',       description: 'Send a file to a channel' },
    { name: 'schedule_cron',   description: 'Create or modify a cron job' },
  ],
};

/** Set of built-in tool names — used to avoid duplicating internal tools in MCP groups. */
const BUILTIN_TOOL_NAMES = new Set(
  Object.values(BUILTIN_TOOL_DETAILS).flatMap(tools => tools.map(t => t.name))
);

/**
 * Returns the favicon URL for an MCP server URL via the Google favicon service.
 * Handles subdomains, redirects, ICO/PNG differences, and size normalization
 * automatically — far more robust than fetching /favicon.ico directly.
 *
 * @param serverUrl - The full MCP server URL.
 * @returns A Google S2 favicon service URL for the given domain.
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

/**
 * Maps a ConnectionStatus value to the corresponding CSS custom property for
 * the status indicator dot.
 *
 * @param status - The server status from the hook.
 * @returns A CSS var() string.
 */
function statusColor(status: string): string {
  if (status === 'online')        return 'var(--color-success)';
  if (status === 'degraded')      return 'var(--color-warning)';
  if (status === 'unauthorized')  return '#f59e0b'; // amber
  return 'var(--color-error)';
}

const McpsView: Component = () => {
  const servers = useMcpServers(client);
  const queryClient = useQueryClient();

  // ── Section tab ─────────────────────────────────────────────────────────
  const [activeSection, setActiveSection] = createSignal<'servers' | 'builtin' | 'permissions'>('servers');
  const config = useConfig(client);

  // ── Built-in capability detail modal ────────────────────────────────────
  const [selectedBuiltin, setSelectedBuiltin] = createSignal<typeof BUILTIN_TOOLS[0] | null>(null);

  // ── Add Server modal state ───────────────────────────────────────────────
  const [showAddServerModal, setShowAddServerModal] = createSignal(false);
  const [showMarketplaceModal, setShowMarketplaceModal] = createSignal(false);
  const [addName, setAddName] = createSignal('');
  const [addUrl, setAddUrl] = createSignal('');
  const [addError, setAddError] = createSignal('');

  // ── Manage / OAuth modal state ───────────────────────────────────────────
  const [manageServerId, setManageServerId] = createSignal<string | null>(null);
  const [oauthStatus, setOauthStatus] = createSignal<'idle' | 'pending' | 'success' | 'error'>('idle');
  const [oauthError, setOauthError] = createSignal('');

  // ── Mutations ────────────────────────────────────────────────────────────
  const connectMcp = createMutation(() => ({
    mutationFn: (vars: { name: string; transport: string; url: string }) =>
      client.request<{ connectMcp: { name?: string; error?: string; requiresAuth?: boolean; url?: string } }>(
        CONNECT_MCP_MUTATION,
        vars,
      ),
    onSuccess: (data, vars) => {
      const res = data.connectMcp;
      if (res?.error && !res?.requiresAuth) {
        setAddError(res.error);
        return;
      }
      // Close modal and refresh list (server may be pending-auth)
      setShowAddServerModal(false);
      setAddName('');
      setAddUrl('');
      setAddError('');
      queryClient.invalidateQueries({ queryKey: ['mcpServers'] });
      // Auto-launch the OAuth flow if the server requires authorization
      if (res?.requiresAuth) {
        initiateOAuth.mutate({ name: vars.name, url: vars.url });
      }
    },
    onError: (err: Error) => setAddError(err.message),
  }));

  const disconnectMcp = createMutation(() => ({
    mutationFn: (vars: { name: string }) =>
      client.request<{ disconnectMcp: boolean }>(DISCONNECT_MCP_MUTATION, vars),
    onSuccess: () => {
      setManageServerId(null);
      queryClient.invalidateQueries({ queryKey: ['mcpServers'] });
      queryClient.invalidateQueries({ queryKey: ['mcpTools'] });
    },
  }));

  const initiateOAuth = createMutation(() => ({
    mutationFn: (vars: { name: string; url: string }) =>
      client.request<{ initiateOAuth: { success: boolean; authUrl?: string; error?: string } }>(
        INITIATE_OAUTH_MUTATION,
        vars,
      ),
    onSuccess: (data) => {
      const res = data.initiateOAuth;
      if (!res.success || !res.authUrl) {
        setOauthStatus('error');
        setOauthError(res.error ?? 'Unknown error');
        return;
      }
      setOauthStatus('pending');
      const popup = window.open(res.authUrl, 'oauth_popup', 'width=600,height=700');

      const handler = (event: MessageEvent) => {
        if (event.data?.type === 'oauth_success') {
          setOauthStatus('success');
          queryClient.invalidateQueries({ queryKey: ['mcpServers'] });
          queryClient.invalidateQueries({ queryKey: ['mcpTools'] });
          queryClient.invalidateQueries({ queryKey: ['toolPermissions'] });
          // El backend reconecta en goroutine; invalidar de nuevo tras 2s y 4s para capturar las nuevas herramientas
          setTimeout(() => {
            queryClient.invalidateQueries({ queryKey: ['mcpServers'] });
            queryClient.invalidateQueries({ queryKey: ['mcpTools'] });
            queryClient.invalidateQueries({ queryKey: ['toolPermissions'] });
          }, 2000);
          setTimeout(() => {
            queryClient.invalidateQueries({ queryKey: ['mcpServers'] });
            queryClient.invalidateQueries({ queryKey: ['mcpTools'] });
            queryClient.invalidateQueries({ queryKey: ['toolPermissions'] });
          }, 4000);
          window.removeEventListener('message', handler);
        } else if (event.data?.type === 'oauth_error') {
          setOauthStatus('error');
          setOauthError(event.data.error ?? 'Authorization denied');
          window.removeEventListener('message', handler);
        }
      };
      window.addEventListener('message', handler);

      // Poll whether the popup is closed without postMessage (user closed manually)
      const poll = setInterval(() => {
        if (popup?.closed) {
          clearInterval(poll);
          window.removeEventListener('message', handler);
          if (oauthStatus() === 'pending') setOauthStatus('idle');
        }
      }, 1000);

      onCleanup(() => {
        clearInterval(poll);
        window.removeEventListener('message', handler);
      });
    },
    onError: (err: Error) => {
      setOauthStatus('error');
      setOauthError(err.message);
    },
  }));

  // ── Permissions state ────────────────────────────────────────────────────
  const mcpUsers = useMcpUsers(client);
  const mcpTools = useMcpTools(client);
  const [selectedUserId, setSelectedUserId] = createSignal('');
  const perms = useToolPermissions(client, selectedUserId);
  /** Set of group keys that are currently collapsed. */
  const [collapsedGroups, setCollapsedGroups] = createSignal(new Set<string>());
  const toggleGroup = (key: string) => {
    setCollapsedGroups(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  };
  /** Set of tools explicitly denied for the selected user.
   * Everything else is allowed by default. */
  const deniedTools = createMemo(() => {
    const set = new Set<string>();
    for (const p of perms.data ?? []) {
      if (p.mode === 'deny') set.add(p.toolName);
    }
    return set;
  });
  /** Tools grouped by serverName for the permissions matrix.
   * Built-in capability tools are prepended as groups. MCP tools are grouped by server. */
  const groupedTools = createMemo(() => {
    type ToolItem = NonNullable<typeof mcpTools.data>[number];
    const groups = new Map<string, ToolItem[]>();

    // Prepend built-in capability groups (single source of truth for internal tools).
    for (const cap of BUILTIN_TOOLS) {
      const tools = BUILTIN_TOOL_DETAILS[cap.key] ?? [];
      if (tools.length > 0) {
        const groupName = `__builtin__:${cap.key}`;
        groups.set(groupName, tools.map(tool => ({
          name: tool.name,
          serverName: groupName,
          description: tool.description,
        })));
      }
    }

    // Append MCP server tool groups. Skip internal tools (already in built-in).
    // Use serverName from API, or parse from "serverName:toolName" format.
    for (const tool of mcpTools.data ?? []) {
      if (BUILTIN_TOOL_NAMES.has(tool.name)) continue; // avoid duplication
      const serverKey = tool.serverName ?? (tool.name.includes(':') ? tool.name.split(':')[0]! : null);
      if (!serverKey) continue; // skip tools we cannot group (no "unknown")
      if (!groups.has(serverKey)) groups.set(serverKey, []);
      groups.get(serverKey)!.push(tool);
    }
    return groups;
  });
  const setPermission = createMutation(() => ({
    mutationFn: (vars: { userId: string; toolName: string; mode: string }) =>
      client.request(SET_TOOL_PERMISSION_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['toolPermissions', selectedUserId()] });
    },
  }));
  const deletePermission = createMutation(() => ({
    mutationFn: (vars: { userId: string; toolName: string }) =>
      client.request(DELETE_TOOL_PERMISSION_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['toolPermissions', selectedUserId()] });
    },
  }));
  const bulkPermission = createMutation(() => ({
    mutationFn: (vars: { userId: string; mode: string }) =>
      client.request(SET_ALL_TOOL_PERMISSIONS_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['toolPermissions', selectedUserId()] });
    },
  }));
  function toggleTool(toolName: string) {
    const uid = selectedUserId();
    if (!uid) return;
    if (!deniedTools().has(toolName)) {
      // Currently allowed → explicitly deny it.
      setPermission.mutate({ userId: uid, toolName, mode: 'deny' });
    } else {
      // Currently denied → remove explicit entry, reverts to allow-by-default.
      deletePermission.mutate({ userId: uid, toolName });
    }
  }

  const handleAddServer = () => {
    if (!addName() || !addUrl()) return;
    setAddError('');
    connectMcp.mutate({ name: addName(), transport: 'http', url: addUrl() });
  };

  const openManage = (name: string) => {
    setManageServerId(name);
    setOauthStatus('idle');
    setOauthError('');
  };

  const managedServer = () => servers.data?.find(s => s.name === manageServerId());

  return (
    <AppShell activeTab="mcps">
      <div class="mcps-view">
        {/* Section tabs */}
        <div class="mcps-section-tabs">
          <button
            class="mcps-section-tab"
            classList={{ 'mcps-section-tab--active': activeSection() === 'servers' }}
            onClick={() => setActiveSection('servers')}
          >
            <span class="material-symbols-outlined">hub</span>
            {t('mcps.servers')}
          </button>
          <button
            class="mcps-section-tab"
            classList={{ 'mcps-section-tab--active': activeSection() === 'builtin' }}
            onClick={() => setActiveSection('builtin')}
          >
            <span class="material-symbols-outlined">construction</span>
            {t('mcps.builtin')}
          </button>
          <button
            class="mcps-section-tab"
            classList={{ 'mcps-section-tab--active': activeSection() === 'permissions' }}
            onClick={() => setActiveSection('permissions')}
          >
            <span class="material-symbols-outlined">lock</span>
            {t('permissions.title')}
          </button>
        </div>

        {/* ── Servers section ─────────────────────────────────────────── */}
        <Show when={activeSection() === 'servers'}>
          <Show when={servers.data && servers.data.length > 0}>
            <div class="mcps-header">
              <div>
                <h1>{t('mcps.serversHeader')}</h1>
                <p>{t('mcps.serversDesc')}</p>
              </div>
              <div class="mcps-header-actions">
                <button class="marketplace-btn" onClick={() => setShowMarketplaceModal(true)}>
                  <span class="material-symbols-outlined">storefront</span>
                  {t('marketplace.button')}
                </button>
                <button class="add-server-btn" onClick={() => setShowAddServerModal(true)}>
                  + {t('mcps.addServer')}
                </button>
              </div>
            </div>
          </Show>

          <Show when={!servers.isLoading && (!servers.data || servers.data.length === 0)}>
            <div class="mcps-empty">
              <span class="material-symbols-outlined mcps-empty-icon">hub</span>
              <p class="mcps-empty-title">{t('mcps.noServers')}</p>
              <p class="mcps-empty-hint">{t('mcps.noServersHint')}</p>
              <div class="mcps-empty-actions">
                <button class="btn btn-md btn-secondary" onClick={() => setShowMarketplaceModal(true)}>
                  <span class="material-symbols-outlined">storefront</span>
                  {t('marketplace.button')}
                </button>
                <button class="btn btn-md btn-primary" onClick={() => setShowAddServerModal(true)}>
                  + {t('mcps.addServerBtn')}
                </button>
              </div>
            </div>
          </Show>

          <div class="servers-grid">
            <Suspense>
              <For each={servers.data}>
                {(server) => (
                  <div
                    class="server-card"
                    classList={{ 'server-card--inactive': server.status !== 'online' }}
                  >
                    <div class="server-header">
                      <div class="server-icon-container">
                        <Show
                          when={server.url}
                          fallback={
                            <span class="material-symbols-outlined server-icon">extension</span>
                          }
                        >
                          <img
                            class="server-favicon"
                            src={faviconUrl(server.url!)}
                            alt={server.name}
                            onError={(e) => {
                              (e.currentTarget as HTMLImageElement).style.display = 'none';
                              const fallback = e.currentTarget.nextElementSibling as HTMLElement | null;
                              if (fallback) fallback.style.display = '';
                            }}
                          />
                          <span class="material-symbols-outlined server-icon" style="display:none">extension</span>
                        </Show>
                      </div>
                      <h2 class="server-name">{server.name}</h2>
                    </div>

                    <div class="server-meta">
                      <span class="server-type-badge">{server.transport}</span>
                      <span class="tools-count">{server.toolCount}</span>
                      <span class="tools-label">{t('mcps.tools')}</span>
                      <button class="manage-btn" onClick={() => openManage(server.name)}>
                        {t('mcps.manage')}
                      </button>
                      <span
                        class="server-status-dot"
                        style={{ background: statusColor(server.status) }}
                      />
                    </div>
                  </div>
                )}
              </For>
            </Suspense>
          </div>
        </Show>

        {/* ── Built-in capabilities section ────────────────────────────── */}
        <Show when={activeSection() === 'builtin'}>
          <div class="builtin-section">
            <div class="builtin-header">
              <div>
                <h1>{t('mcps.builtinCapabilities')}</h1>
                <p class="builtin-header__hint">{t('mcps.builtinHint')}</p>
              </div>
            </div>

            <div class="builtin-grid">
              <For each={BUILTIN_TOOLS}>
                {(tool) => {
                  const globallyEnabled = () => !!(config.data?.capabilities?.[tool.key]);

                  return (
                    <div
                      class={`builtin-card ${globallyEnabled() ? 'builtin-card--active' : 'builtin-card--disabled'}`}
                      onClick={() => setSelectedBuiltin(tool)}
                      title={t('mcps.cap.viewTools')}
                    >
                      <div class="builtin-card__icon-wrap">
                        <span class="material-symbols-outlined builtin-card__icon">{tool.icon}</span>
                        <Show when={!globallyEnabled()}>
                          <span class="material-symbols-outlined builtin-card__badge builtin-card__badge--lock">lock</span>
                        </Show>
                        <Show when={globallyEnabled()}>
                          <span class="material-symbols-outlined builtin-card__badge builtin-card__badge--ok">check_circle</span>
                        </Show>
                      </div>
                      <div class="builtin-card__body">
                        <span class="builtin-card__name">{t(`mcps.cap.${tool.key}`)}</span>
                        <span class="builtin-card__desc">{t(`mcps.cap.${tool.key}Desc`)}</span>
                        <span class={`builtin-card__status builtin-card__status--${globallyEnabled() ? 'active' : 'disabled'}`}>
                          {globallyEnabled() ? t('mcps.capStatus.active') : t('mcps.capStatus.globallyDisabled')}
                        </span>
                      </div>
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>

        {/* ── Permissions section ──────────────────────────────────────── */}
        <Show when={activeSection() === 'permissions'}>
          <Show
            when={!mcpUsers.isLoading && (mcpUsers.data?.length ?? 0) > 0}
            fallback={
              <div class="mcps-empty permissions-empty">
                <span class="material-symbols-outlined mcps-empty-icon">person_off</span>
                <p class="mcps-empty-title">{t('permissions.noUsers')}</p>
                <p class="mcps-empty-hint">{t('permissions.noUsersHint')}</p>
              </div>
            }
          >
          <div class="permissions-layout">
            {/* Left panel — user list */}
            <aside class="permissions-sidebar">
              <div class="permissions-sidebar__header">
                <span class="material-symbols-outlined permissions-sidebar__icon">group</span>
                <span class="permissions-sidebar__title">{t('mcps.users')}</span>
              </div>
              <ul class="permissions-user-list">
                <For each={mcpUsers.data ?? []}>
                  {(user) => (
                    <li
                      class="permissions-user-item"
                      classList={{
                        'permissions-user-item--active': selectedUserId() === user.channelId,
                        'permissions-user-item--agent': !!user.isAgent,
                      }}
                      onClick={() => setSelectedUserId(user.channelId)}
                    >
                      <span class="material-symbols-outlined permissions-user-item__avatar">
                        {user.isAgent ? 'smart_toy' : 'account_circle'}
                      </span>
                      <span class="permissions-user-item__name">{user.displayName}</span>
                      <Show when={user.isAgent}>
                        <span class="permissions-user-item__agent-badge">{t('mcps.bot')}</span>
                      </Show>
                    </li>
                  )}
                </For>
              </ul>
            </aside>

            {/* Right panel — tool permission matrix */}
            <main class="permissions-main">
              <Show
                when={selectedUserId()}
                fallback={
                  <div class="permissions-empty-state">
                    <span class="material-symbols-outlined permissions-empty-state__icon">
                      lock
                    </span>
                    <p class="permissions-empty-state__text">{t('permissions.selectUser')}</p>
                  </div>
                }
              >
                <div class="permissions-main__header">
                  <div class="permissions-policy-note">
                    <span class="material-symbols-outlined permissions-policy-note__icon">
                      info
                    </span>
                    <span class="permissions-policy-note__text">
                      {t('permissions.defaultDeny')}
                    </span>
                  </div>
                  <div class="permissions-bulk-actions">
                    <button
                      class="btn btn-sm btn-bulk-allow"
                      disabled={bulkPermission.isPending || groupedTools().size === 0}
                      onClick={() => bulkPermission.mutate({ userId: selectedUserId(), mode: 'allow' })}
                    >
                      <span class="material-symbols-outlined">done_all</span>
                      {t('permissions.allowAll')}
                    </button>
                    <button
                      class="btn btn-sm btn-bulk-deny"
                      disabled={bulkPermission.isPending || groupedTools().size === 0}
                      onClick={() => bulkPermission.mutate({ userId: selectedUserId(), mode: 'deny' })}
                    >
                      <span class="material-symbols-outlined">remove_done</span>
                      {t('permissions.denyAll')}
                    </button>
                  </div>
                </div>

                <Show
                  when={groupedTools().size > 0}
                  fallback={
                    <div class="permissions-empty-state">
                      <span class="material-symbols-outlined permissions-empty-state__icon">
                        build_circle
                      </span>
                      <p class="permissions-empty-state__text">{t('permissions.noTools')}</p>
                      <p class="permissions-empty-state__hint">{t('permissions.noToolsHint')}</p>
                    </div>
                  }
                >
                  <div class="permissions-tool-table">
                    <div class="permissions-tool-table__head">
                      <span>{t('permissions.tool')}</span>
                      <span class="permissions-tool-table__desc-col">
                        {t('permissions.description')}
                      </span>
                      <span>{t('permissions.status')}</span>
                    </div>
                    <ul class="permissions-tool-table__body">
                      <For each={[...groupedTools().entries()]}>
                        {([groupKey, tools]) => {
                          const isBuiltin = groupKey.startsWith('__builtin__:');
                          const capKey = isBuiltin ? groupKey.replace('__builtin__:', '') : null;
                          const cap = capKey ? BUILTIN_TOOLS.find(c => c.key === capKey) : null;
                          const displayName = capKey ? t(`mcps.cap.${capKey}`) : groupKey;
                          const icon = cap ? cap.icon : 'extension';
                          return (
                            <>
                              <li
                                class="permissions-tool-group__header"
                                onClick={() => toggleGroup(groupKey)}
                              >
                                <span class="material-symbols-outlined permissions-tool-group__icon">{icon}</span>
                                <span class="permissions-tool-group__name">{displayName}</span>
                                <span class="permissions-tool-group__count">{tools.length}</span>
                                <span
                                  class="material-symbols-outlined permissions-tool-group__chevron"
                                  classList={{ 'permissions-tool-group__chevron--collapsed': collapsedGroups().has(groupKey) }}
                                >expand_more</span>
                              </li>
                              <Show when={!collapsedGroups().has(groupKey)}>
                                <For each={tools}>
                                {(tool) => {
                                  const allowed = createMemo(() => !deniedTools().has(tool.name));
                                  return (
                                    <li class="permissions-tool-row">
                                      <span class="permissions-tool-row__name">{tool.name}</span>
                                      <span class="permissions-tool-row__desc">{tool.description}</span>
                                      <button
                                        class="permissions-toggle"
                                        classList={{
                                          'permissions-toggle--allow': allowed(),
                                          'permissions-toggle--deny': !allowed(),
                                        }}
                                        onClick={() => toggleTool(tool.name)}
                                        title={allowed() ? t('permissions.deny') : t('permissions.allow')}
                                      >
                                        <span class="material-symbols-outlined permissions-toggle__icon">
                                          {allowed() ? 'check_circle' : 'block'}
                                        </span>
                                        <span class="permissions-toggle__label">
                                          {allowed() ? t('permissions.allow') : t('permissions.deny')}
                                        </span>
                                      </button>
                                    </li>
                                  );
                                }}
                              </For>
                              </Show>
                            </>
                          );
                        }}
                      </For>
                    </ul>
                  </div>
                </Show>
              </Show>
            </main>
          </div>
          </Show>
        </Show>

        {/* Built-in Capability Detail Modal */}
        <Modal
          isOpen={selectedBuiltin() !== null}
          onClose={() => setSelectedBuiltin(null)}
          title={selectedBuiltin() ? t(`mcps.cap.${selectedBuiltin()!.key}`) : ''}
        >
          <Show when={selectedBuiltin()}>
            {(cap) => {
              const globallyEnabled = () => !!(config.data?.capabilities?.[cap().key]);
              const tools = BUILTIN_TOOL_DETAILS[cap().key] ?? [];
              return (
                <div class="modal-form">
                  <div class="modal-section">
                    <div class="builtin-detail-meta">
                      <span class="material-symbols-outlined builtin-detail-meta__icon">{cap().icon}</span>
                      <div>
                        <p class="builtin-detail-meta__desc">{t(`mcps.cap.${cap().key}Desc`)}</p>
                        <span class={`builtin-detail-meta__status${globallyEnabled() ? '--active' : '--disabled'}`}>
                          {globallyEnabled() ? t('mcps.capStatus.active') : t('mcps.capStatus.globallyDisabled')}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div class="modal-section">
                    <h4 class="section-title">{t('mcps.cap.toolsExposed')}</h4>
                    <ul class="server-tools__list">
                      <For each={tools} fallback={<p class="section-text">{t('mcps.noTools')}</p>}>
                        {(tool) => (
                          <li class="server-tools__item">
                            <span class="server-tools__item-name">{tool.name}</span>
                            <span class="server-tools__item-desc">{tool.description}</span>
                          </li>
                        )}
                      </For>
                    </ul>
                  </div>
                  <div class="form-actions">
                    <button class="btn btn-md btn-secondary" onClick={() => setSelectedBuiltin(null)}>
                      {t('common.close')}
                    </button>
                  </div>
                </div>
              );
            }}
          </Show>
        </Modal>

        {/* Add Server Modal */}
        <Modal
          isOpen={showAddServerModal()}
          onClose={() => { setShowAddServerModal(false); setAddError(''); }}
          title={t('mcps.addServer')}
        >
          <div class="modal-form">
            <div class="form-group">
              <label>{t('mcps.serverName')}</label>
              <input
                type="text"
                placeholder={t('mcps.serverNamePlaceholder')}
                value={addName()}
                onInput={(e) => setAddName(e.currentTarget.value)}
              />
            </div>
            <div class="form-group">
              <label>{t('mcps.serverUrl')}</label>
              <input
                type="url"
                placeholder={t('mcps.serverUrlPlaceholder')}
                value={addUrl()}
                onInput={(e) => setAddUrl(e.currentTarget.value)}
              />
            </div>
            <p class="modal-transport-note">
              <span class="material-symbols-outlined">lock</span>
              {t('mcps.httpOnlyNote')}
            </p>
            <Show when={addError()}>
              <p class="modal-form-error">{addError()}</p>
            </Show>
            <div class="form-actions">
              <button
                class="btn btn-md btn-secondary"
                onClick={() => { setShowAddServerModal(false); setAddError(''); }}
              >
                {t('common.cancel')}
              </button>
              <button
                class="btn btn-md btn-primary"
                disabled={connectMcp.isPending}
                onClick={handleAddServer}
              >
                {connectMcp.isPending ? t('common.loading') : t('mcps.addServerBtn')}
              </button>
            </div>
          </div>
        </Modal>

        {/* Manage Server Modal */}
        <Modal
          isOpen={manageServerId() !== null}
          onClose={() => setManageServerId(null)}
          title={`${t('mcps.manageServer')}: ${manageServerId() ?? ''}`}
        >
          <div class="modal-form">
            <div class="modal-section">
              <h4 class="section-title">{t('mcps.serverTools')}</h4>
              <div class="server-tools">
                <Show
                  when={(mcpTools.data?.filter(tool => tool.serverName === manageServerId()) ?? []).length > 0}
                  fallback={<p class="section-text">{t('mcps.availableTools')}</p>}
                >
                  <ul class="server-tools__list">
                    <For each={mcpTools.data?.filter(tool => tool.serverName === manageServerId()) ?? []}>
                      {(tool) => (
                        <li class="server-tools__item">
                          <span class="server-tools__item-name">{tool.name}</span>
                          <Show when={tool.description}>
                            <span class="server-tools__item-desc">{tool.description}</span>
                          </Show>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
              </div>
            </div>
            <div class="modal-section">
              <h4 class="section-title">{t('mcps.serverStatus')}</h4>
              <p class="section-text">{managedServer()?.status ?? t('status.unknown')}</p>
            </div>

            {/* OAuth authorization */}
            <Show when={managedServer() !== undefined}>
              <div class="modal-section">
                <h4 class="section-title">{t('mcps.oauth')}</h4>
                <Show when={oauthStatus() === 'pending'}>
                  <p class="oauth-status oauth-status--pending">{t('mcps.oauthPending')}</p>
                </Show>
                <Show when={oauthStatus() === 'success'}>
                  <p class="oauth-status oauth-status--success">
                    <span class="material-symbols-outlined">check_circle</span>
                    {t('mcps.oauthSuccess')}
                  </p>
                </Show>
                <Show when={oauthStatus() === 'error'}>
                  <p class="oauth-status oauth-status--error">
                    {t('mcps.oauthError')}: {oauthError()}
                  </p>
                </Show>
                <Show when={oauthStatus() !== 'pending'}>
                  <button
                    class="btn btn-md btn-oauth"
                    disabled={initiateOAuth.isPending}
                    onClick={() => {
                      const s = managedServer();
                      if (s) {
                        setOauthStatus('idle');
                        setOauthError('');
                        initiateOAuth.mutate({ name: s.name, url: s.url ?? '' });
                      }
                    }}
                  >
                    <span class="material-symbols-outlined">lock_open</span>
                    {t('mcps.authorizeOAuth')}
                  </button>
                </Show>
              </div>
            </Show>

            <div class="form-actions">
              <button
                class="btn btn-md btn-danger"
                disabled={disconnectMcp.isPending}
                onClick={() => {
                  const s = managedServer();
                  if (s) disconnectMcp.mutate({ name: s.name });
                }}
              >
                {t('mcps.removeServer')}
              </button>
              <button
                class="btn btn-md btn-secondary"
                onClick={() => setManageServerId(null)}
              >
                {t('common.close')}
              </button>
            </div>
          </div>
        </Modal>

        {/* Marketplace Modal */}
        <MarketplaceModal
          isOpen={showMarketplaceModal()}
          onClose={() => setShowMarketplaceModal(false)}
          onAdd={(server) => {
            setAddName(server.name);
            setAddUrl(server.url);
            setAddError('');
            if (server.oauth) {
              initiateOAuth.mutate({ name: server.name, url: server.url });
            } else {
              connectMcp.mutate({ name: server.name, transport: 'http', url: server.url });
            }
          }}
        />
      </div>
    </AppShell>
  );
};

export default McpsView;
