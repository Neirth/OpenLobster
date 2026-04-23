// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createSignal, createMemo, onMount, For, Show, type Component } from "solid-js";
import { useQueryClient, createMutation, createQuery } from "@tanstack/solid-query";
import { t, getStoredToken } from "@/App";
import { setNeedsAuth } from "@/stores/authStore";
import {
  CONFIG_QUERY,
  PLUGINS_QUERY,
} from "@/graphql/queries";
import {
  CONNECT_MCP_MUTATION,
  UPDATE_CONFIG_MUTATION,
} from "@/graphql/mutations";
import { GRAPHQL_ENDPOINT } from "@/graphql/constants";
import { client } from "@/graphql/config";
import type { SchemaProperty } from "@/schemas/config.schema";
import { WizardField } from "./WizardField";
import { WizardSchemaField } from "./WizardSchemaField";
import "./WizardView.css";

interface MarketplaceServer {
  id: string;
  name: string;
  company: string;
  description: string;
  url: string;
  homepage?: string;
  transport?: string;
  category?: string;
  oauth?: boolean;
}

interface WizardPlugin {
  id: string;
  name: string;
  pluginType: string;
  available: boolean;
  schemaJson?: string;
  configJson?: string;
}

const fetchMarketplace = async (): Promise<MarketplaceServer[]> => {
  try {
    const res = await fetch("/marketplace.json");
    if (!res.ok) return [];
    return await res.json();
  } catch (err) {
    console.error("Failed to fetch marketplace", err);
    return [];
  }
};

const fetchWizardPlugins = async (): Promise<WizardPlugin[]> => {
  const res = await fetch(GRAPHQL_ENDPOINT, {
    method: "POST",
    headers: graphqlHeaders(),
    body: JSON.stringify({ query: PLUGINS_QUERY }),
  });
  if (res.status === 401) {
    setNeedsAuth(true);
    return [];
  }
  const data = await res.json();
  const plugins = data?.data?.plugins;
  if (!Array.isArray(plugins)) {
    return [];
  }
  return plugins
    .filter((plugin) => typeof plugin?.id === "string")
    .map((plugin) => ({
      id: plugin.id as string,
      name: (plugin.name as string | undefined) ?? (plugin.id as string),
      pluginType: (plugin.pluginType as string | undefined) ?? "",
      available: plugin.available !== false,
      schemaJson: plugin.schemaJson as string | undefined,
      configJson: plugin.configJson as string | undefined,
    }));
};

function faviconUrl(url: string, homepage?: string): string {
  try {
    const { hostname } = new URL(homepage ?? url);
    const parts = hostname.split(".");
    const rootDomain = parts.length > 2 ? parts.slice(-2).join(".") : hostname;
    return `https://www.google.com/s2/favicons?domain=${rootDomain}&sz=32`;
  } catch {
    return "";
  }
}

const TOTAL_STEPS = 8;

function graphqlHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

const CAPABILITIES = [
  { key: "browser", icon: "language" },
  { key: "terminal", icon: "terminal" },
  { key: "subagents", icon: "device_hub" },
  { key: "memory", icon: "memory_alt" },
  { key: "mcp", icon: "extension" },
  { key: "filesystem", icon: "folder_open" },
  { key: "sessions", icon: "forum" },
] as const;

const WIZARD_STORAGE_KEY = "openlobster_wizard_completed";

export function isFirstBoot(): boolean {
  if (typeof window === "undefined") return false;
  return localStorage.getItem(WIZARD_STORAGE_KEY) !== "true";
}

export function setWizardCompleted(): void {
  if (typeof window !== "undefined") {
    localStorage.setItem(WIZARD_STORAGE_KEY, "true");
  }
}

function getDefaultFormValues(): Record<string, unknown> {
  return {
    agentName: "OpenLobster",
    model: "llama3.2:latest",
    capabilities: {
      browser: false,
      terminal: false,
      subagents: true,
      memory: true,
      mcp: true,
      filesystem: true,
      sessions: true,
    },
    pluginDefaultAi: "",
    pluginDefaultMemory: "",
    pluginDefaultSecrets: "",
    pluginDefaultAudio: "",
    a2aEnabled: true,
    graphqlBaseUrl: typeof window !== "undefined" ? window.location.origin : "",
  };
}



export interface WizardViewProps {
  onComplete: () => void;
}

function getValueAtPath(obj: Record<string, unknown> | undefined, path: string): unknown {
  if (!obj) return undefined;
  return path.split(".").reduce((acc, part) => (acc && typeof acc === "object") ? (acc as Record<string, unknown>)[part] : undefined, obj as unknown);
}

const WizardView: Component<WizardViewProps> = (props) => {
  const queryClient = useQueryClient();
  const [step, setStep] = createSignal(0);
  const [formValues, setFormValues] = createSignal<Record<string, unknown>>(
    getDefaultFormValues(),
  );
  const [isLoading, setIsLoading] = createSignal(true);
  const [isSaving, setIsSaving] = createSignal(false);
  const [saveError, setSaveError] = createSignal<string | null>(null);
  const [marketplaceSearch, setMarketplaceSearch] = createSignal("");
  const [marketplaceSelected, setMarketplaceSelected] = createSignal<MarketplaceServer | null>(null);
  const [marketplaceError, setMarketplaceError] = createSignal<string | null>(null);

  const wizardPlugins = createQuery(() => ({
    queryKey: ["wizardPlugins"],
    queryFn: fetchWizardPlugins,
  }));

  const marketplaceServers = createQuery(() => ({
    queryKey: ["marketplaceServers"],
    queryFn: fetchMarketplace,
  }));

  const [detailName, setDetailName] = createSignal("");
  const [detailUrl, setDetailUrl] = createSignal("");

  onMount(() => {
    if (marketplaceSelected()) {
      setDetailName(marketplaceSelected()!.name);
      setDetailUrl(marketplaceSelected()!.url);
    }
  });

  const marketplaceFiltered = createMemo(() => {
    const list = marketplaceServers.data ?? [];
    const q = marketplaceSearch().toLowerCase();
    if (!q) return list;
    return list.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        (s.category ?? "").toLowerCase().includes(q),
    );
  });

  const pluginOptions = createMemo(() => {
    const sorted = (wizardPlugins.data ?? [])
      .filter((plugin) => plugin.available && plugin.pluginType)
      .slice()
      .sort((a, b) => a.name.localeCompare(b.name));

    return {
      ai: sorted.filter((plugin) => plugin.pluginType === "ai"),
      memory: sorted.filter((plugin) => plugin.pluginType === "memory"),
      secrets: sorted.filter((plugin) => plugin.pluginType === "secrets"),
      audio: sorted.filter((plugin) => plugin.pluginType === "audio"),
      messaging: sorted.filter((plugin) => plugin.pluginType === "messaging"),
    };
  });

  const connectMcp = createMutation(() => ({
    mutationFn: (vars: { name: string; transport: string; url: string }) =>
      client.request<{
        connectMcp: { success?: boolean; error?: string; requiresAuth?: boolean; url?: string };
      }>(CONNECT_MCP_MUTATION, vars),
    onSuccess: (data) => {
      const res = data.connectMcp;
      setMarketplaceError(null);
      if (res?.error) {
        setMarketplaceError(res.error);
        return;
      }
      if (res?.requiresAuth && res?.url) {
        window.open(res.url, "_blank");
        return;
      }
      setMarketplaceSelected(null);
      queryClient.invalidateQueries({ queryKey: ["mcpServers"] });
    },
    onError: (err) => {
      setMarketplaceError(err instanceof Error ? err.message : t("settings.saveError"));
    },
  }));

  const handleFieldChange = (field: string, value: unknown) => {
    setFormValues((prev) => {
      const next = { ...prev };
      const blockedKeys = new Set(["__proto__", "constructor", "prototype"]);
      if (field.includes(".")) {
        const parts = field.split(".");
        if (parts.some((part) => blockedKeys.has(part))) {
          return prev;
        }
        let target: Record<string, unknown> = next;
        for (let i = 0; i < parts.length - 1; i++) {
          const part = parts[i];
          if (!target[part] || typeof target[part] !== "object") {
            target[part] = {};
          }
          target = target[part] as Record<string, unknown>;
        }
        target[parts[parts.length - 1]] = value;
      } else {
        if (blockedKeys.has(field)) {
          return prev;
        }
        next[field] = value;
      }
      return next;
    });
  };

  onMount(async () => {
    try {
      setIsLoading(true);
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({ query: CONFIG_QUERY }),
      });
      if (res.status === 401) {
        setNeedsAuth(true);
        return;
      }
      const data = await res.json();
      const config = data?.data?.config;
      if (config) {
        setFormValues({
          ...getDefaultFormValues(),
          agentName: config.agent?.name ?? "OpenLobster",
          model: config.agent?.model ?? "llama3.2:latest",
          pluginDefaultAi: config.plugins?.defaultAi ?? "",
          pluginDefaultMemory: config.plugins?.defaultMemory ?? "",
          pluginDefaultSecrets: config.plugins?.defaultSecrets ?? "",
          pluginDefaultAudio: config.plugins?.defaultAudio ?? "",
          capabilities: {
            ...((getDefaultFormValues().capabilities as Record<string, unknown>) || {}),
            ...((config.agent?.capabilities as Record<string, unknown>) ?? {}),
          },
          graphqlBaseUrl: config.agent?.graphqlBaseUrl ?? (typeof window !== "undefined" ? window.location.origin : ""),
        });
      }
    } catch (err) {
      console.error("Failed to load config", err);
    } finally {
      setIsLoading(false);
    }
  });

  const canGoNext = () => {
    if (step() === 1) return !!formValues().agentName;
    return true;
  };

  const goNext = () => {
    if (canGoNext()) setStep((s) => Math.min(s + 1, TOTAL_STEPS - 1));
  };

  const goBack = () => {
    if (step() === 4 && marketplaceSelected()) {
      setMarketplaceSelected(null);
    } else {
      setStep((s) => Math.max(s - 1, 0));
    }
  };

  const handleAddMarketplace = async (server: MarketplaceServer) => {
    setMarketplaceError(null);
    connectMcp.mutate({
      name: detailName() || server.name,
      transport: server.transport || "sse",
      url: detailUrl() || server.url,
    });
  };

  const handleSaveAndFinish = async () => {
    try {
      setIsSaving(true);
      setSaveError(null);

      const values = formValues();
      const pluginsProp = values.plugins as Record<string, Record<string, Record<string, unknown>>> | undefined;
      const enabled = pluginsProp?.enabled || {};
      const settings = pluginsProp?.settings || {};

      const variables = {
        input: {
          agentName: values.agentName,
          model: values.model,
          graphqlBaseUrl: values.graphqlBaseUrl,
          pluginDefaultAi: values.pluginDefaultAi,
          pluginDefaultMemory: values.pluginDefaultMemory,
          pluginDefaultSecrets: values.pluginDefaultSecrets,
          pluginDefaultAudio: values.pluginDefaultAudio,
          capabilities: values.capabilities,
          
          // Messaging channels mapping
          channelTelegramEnabled: !!enabled["openlobster-messages-telegram"],
          channelTelegramToken: settings["openlobster-messages-telegram"]?.token || "",
          
          channelDiscordEnabled: !!enabled["openlobster-messages-discord"],
          channelDiscordToken: settings["openlobster-messages-discord"]?.token || "",
          
          channelSlackEnabled: !!enabled["openlobster-messages-slack"],
          channelSlackBotToken: settings["openlobster-messages-slack"]?.botToken || "",
          channelSlackAppToken: settings["openlobster-messages-slack"]?.appToken || "",
          
          wizardCompleted: true,
        },
      };

      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({
          query: UPDATE_CONFIG_MUTATION,
          variables,
        }),
      });

      const data = await res.json();
      if (data?.errors) {
        setSaveError(data.errors[0].message);
        return;
      }

      setWizardCompleted();
      props.onComplete();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div class="wizard-view">
      <div class="wizard-box">
        <div class="wizard-stepper">
          <For each={Array.from({ length: TOTAL_STEPS })}>
            {(_, i) => (
              <div
                class="wizard-step-dot"
                classList={{
                  "wizard-step-dot--active": i() <= step(),
                  "wizard-step-dot--current": i() === step(),
                }}
              />
            )}
          </For>
        </div>

        <div class="wizard-content">
          <Show when={isLoading()}>
            <div class="wizard-loading">
              <span class="material-symbols-outlined wizard-loading-icon spinning">hourglass_empty</span>
              <p>{t("common.loading")}</p>
            </div>
          </Show>

          <Show when={!isLoading()}>
            {/* Step 0: Welcome */}
            <Show when={step() === 0}>
              <div class="wizard-step wizard-step--welcome">
                <span class="material-symbols-outlined wizard-welcome-icon">auto_awesome</span>
                <h2>{t("wizard.welcome.title")}</h2>
                <p>{t("wizard.welcome.description")}</p>
              </div>
            </Show>

            {/* Step 1: Identity */}
            <Show when={step() === 1}>
              <div class="wizard-step">
                <h2>{t("wizard.agentConfig.title")}</h2>
                <p>{t("wizard.agentConfig.description")}</p>
                <div class="wizard-form">
                  <WizardField 
                    label={t("settings.field.agentName")}
                    description={t("settings.field.agentNameDesc")}
                  >
                    <input
                      type="text"
                      value={(formValues().agentName as string) ?? ""}
                      onInput={(e) => handleFieldChange("agentName", e.currentTarget.value)}
                      placeholder="e.g. agent-01, MyPersonalAgent"
                    />
                  </WizardField>
                  <WizardField 
                    label={t("settings.field.graphqlBaseUrl")}
                    description={t("settings.field.graphqlBaseUrlDesc")}
                  >
                    <input
                      type="text"
                      value={(formValues().graphqlBaseUrl as string) ?? ""}
                      onInput={(e) => handleFieldChange("graphqlBaseUrl", e.currentTarget.value)}
                      placeholder="https://openlobster.example.com"
                    />
                  </WizardField>
                </div>
              </div>
            </Show>

            {/* Step 2: Intelligence - Brain */}
            <Show when={step() === 2}>
              <div class="wizard-step">
                <h2>{t("wizard.aiProvider.title")}</h2>
                <p>{t("wizard.aiProvider.description")}</p>
                <div class="wizard-form">
                  <WizardField 
                    label={t("settings.field.provider")}
                  >
                    <select
                      value={(formValues().pluginDefaultAi as string) ?? ""}
                      onChange={(e) => handleFieldChange("pluginDefaultAi", e.currentTarget.value)}
                    >
                      <option value="">{t("plugins.defaultAuto")}</option>
                      <For each={pluginOptions().ai}>
                        {(p) => <option value={p.id}>{p.name}</option>}
                      </For>
                    </select>
                  </WizardField>

                  <Show when={formValues().pluginDefaultAi}>
                    {(selectedId) => {
                      const plugin = () =>
                        pluginOptions().ai.find((p) => p.id === selectedId());
                      const schema = () =>
                        plugin()?.schemaJson ? JSON.parse(plugin()!.schemaJson!) : null;
                      return (
                        <Show when={schema()}>
                          <div class="wizard-plugin-dynamic-fields">
                            <For each={Object.entries(schema().properties || {})}>
                              {([key, prop]) => (
                                <WizardSchemaField
                                  field={`plugins.settings.${selectedId()}.${key}`}
                                  schema={prop as unknown as SchemaProperty}
                                  values={(getValueAtPath(formValues(), `plugins.settings.${selectedId()}`) as Record<string, unknown>) || {}}
                                  onChange={handleFieldChange}
                                />
                              )}
                            </For>
                          </div>
                        </Show>
                      );
                    }}
                  </Show>
                </div>
              </div>
            </Show>

            {/* Step 3: Skills - Capabilities */}
            <Show when={step() === 3}>
              <div class="wizard-step">
                <h2>{t("wizard.capabilities.title")}</h2>
                <p>{t("wizard.capabilities.description")}</p>
                <div class="wizard-capabilities">
                  <For each={CAPABILITIES}>
                    {(cap) => {
                      const caps = () =>
                        (formValues().capabilities as Record<string, boolean>) ?? {};
                      const checked = () => !!caps()[cap.key];
                      return (
                        <label class="wizard-capability-card">
                          <input
                            type="checkbox"
                            checked={checked()}
                            onChange={(e) =>
                              handleFieldChange(`capabilities.${cap.key}`, e.currentTarget.checked)
                            }
                          />
                          <span class="material-symbols-outlined wizard-cap-icon">{cap.icon}</span>
                          <span class="wizard-cap-label">{t(`mcps.cap.${cap.key}`)}</span>
                        </label>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            {/* Step 4: Tools - Marketplace */}
            <Show when={step() === 4}>
              <div class="wizard-step wizard-step--marketplace">
                <Show
                  when={marketplaceSelected()}
                  keyed
                  fallback={
                    <>
                      <h2>{t("wizard.marketplace.title")}</h2>
                      <p>{t("wizard.marketplace.description")}</p>
                      <Show when={marketplaceError()}>
                        <p class="wizard-error wizard-marketplace-error">{marketplaceError()}</p>
                      </Show>
                      <div class="wizard-marketplace-search">
                        <span class="material-symbols-outlined">search</span>
                        <input
                          type="search"
                          placeholder={t("marketplace.searchPlaceholder")}
                          value={marketplaceSearch()}
                          onInput={(e) => setMarketplaceSearch(e.currentTarget.value)}
                          autocomplete="off"
                        />
                      </div>
                      <div class="wizard-marketplace-body">
                        <Show when={marketplaceServers.isLoading}>
                          <div class="wizard-marketplace-loading">
                            <span class="material-symbols-outlined spinning">hourglass_empty</span>
                            <p>{t("marketplace.loading")}</p>
                          </div>
                        </Show>
                        <Show when={!marketplaceServers.isLoading}>
                          <div class="wizard-marketplace-grid">
                            <For each={marketplaceFiltered()}>
                              {(server) => (
                                <button
                                  class="wizard-marketplace-card"
                                  onClick={() => setMarketplaceSelected(server)}
                                >
                                  <div class="wizard-marketplace-card__icon">
                                    <img src={faviconUrl(server.url, server.homepage)} alt="" />
                                  </div>
                                  <div class="wizard-marketplace-card__body">
                                    <span class="wizard-marketplace-card__name">{server.name}</span>
                                    <span class="wizard-marketplace-card__company">{server.company}</span>
                                    <p class="wizard-marketplace-card__desc">{server.description}</p>
                                  </div>
                                  <span class="material-symbols-outlined wizard-marketplace-card__chevron">
                                    chevron_right
                                  </span>
                                </button>
                              )}
                            </For>
                          </div>
                        </Show>
                      </div>
                    </>
                  }
                >
                  {(server) => (
                    <div class="wizard-marketplace-detail">
                      <button class="wizard-marketplace-back" onClick={() => setMarketplaceSelected(null)}>
                        <span class="material-symbols-outlined">arrow_back</span>
                        {t("marketplace.back")}
                      </button>
                      <div class="wizard-marketplace-detail__hero">
                        <div class="wizard-marketplace-detail__icon">
                          <img src={faviconUrl(server.url, server.homepage)} alt="" />
                        </div>
                        <div>
                          <h3 class="wizard-marketplace-detail__name">{server.name}</h3>
                          <p class="wizard-marketplace-detail__company">{server.company}</p>
                        </div>
                      </div>
                      <p class="wizard-marketplace-detail__desc">{server.description}</p>
                      <div class="wizard-form wizard-marketplace-detail__form">
                        <WizardField 
                          label={t("marketplace.name")}
                          description={t("mcps.serverNamePlaceholder")}
                        >
                          <input
                            type="text"
                            value={detailName()}
                            onInput={(e) => setDetailName(e.currentTarget.value)}
                            placeholder={server.name}
                          />
                        </WizardField>
                        <WizardField 
                          label={t("marketplace.endpoint")}
                          description={t("mcps.serverUrlPlaceholder")}
                        >
                          <input
                            type="text"
                            value={detailUrl()}
                            onInput={(e) => setDetailUrl(e.currentTarget.value)}
                            placeholder={server.url}
                          />
                        </WizardField>
                      </div>
                      <Show when={marketplaceError()}>
                        <p class="wizard-error">{marketplaceError()}</p>
                      </Show>
                      <button
                        class="wizard-btn wizard-btn-primary"
                        onClick={() => handleAddMarketplace(server)}
                        disabled={connectMcp.isPending}
                      >
                        <span class="material-symbols-outlined">add_circle</span>
                        {connectMcp.isPending
                          ? t("common.loading")
                          : t("marketplace.connect")}
                      </button>
                    </div>
                  )}
                </Show>
              </div>
            </Show>

            {/* Step 5: Infrastructure - Knowledge */}
            <Show when={step() === 5}>
              <div class="wizard-step">
                <h2>{t("wizard.knowledge.title")}</h2>
                <p>{t("wizard.knowledge.description")}</p>
                <div class="wizard-form">
                  <WizardField 
                    label={t("plugins.defaultMemoryLabel")}
                    description={t("plugins.defaultMemoryDesc") || ""}
                  >
                    <select
                      value={(formValues().pluginDefaultMemory as string) ?? ""}
                      onChange={(e) => handleFieldChange("pluginDefaultMemory", e.currentTarget.value)}
                    >
                      <option value="">{t("plugins.defaultAuto")}</option>
                      <For each={pluginOptions().memory}>
                        {(p) => <option value={p.id}>{p.name}</option>}
                      </For>
                    </select>
                  </WizardField>
                  <Show when={formValues().pluginDefaultMemory}>
                    {(id) => {
                      const p = () => pluginOptions().memory.find((plug) => plug.id === id());
                      const schema = () => (p()?.schemaJson ? JSON.parse(p()!.schemaJson!) : null);
                      return (
                        <Show when={schema()}>
                          <div class="wizard-plugin-dynamic-fields">
                            <For each={Object.entries(schema().properties || {})}>
                              {([k, v]) => (
                                <WizardSchemaField
                                  field={`plugins.settings.${id()}.${k}`}
                                  schema={v as unknown as SchemaProperty}
                                  values={(getValueAtPath(formValues(), `plugins.settings.${id()}`) as Record<string, unknown>) || {}}
                                  onChange={handleFieldChange}
                                />
                              )}
                            </For>
                          </div>
                        </Show>
                      );
                    }}
                  </Show>

                  <WizardField 
                    label={t("plugins.defaultSecretsLabel")}
                    description={t("plugins.defaultSecretsDesc") || ""}
                  >
                    <select
                      value={(formValues().pluginDefaultSecrets as string) ?? ""}
                      onChange={(e) => handleFieldChange("pluginDefaultSecrets", e.currentTarget.value)}
                    >
                      <option value="">{t("plugins.defaultAuto")}</option>
                      <For each={pluginOptions().secrets}>
                        {(p) => <option value={p.id}>{p.name}</option>}
                      </For>
                    </select>
                  </WizardField>
                  <Show when={formValues().pluginDefaultSecrets}>
                    {(id) => {
                      const p = () => pluginOptions().secrets.find((plug) => plug.id === id());
                      const schema = () => (p()?.schemaJson ? JSON.parse(p()!.schemaJson!) : null);
                      return (
                        <Show when={schema()}>
                          <div class="wizard-plugin-dynamic-fields">
                            <For each={Object.entries(schema().properties || {})}>
                              {([k, v]) => (
                                <WizardSchemaField
                                  field={`plugins.settings.${id()}.${k}`}
                                  schema={v as unknown as SchemaProperty}
                                  values={(getValueAtPath(formValues(), `plugins.settings.${id()}`) as Record<string, unknown>) || {}}
                                  onChange={handleFieldChange}
                                />
                              )}
                            </For>
                          </div>
                        </Show>
                      );
                    }}
                  </Show>
                </div>
              </div>
            </Show>

            {/* Step 6: Connectivity - Messaging Plugins */}
            {/* BUILD_VERSION_V4_ULTIMATE_REDEMPTION */}
            <Show when={step() === 6}>
              <div class="wizard-step">
                <h2>{t("wizard.connectivity.title")}</h2>
                <p>{t("wizard.connectivity.description")}</p>
                <div class="wizard-form">
                  <For each={pluginOptions().messaging}>
                    {(p) => {
                      const enabledKey = `plugins.enabled.${p.id}`;
                      const enabled = () => !!getValueAtPath(formValues(), enabledKey);
                      const schema = () => (p.schemaJson ? JSON.parse(p.schemaJson) : null);

                      const channelLabels: Record<string, { title: string, desc: string }> = {
                        discord: { 
                          title: "Discord", 
                          desc: "Conecta OpenLobster a un servidor o canal de Discord para interactuar con tu agente." 
                        },
                        slack: { 
                          title: "Slack", 
                          desc: "Habilita la integración con Slack para recibir mensajes y comandos en tus espacios de trabajo." 
                        },
                        telegram: { 
                          title: "Telegram", 
                          desc: "Usa Telegram para hablar con tu agente de forma segura desde cualquier lugar." 
                        }
                      };

                      const info = channelLabels[p.id] || { title: p.name, desc: `Habilita la integración con ${p.name}.` };

                      return (
                        <div class="wizard-channel-card">
                          <div class="channel-info">
                            <h4 class="channel-title">{info.title}</h4>
                            <p class="channel-description">{info.desc}</p>
                          </div>
                          
                          <div class="channel-action">
                            <label class="toggle-switch">
                              <input
                                type="checkbox"
                                checked={enabled()}
                                onChange={(e) =>
                                  handleFieldChange(enabledKey, e.currentTarget.checked)
                                }
                              />
                              <span class="toggle-slider" />
                            </label>
                          </div>
                          
                          <Show when={enabled() && schema()}>
                            <div class="wizard-channel-settings-container">
                              <div class="wizard-channel-settings">
                                <For each={Object.entries(schema().properties || {})}>
                                  {([k, v]) => (
                                    <WizardSchemaField
                                      field={`plugins.settings.${p.id}.${k}`}
                                      schema={v as unknown as SchemaProperty}
                                      values={(getValueAtPath(formValues(), `plugins.settings.${p.id}`) as Record<string, unknown>) || {}}
                                      onChange={handleFieldChange}
                                    />
                                  )}
                                </For>
                              </div>
                            </div>
                          </Show>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            {/* Step 7: Finalize */}
            <Show when={step() === 7}>
              <div class="wizard-step wizard-step--done">
                <span class="material-symbols-outlined wizard-done-icon">check_circle</span>
                <h2>{t("wizard.done.title")}</h2>
                <p>{t("wizard.done.description")}</p>
                <Show when={saveError()}>
                  <p class="wizard-error">{saveError()}</p>
                </Show>
              </div>
            </Show>
          </Show>
        </div>

        <div class="wizard-actions">
          <Show when={(step() > 0 && step() < 7) || (step() === 4 && marketplaceSelected())}>
            <button class="wizard-btn wizard-btn-secondary" onClick={goBack}>
              {step() === 4 && marketplaceSelected() ? t("marketplace.back") : t("wizard.back")}
            </button>
          </Show>
          <div class="wizard-actions-right">
            <Show when={step() === 4 && !marketplaceSelected()}>
              <button class="wizard-btn wizard-btn-secondary" onClick={goNext}>
                {t("wizard.skip")}
              </button>
            </Show>
            <Show when={step() < 7 && !(step() === 4 && marketplaceSelected())}>
              <button
                class="wizard-btn wizard-btn-primary"
                onClick={goNext}
                disabled={!canGoNext()}
              >
                {t("wizard.next")}
              </button>
            </Show>
            <Show when={step() === 7}>
              <button
                class="wizard-btn wizard-btn-primary"
                onClick={handleSaveAndFinish}
                disabled={isSaving()}
              >
                {isSaving() ? t("settings.saving") : t("wizard.finish")}
              </button>
            </Show>
          </div>
        </div>
      </div>
    </div>
  );
};

export default WizardView;
