// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import {
  createSignal,
  For,
  Show,
  onMount,
  createMemo,
  createResource,
  createEffect,
} from "solid-js";
import { createMutation, useQueryClient } from "@tanstack/solid-query";
import { t } from "../../App";
import { getStoredToken, setNeedsAuth } from "../../stores/authStore";
import { CONFIG_QUERY } from "@openlobster/ui/graphql/queries";
import {
  CONNECT_MCP_MUTATION,
  INITIATE_OAUTH_MUTATION,
} from "@openlobster/ui/graphql/mutations";
import { UPDATE_CONFIG_MUTATION } from "@openlobster/ui/graphql/mutations";
import { GRAPHQL_ENDPOINT } from "../../graphql/client";
import { client } from "../../graphql/client";
import { Input } from "../Input/Input";
import "./FirstBootWizard.css";
import "../SchemaForm/SchemaForm.css";

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

const fetchMarketplace = async (): Promise<MarketplaceServer[]> => {
  const res = await fetch("/marketplace.json");
  if (!res.ok) throw new Error("Failed to load marketplace");
  return res.json() as Promise<MarketplaceServer[]>;
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

const TOTAL_STEPS = 7;

function graphqlHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

function getDefaultFormValues(): Record<string, unknown> {
  return {
    agentName: "OpenLobster",
    provider: "ollama",
    model: "llama3.2:latest",
    apiKey: "",
    baseURL: "",
    ollamaHost: "http://localhost:11434",
    ollamaApiKey: "",
    anthropicApiKey: "",
    dockerModelRunnerEndpoint: "http://host.docker.internal:12434/engines/v1",
    capabilities: {
      browser: false,
      terminal: false,
      subagents: true,
      memory: true,
      mcp: true,
      filesystem: true,
      sessions: true,
    },
    graphqlBaseUrl: typeof window !== "undefined" ? window.location.origin : "",
    channelTelegramEnabled: false,
    channelTelegramToken: "",
    channelDiscordEnabled: false,
    channelDiscordToken: "",
    channelSlackEnabled: false,
    channelSlackBotToken: "",
    channelSlackAppToken: "",
    channelWhatsAppEnabled: false,
    channelWhatsAppPhoneId: "",
    channelWhatsAppApiToken: "",
    channelTwilioEnabled: false,
    channelTwilioAccountSid: "",
    channelTwilioAuthToken: "",
    channelTwilioFromNumber: "",
  };
}

const PROVIDERS = [
  { value: "ollama", labelKey: "wizard.provider.ollama" },
  { value: "openai", labelKey: "wizard.provider.openai" },
  { value: "anthropic", labelKey: "wizard.provider.anthropic" },
  { value: "openrouter", labelKey: "wizard.provider.openrouter" },
  { value: "docker-model-runner", labelKey: "wizard.provider.docker" },
  { value: "openai-compatible", labelKey: "wizard.provider.openaiCompatible" },
] as const;

const CHANNELS = [
  { key: "telegram", labelKey: "settings.field.channelTelegramEnabled", enabledKey: "channelTelegramEnabled", tokenKey: "channelTelegramToken" },
  { key: "discord", labelKey: "settings.field.channelDiscordEnabled", enabledKey: "channelDiscordEnabled", tokenKey: "channelDiscordToken" },
  { key: "slack", labelKey: "settings.field.channelSlackEnabled", enabledKey: "channelSlackEnabled", tokenKey: "channelSlackBotToken", tokenKey2: "channelSlackAppToken" },
  { key: "whatsapp", labelKey: "settings.field.channelWhatsAppEnabled", enabledKey: "channelWhatsAppEnabled", tokenKey: "channelWhatsAppApiToken", tokenKey2: "channelWhatsAppPhoneId" },
  { key: "twilio", labelKey: "settings.field.channelTwilioEnabled", enabledKey: "channelTwilioEnabled", tokenKey: "channelTwilioAccountSid", tokenKey2: "channelTwilioAuthToken", tokenKey3: "channelTwilioFromNumber" },
] as const;

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

export interface FirstBootWizardProps {
  onComplete: () => void;
}

const FirstBootWizard: Component<FirstBootWizardProps> = (props) => {
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
  const [detailName, setDetailName] = createSignal("");
  const [detailUrl, setDetailUrl] = createSignal("");

  createEffect(() => {
    const s = marketplaceSelected();
    if (s) {
      setDetailName(s.name || s.id || "");
      setDetailUrl(s.url || "");
    } else {
      setDetailName("");
      setDetailUrl("");
    }
  });

  const [marketplaceServers] = createResource(
    () => (step() === 5 ? "fetch" : null),
    () => fetchMarketplace(),
  );

  const marketplaceFiltered = createMemo(() => {
    const q = marketplaceSearch().toLowerCase();
    const data = marketplaceServers() ?? [];
    if (!q) return data;
    return data.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.company.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        (s.category ?? "").toLowerCase().includes(q),
    );
  });

  const connectMcp = createMutation(() => ({
    mutationFn: (vars: { name: string; transport: string; url: string }) =>
      client.request<{
        connectMcp: { success?: boolean; error?: string; requiresAuth?: boolean; url?: string };
      }>(CONNECT_MCP_MUTATION, vars),
    onSuccess: (data, vars) => {
      const res = data.connectMcp;
      setMarketplaceError(null);
      if (res?.error) {
        setMarketplaceError(res.error);
        return;
      }
      if (res?.requiresAuth) {
        setMarketplaceSelected(null);
        initiateOAuth.mutate({ name: vars.name, url: vars.url });
      } else if (res?.success !== false) {
        setMarketplaceSelected(null);
      }
      queryClient.invalidateQueries({ queryKey: ["mcpServers"] });
    },
    onError: (err) => {
      setMarketplaceError(err instanceof Error ? err.message : t("settings.saveError"));
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
      setMarketplaceError(null);
      if (res?.error) {
        setMarketplaceError(res.error);
        return;
      }
      if (res?.authUrl) {
        window.open(res.authUrl, "oauth_popup", "width=600,height=700");
        setMarketplaceSelected(null);
      }
      queryClient.invalidateQueries({ queryKey: ["mcpServers"] });
    },
    onError: (err) => {
      setMarketplaceError(err instanceof Error ? err.message : t("settings.saveError"));
    },
  }));

  const handleFieldChange = (field: string, value: unknown) => {
    setFormValues((prev) => {
      const next = { ...prev };
      if (field.includes(".")) {
        const parts = field.split(".");
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
          provider: config.agent?.provider ?? "ollama",
          model: config.agent?.model ?? "llama3.2:latest",
          apiKey: config.agent?.apiKey ?? "",
          baseURL: config.agent?.baseURL ?? "",
          ollamaHost: config.agent?.ollamaHost ?? "http://localhost:11434",
          ollamaApiKey: config.agent?.ollamaApiKey ?? "",
          anthropicApiKey: config.agent?.anthropicApiKey ?? "",
          dockerModelRunnerEndpoint: config.agent?.dockerModelRunnerEndpoint ?? "http://host.docker.internal:12434/engines/v1",
          capabilities: config.capabilities ?? getDefaultFormValues().capabilities,
          graphqlBaseUrl: config.graphql?.baseUrl || (typeof window !== "undefined" ? window.location.origin : ""),
          channelTelegramEnabled: config.channelSecrets?.telegramEnabled ?? false,
          channelTelegramToken: config.channelSecrets?.telegramToken ?? "",
          channelDiscordEnabled: config.channelSecrets?.discordEnabled ?? false,
          channelDiscordToken: config.channelSecrets?.discordToken ?? "",
          channelSlackEnabled: config.channelSecrets?.slackEnabled ?? false,
          channelSlackBotToken: config.channelSecrets?.slackBotToken ?? "",
          channelSlackAppToken: config.channelSecrets?.slackAppToken ?? "",
          channelWhatsAppEnabled: config.channelSecrets?.whatsAppEnabled ?? false,
          channelWhatsAppPhoneId: config.channelSecrets?.whatsAppPhoneId ?? "",
          channelWhatsAppApiToken: config.channelSecrets?.whatsAppApiToken ?? "",
          channelTwilioEnabled: config.channelSecrets?.twilioEnabled ?? false,
          channelTwilioAccountSid: config.channelSecrets?.twilioAccountSid ?? "",
          channelTwilioAuthToken: config.channelSecrets?.twilioAuthToken ?? "",
          channelTwilioFromNumber: config.channelSecrets?.twilioFromNumber ?? "",
        });
      }
    } catch {
      // Keep defaults on error
    } finally {
      setIsLoading(false);
    }
  });

  const handleSaveAndFinish = async () => {
    setSaveError(null);
    setIsSaving(true);
    const v = formValues() as Record<string, unknown>;
    const caps = (v.capabilities ?? {}) as Record<string, boolean>;
    try {
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({
          query: UPDATE_CONFIG_MUTATION,
          variables: {
            input: {
              agentName: v.agentName,
              provider: v.provider,
              model: v.model,
              apiKey: v.apiKey ?? "",
              baseURL: v.baseURL ?? "",
              ollamaHost: v.ollamaHost ?? "",
              ollamaApiKey: v.ollamaApiKey ?? "",
              anthropicApiKey: v.anthropicApiKey ?? "",
                wizardCompleted: true,
              dockerModelRunnerEndpoint: v.dockerModelRunnerEndpoint ?? "",
              capabilities: caps,
              graphqlBaseUrl: v.graphqlBaseUrl ?? "",
              channelTelegramEnabled: v.channelTelegramEnabled ?? false,
              channelTelegramToken: v.channelTelegramToken ?? "",
              channelDiscordEnabled: v.channelDiscordEnabled ?? false,
              channelDiscordToken: v.channelDiscordToken ?? "",
              channelSlackEnabled: v.channelSlackEnabled ?? false,
              channelSlackBotToken: v.channelSlackBotToken ?? "",
              channelSlackAppToken: v.channelSlackAppToken ?? "",
              channelWhatsAppEnabled: v.channelWhatsAppEnabled ?? false,
              channelWhatsAppPhoneId: v.channelWhatsAppPhoneId ?? "",
              channelWhatsAppApiToken: v.channelWhatsAppApiToken ?? "",
              channelTwilioEnabled: v.channelTwilioEnabled ?? false,
              channelTwilioAccountSid: v.channelTwilioAccountSid ?? "",
              channelTwilioAuthToken: v.channelTwilioAuthToken ?? "",
              channelTwilioFromNumber: v.channelTwilioFromNumber ?? "",
            },
          },
        }),
      });
      if (res.status === 401) {
        setNeedsAuth(true);
        return;
      }
      const data = await res.json();
      if (data?.errors?.length) {
        setSaveError(data.errors[0]?.message ?? t("settings.saveError"));
        return;
      }
      void queryClient.invalidateQueries({ queryKey: ["config"] });
      void queryClient.refetchQueries({ queryKey: ["agent"] });
      props.onComplete();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : t("settings.saveError"));
    } finally {
      setIsSaving(false);
    }
  };

  const handleMarketplaceAdd = (server: MarketplaceServer) => {
    setMarketplaceError(null);
    const name = (detailName() || server.name || server.id || "").trim();
    const url = (detailUrl() || server.url || "").trim();
    if (!name) {
      setMarketplaceError(t("marketplace.errorNameRequired"));
      return;
    }
    if (!url) {
      setMarketplaceError(t("marketplace.errorUrlRequired"));
      return;
    }
    if (server.oauth) {
      initiateOAuth.mutate({ name, url });
    } else {
      connectMcp.mutate({ name, transport: "http", url });
    }
  };

  const canGoNext = createMemo(() => {
    const v = formValues();
    if (step() === 1) {
      const name = (v.agentName as string)?.trim();
      return !!name;
    }
    return true;
  });

  const goNext = () => {
    if (step() === 5) setMarketplaceSelected(null);
    if (step() < TOTAL_STEPS - 1) setStep((s) => s + 1);
  };

  const goBack = () => {
    if (step() === 5 && marketplaceSelected()) {
      setMarketplaceSelected(null);
    } else if (step() > 0) {
      setStep((s) => s - 1);
    }
  };

  return (
    <div class="wizard-overlay">
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
              <span class="material-symbols-outlined wizard-loading-icon">hourglass_empty</span>
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

            {/* Step 1: Agent name + baseUrl */}
            <Show when={step() === 1}>
              <div class="wizard-step">
                <h2>{t("wizard.agentConfig.title")}</h2>
                <p>{t("wizard.agentConfig.description")}</p>
                <div class="wizard-form">
                  <Input
                    label={t("settings.field.agentName")}
                    type="text"
                    value={(formValues().agentName as string) ?? ""}
                    onInput={(e) => handleFieldChange("agentName", e.currentTarget.value)}
                    placeholder="OpenLobster"
                  />
                  <Input
                    label={t("settings.field.graphqlBaseUrl")}
                    hint={t("settings.field.graphqlBaseUrlDesc")}
                    type="text"
                    value={(formValues().graphqlBaseUrl as string) ?? ""}
                    onInput={(e) => handleFieldChange("graphqlBaseUrl", e.currentTarget.value)}
                    placeholder="https://openlobster.example.com"
                  />
                </div>
              </div>
            </Show>

            {/* Step 2: AI Provider */}
            <Show when={step() === 2}>
              <div class="wizard-step">
                <h2>{t("wizard.aiProvider.title")}</h2>
                <p>{t("wizard.aiProvider.description")}</p>
                <div class="wizard-form">
                  <div class="wizard-field">
                    <label>{t("settings.field.provider")}</label>
                    <select
                      value={(formValues().provider as string) ?? "ollama"}
                      onChange={(e) => handleFieldChange("provider", e.currentTarget.value)}
                    >
                      <For each={PROVIDERS}>
                        {(p) => <option value={p.value}>{t(p.labelKey)}</option>}
                      </For>
                    </select>
                  </div>
                  <Input
                    label={t("settings.field.model")}
                    type="text"
                    value={(formValues().model as string) ?? ""}
                    onInput={(e) => handleFieldChange("model", e.currentTarget.value)}
                    placeholder="llama3.2:latest"
                  />
                  <Show when={(formValues().provider as string) === "ollama"}>
                    <Input
                      label={t("settings.field.ollamaHost")}
                      type="text"
                      value={(formValues().ollamaHost as string) ?? ""}
                      onInput={(e) => handleFieldChange("ollamaHost", e.currentTarget.value)}
                      placeholder="http://localhost:11434"
                    />
                    <Input
                      label={t("settings.field.ollamaApiKey")}
                      hint={t("settings.field.ollamaApiKeyDesc")}
                      type="password"
                      value={(formValues().ollamaApiKey as string) ?? ""}
                      onInput={(e) => handleFieldChange("ollamaApiKey", e.currentTarget.value)}
                    />
                  </Show>
                  <Show when={["openai", "openrouter", "opencode-zen"].includes((formValues().provider as string) ?? "")}>
                    <Input
                      label={t("settings.field.apiKey")}
                      type="password"
                      value={(formValues().apiKey as string) ?? ""}
                      onInput={(e) => handleFieldChange("apiKey", e.currentTarget.value)}
                      placeholder="sk-..."
                    />
                  </Show>
                  <Show when={(formValues().provider as string) === "anthropic"}>
                    <Input
                      label={t("settings.field.anthropicApiKey")}
                      type="password"
                      value={(formValues().anthropicApiKey as string) ?? ""}
                      onInput={(e) => handleFieldChange("anthropicApiKey", e.currentTarget.value)}
                      placeholder="sk-ant-..."
                    />
                  </Show>
                  <Show when={(formValues().provider as string) === "openai-compatible"}>
                    <Input
                      label={t("settings.field.baseURL")}
                      type="text"
                      value={(formValues().baseURL as string) ?? ""}
                      onInput={(e) => handleFieldChange("baseURL", e.currentTarget.value)}
                      placeholder="http://localhost:8000/v1"
                    />
                  </Show>
                  <Show when={(formValues().provider as string) === "docker-model-runner"}>
                    <Input
                      label={t("settings.field.dockerModelRunnerEndpoint")}
                      type="text"
                      value={(formValues().dockerModelRunnerEndpoint as string) ?? ""}
                      onInput={(e) => handleFieldChange("dockerModelRunnerEndpoint", e.currentTarget.value)}
                    />
                  </Show>
                </div>
              </div>
            </Show>

            {/* Step 3: Channels */}
            <Show when={step() === 3}>
              <div class="wizard-step">
                <h2>{t("wizard.channels.title")}</h2>
                <p>{t("wizard.channels.description")}</p>
                <div class="wizard-form wizard-channels">
                  <For each={CHANNELS}>
                    {(ch) => {
                      const enabled = () =>
                        !!(formValues()[ch.enabledKey] as boolean);
                      return (
                        <div class="wizard-channel-row">
                          <div class="wizard-channel-toggle-row">
                            <label class="toggle-switch">
                              <input
                                type="checkbox"
                                checked={enabled()}
                                onChange={(e) =>
                                  handleFieldChange(ch.enabledKey, e.currentTarget.checked)
                                }
                              />
                              <span class="toggle-slider" />
                            </label>
                            <span class="wizard-channel-label">{t(ch.labelKey)}</span>
                          </div>
                          <Show when={enabled()}>
                            <Input
                              type="password"
                              placeholder={t(`wizard.channel.${ch.key}.placeholder`)}
                              hint={t(`wizard.channel.${ch.key}.hint`)}
                              value={(formValues()[ch.tokenKey] as string) ?? ""}
                              onInput={(e) =>
                                handleFieldChange(ch.tokenKey, e.currentTarget.value)
                              }
                            />
                          </Show>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            {/* Step 4: Capabilities */}
            <Show when={step() === 4}>
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
                          <span>{t(`mcps.cap.${cap.key}`)}</span>
                        </label>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            {/* Step 5: Marketplace MCP (skippable) — contenido inline, sin modal */}
            <Show when={step() === 5}>
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
                        <Show when={marketplaceServers.loading}>
                          <div class="wizard-marketplace-loading">
                            <span class="material-symbols-outlined">rotate_right</span>
                            <p>{t("marketplace.loading")}</p>
                          </div>
                        </Show>
                        <Show when={marketplaceServers.error}>
                          <div class="wizard-marketplace-loading">
                            <span class="material-symbols-outlined">error</span>
                            <p>{t("marketplace.error")}</p>
                          </div>
                        </Show>
                        <Show when={!marketplaceServers.loading && !marketplaceServers.error && marketplaceFiltered().length === 0}>
                          <div class="wizard-marketplace-loading">
                            <span class="material-symbols-outlined">search_off</span>
                            <p>{t("marketplace.noResults")}</p>
                          </div>
                        </Show>
                        <div class="wizard-marketplace-grid">
                          <For each={marketplaceFiltered()}>
                            {(server) => (
                              <button
                                class="wizard-marketplace-card"
                                onClick={() => setMarketplaceSelected(server)}
                              >
                                <div class="wizard-marketplace-card__icon">
                                  <img
                                    src={faviconUrl(server.url, server.homepage)}
                                    alt=""
                                    onError={(e) => {
                                      (e.currentTarget as HTMLImageElement).style.display = "none";
                                      const fb = e.currentTarget.nextElementSibling as HTMLElement | null;
                                      if (fb) fb.style.display = "";
                                    }}
                                  />
                                  <span class="material-symbols-outlined" style={{"display":"none"}}>extension</span>
                                </div>
                                <div class="wizard-marketplace-card__body">
                                  <span class="wizard-marketplace-card__name">{server.name}</span>
                                  <span class="wizard-marketplace-card__company">{server.company}</span>
                                  <p class="wizard-marketplace-card__desc">{server.description}</p>
                                </div>
                                <span class="material-symbols-outlined wizard-marketplace-card__chevron">chevron_right</span>
                              </button>
                            )}
                          </For>
                        </div>
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
                          <img
                            src={faviconUrl(server.url, server.homepage)}
                            alt=""
                            onError={(e) => {
                              (e.currentTarget as HTMLImageElement).style.display = "none";
                              const fb = e.currentTarget.nextElementSibling as HTMLElement | null;
                              if (fb) fb.style.display = "";
                            }}
                          />
                          <span class="material-symbols-outlined" style={{"display":"none"}}>extension</span>
                        </div>
                        <div>
                          <h3 class="wizard-marketplace-detail__name">{server.name}</h3>
                          <p class="wizard-marketplace-detail__company">{server.company}</p>
                        </div>
                      </div>
                      <p class="wizard-marketplace-detail__desc">{server.description}</p>
                      <div class="wizard-form wizard-marketplace-detail__form">
                        <Input
                          label={t("marketplace.name")}
                          type="text"
                          value={detailName()}
                          onInput={(e) => setDetailName(e.currentTarget.value)}
                          placeholder={server.name || server.id}
                        />
                        <Input
                          label={t("marketplace.endpoint")}
                          type="url"
                          value={detailUrl() || server.url}
                          onInput={(e) => setDetailUrl(e.currentTarget.value)}
                          placeholder="https://..."
                        />
                      </div>
                      <Show when={marketplaceError()}>
                        <p class="wizard-error wizard-marketplace-error">{marketplaceError()}</p>
                      </Show>
                      <button
                        class="wizard-btn wizard-btn-primary"
                        onClick={() => handleMarketplaceAdd(server)}
                        disabled={connectMcp.isPending || initiateOAuth.isPending}
                      >
                        <span class="material-symbols-outlined">add_circle</span>
                        {connectMcp.isPending || initiateOAuth.isPending
                          ? t("common.loading")
                          : t("marketplace.connect")}
                      </button>
                    </div>
                  )}
                </Show>
              </div>
            </Show>

            {/* Step 6: All done */}
            <Show when={step() === 6}>
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
          <Show when={(step() > 0 && step() < 6) || (step() === 5 && marketplaceSelected())}>
            <button class="wizard-btn wizard-btn-secondary" onClick={goBack}>
              {step() === 5 && marketplaceSelected() ? t("marketplace.back") : t("wizard.back")}
            </button>
          </Show>
          <div class="wizard-actions-right">
            <Show when={step() === 5 && !marketplaceSelected()}>
              <button class="wizard-btn wizard-btn-secondary" onClick={goNext}>
                {t("wizard.skip")}
              </button>
            </Show>
            <Show when={step() < 6 && !(step() === 5 && marketplaceSelected())}>
              <button
                class="wizard-btn wizard-btn-primary"
                onClick={goNext}
                disabled={!canGoNext()}
              >
                {t("wizard.next")}
              </button>
            </Show>
            <Show when={step() === 6}>
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

export default FirstBootWizard;
