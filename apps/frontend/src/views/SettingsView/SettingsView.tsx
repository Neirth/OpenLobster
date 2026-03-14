// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, For, Show, onMount } from "solid-js";
import { useQueryClient } from "@tanstack/solid-query";
import { t } from "../../App";
import { getStoredToken, setNeedsAuth } from "../../stores/authStore";
import { configSchema, configGroups } from "../../schemas/config.schema";
import { SchemaField } from "../../components/SchemaForm/SchemaField";
import {
  CONFIG_QUERY,
  SYSTEM_FILES_QUERY,
} from "@openlobster/ui/graphql/queries";
import { WRITE_SYSTEM_FILE_MUTATION } from "@openlobster/ui/graphql/mutations";
import { GRAPHQL_ENDPOINT } from "../../graphql/client";
import AppShell from "../../components/AppShell";
import "./SettingsView.css";

/**
 * External resource links shown in the Section Links panel.
 */
function graphqlHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

/** Default form values used when loading fails or for initialisation. */
function getDefaultFormValues(): Record<string, any> {
  return {
    agentName: "OpenLobster",
    provider: "ollama",
    model: "llama3.2:latest",
    apiKey: "",
    baseURL: "",
    ollamaHost: "http://localhost:11434",
    ollamaApiKey: "",
    anthropicApiKey: "",
    dockerModelRunnerEndpoint: "http://localhost:12434/engines/v1",
    dockerModelRunnerModel: "ai/smollm2",
    capabilities: {
      browser: false,
      terminal: false,
      subagents: true,
      memory: true,
      mcp: true,
      filesystem: true,
      sessions: true,
    },
    databaseDriver: "sqlite",
    databaseDSN: "./data/openlobster.db",
    databaseMaxOpenConns: 0,
    databaseMaxIdleConns: 0,
    memoryBackend: "file",
    memoryFilePath: "./data/memory",
    memoryNeo4jURI: "",
    memoryNeo4jUser: "",
    memoryNeo4jPassword: "",
    subagentsMaxConcurrent: 5,
    subagentsDefaultTimeout: "300s",
    graphqlEnabled: true,
    graphqlPort: 8080,
    graphqlHost: "127.0.0.1",
    graphqlBaseUrl: "",
    loggingLevel: "info",
    loggingPath: "./logs",
    secretsBackend: "file",
    secretsFilePath: "./data/secrets",
    secretsOpenbaoURL: "",
    secretsOpenbaoToken: "",
    schedulerEnabled: true,
    schedulerMemoryEnabled: true,
    schedulerMemoryInterval: "4h",
    channelTelegramEnabled: false,
    channelTelegramToken: "",
    channelDiscordEnabled: false,
    channelDiscordToken: "",
    channelWhatsAppEnabled: false,
    channelWhatsAppPhoneId: "",
    channelWhatsAppApiToken: "",
    channelTwilioEnabled: false,
    channelTwilioAccountSid: "",
    channelTwilioAuthToken: "",
    channelTwilioFromNumber: "",
  };
}

/** 3x2 table with documentation links for creating bots on each platform. */
const BOT_DOC_LINKS: { labelKey: string; href: string }[] = [
  { labelKey: "settings.docTelegram", href: "https://core.telegram.org/bots" },
  { labelKey: "settings.docDiscord", href: "https://discord.com/developers/docs" },
  { labelKey: "settings.docWhatsApp", href: "https://developers.facebook.com/docs/whatsapp" },
  { labelKey: "settings.docTwilio", href: "https://www.twilio.com/docs" },
  { labelKey: "settings.docSlack", href: "https://api.slack.com/" },
];

/**
 * SettingsView renders the agent configuration page with dynamic schema-based rendering.
 * Configuration fields are shown/hidden based on dependencies defined in the schema.
 * On save, converts JSON form data to YAML before sending to backend.
 */
const SettingsView: Component = () => {
  const queryClient = useQueryClient();
  // Form values state - will be loaded from server
  const [formValues, setFormValues] = createSignal<Record<string, any>>({});
  const [isLoading, setIsLoading] = createSignal(true);
  const [isSaving, setIsSaving] = createSignal(false);
  const [saveMessage, setSaveMessage] = createSignal<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  // Workspace files state
  const WORKSPACE_FILES = ["AGENTS.md", "SOUL.md", "IDENTITY.md"] as const;
  const [activeFile, setActiveFile] = createSignal<string>("AGENTS.md");
  const [fileContents, setFileContents] = createSignal<Record<string, string>>(
    {},
  );
  const [fileSaving, setFileSaving] = createSignal(false);
  const [fileSaveMsg, setFileSaveMsg] = createSignal<{
    type: "success" | "error";
    text: string;
  } | null>(null);
  // Load configuration from server on mount
  onMount(async () => {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 10000);

    try {
      setIsLoading(true);
      const [configRes, filesRes] = await Promise.all([
        fetch(GRAPHQL_ENDPOINT, {
          method: "POST",
          signal: controller.signal,
          headers: graphqlHeaders(),
          body: JSON.stringify({ query: CONFIG_QUERY }),
        }),
        fetch(GRAPHQL_ENDPOINT, {
          method: "POST",
          headers: graphqlHeaders(),
          body: JSON.stringify({ query: SYSTEM_FILES_QUERY }),
        }),
      ]);

      if (configRes.status === 401 || filesRes.status === 401) {
        setNeedsAuth(true);
        return;
      }

      const data = await configRes.json();
      const filesData = await filesRes.json();

      if (data.errors) {
        console.error("Failed to load configuration:", data.errors);
        setSaveMessage({
          type: "error",
          text: t("settings.loadError"),
        });
        // Still load default values so the form remains editable (including graphqlBaseUrl).
        setFormValues(getDefaultFormValues());
        return;
      }

      // Load workspace files into state (graceful fallback if systemFiles fails)
      const sysFiles: { name: string; content: string }[] =
        filesData?.errors ? [] : (filesData?.data?.systemFiles ?? []);
      const contentsMap: Record<string, string> = {};
      for (const f of sysFiles) {
        contentsMap[f.name] = f.content;
      }
      setFileContents(contentsMap);

      const config = data.data?.config;
      if (config) {
        // Transform server config to form values format (nested structure)
        setFormValues({
          agentName: config.agent?.name || "OpenLobster",
          provider: config.agent?.provider || "ollama",
          model: config.agent?.model || "llama3.2:latest",
          apiKey: config.agent?.apiKey || "",
          baseURL: config.agent?.baseURL || "",
          ollamaHost: config.agent?.ollamaHost || "http://localhost:11434",
          ollamaApiKey: config.agent?.ollamaApiKey || "",
          anthropicApiKey: config.agent?.anthropicApiKey || "",
          dockerModelRunnerEndpoint: config.agent?.dockerModelRunnerEndpoint || "http://localhost:12434/engines/v1",
          dockerModelRunnerModel: config.agent?.dockerModelRunnerModel || "ai/smollm2",
          capabilities: config.capabilities || {
            browser: false,
            terminal: false,
            subagents: true,
            memory: true,
            mcp: true,
            filesystem: true,
            sessions: true,
          },
          databaseDriver: config.database?.driver || "sqlite",
          databaseDSN: config.database?.dsn || "./data/openlobster.db",
          databaseMaxOpenConns: config.database?.maxOpenConns || 0,
          databaseMaxIdleConns: config.database?.maxIdleConns || 0,
          memoryBackend: config.memory?.backend || "file",
          memoryFilePath: config.memory?.filePath || "./data/memory",
          memoryNeo4jURI: config.memory?.neo4j?.uri || "",
          memoryNeo4jUser: config.memory?.neo4j?.user || "",
          memoryNeo4jPassword: config.memory?.neo4j?.password || "",
          subagentsMaxConcurrent: config.subagents?.maxConcurrent || 5,
          subagentsDefaultTimeout: config.subagents?.defaultTimeout || "300s",
          graphqlEnabled:
            config.graphql?.enabled !== undefined
              ? config.graphql.enabled
              : true,
          graphqlPort: config.graphql?.port || 8080,
          graphqlHost: config.graphql?.host || "127.0.0.1",
          graphqlBaseUrl: config.graphql?.baseUrl || "",
          loggingLevel: config.logging?.level || "info",
          loggingPath: config.logging?.path || "./logs",
          secretsBackend: config.secrets?.backend || "file",
          secretsFilePath: config.secrets?.file?.path || "./data/secrets",
          secretsOpenbaoURL: config.secrets?.openbao?.url || "",
          secretsOpenbaoToken: config.secrets?.openbao?.token || "",
          schedulerEnabled: config.scheduler?.enabled ?? true,
          schedulerMemoryEnabled: config.scheduler?.memoryEnabled ?? true,
          schedulerMemoryInterval: config.scheduler?.memoryInterval ?? "4h",
          channelTelegramEnabled:
            config.channelSecrets?.telegramEnabled ?? false,
          channelTelegramToken: config.channelSecrets?.telegramToken || "",
          channelDiscordEnabled: config.channelSecrets?.discordEnabled ?? false,
          channelDiscordToken: config.channelSecrets?.discordToken || "",
          channelWhatsAppEnabled:
            config.channelSecrets?.whatsAppEnabled ?? false,
          channelWhatsAppPhoneId: config.channelSecrets?.whatsAppPhoneId || "",
          channelWhatsAppApiToken:
            config.channelSecrets?.whatsAppApiToken || "",
          channelTwilioEnabled: config.channelSecrets?.twilioEnabled ?? false,
          channelTwilioAccountSid:
            config.channelSecrets?.twilioAccountSid || "",
          channelTwilioAuthToken: config.channelSecrets?.twilioAuthToken || "",
          channelTwilioFromNumber:
            config.channelSecrets?.twilioFromNumber || "",
        });
      }
    } catch (error) {
      console.error("Failed to load configuration:", error);
      setSaveMessage({
        type: "error",
        text:
          error instanceof Error
            ? error.message
            : "Failed to load configuration",
      });
      // Load default values so the form remains editable (including graphqlBaseUrl).
      setFormValues(getDefaultFormValues());
    } finally {
      clearTimeout(timeoutId);
      setIsLoading(false);
    }
  });

  // Update a single field value
  const handleFieldChange = (field: string, value: any) => {
    setFormValues((prev) => {
      const newValues = { ...prev };

      // Handle nested fields (e.g., "capabilities.browser")
      if (field.includes(".")) {
        const parts = field.split(".");
        let target: any = newValues;

        for (let i = 0; i < parts.length - 1; i++) {
          const part = parts[i];
          if (!target[part]) {
            target[part] = {};
          }
          target = target[part];
        }

        target[parts[parts.length - 1]] = value;
      } else {
        newValues[field] = value;
      }

      return newValues;
    });
  };

  // Save configuration with direct API call
  const handleSave = async () => {
    try {
      setIsSaving(true);
      setSaveMessage(null);

      const v = formValues();

      // Send to backend GraphQL API with all form fields
      const response = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({
          query: `
            mutation UpdateConfig($input: UpdateConfigInput!) {
              updateConfig(input: $input) {
                agentName
                systemPrompt
                provider
                channels {
                  channelId
                  channelName
                  enabled
                }
              }
            }
          `,
          variables: {
            input: {
              agentName: v.agentName,
              systemPrompt: v.systemPrompt,
              provider: v.provider,
              model: v.model,
              apiKey: v.apiKey,
              baseURL: v.baseURL,
              ollamaHost: v.ollamaHost,
              ollamaApiKey: v.ollamaApiKey,
              anthropicApiKey: v.anthropicApiKey,
              dockerModelRunnerEndpoint: v.dockerModelRunnerEndpoint,
              dockerModelRunnerModel: v.dockerModelRunnerModel,
              capabilities: v.capabilities ?? {},
              databaseDriver: v.databaseDriver,
              databaseDSN: v.databaseDSN,
              databaseMaxOpenConns: v.databaseMaxOpenConns,
              databaseMaxIdleConns: v.databaseMaxIdleConns,
              memoryBackend: v.memoryBackend,
              memoryFilePath: v.memoryFilePath,
              memoryNeo4jURI: v.memoryNeo4jURI,
              memoryNeo4jUser: v.memoryNeo4jUser,
              memoryNeo4jPassword: v.memoryNeo4jPassword,
              subagentsMaxConcurrent: v.subagentsMaxConcurrent,
              subagentsDefaultTimeout: v.subagentsDefaultTimeout,
              graphqlEnabled: v.graphqlEnabled,
              graphqlPort: v.graphqlPort,
              graphqlHost: v.graphqlHost,
              graphqlBaseUrl: v.graphqlBaseUrl,
              loggingLevel: v.loggingLevel,
              loggingPath: v.loggingPath,
              secretsBackend: v.secretsBackend,
              secretsFilePath: v.secretsFilePath,
              secretsOpenbaoURL: v.secretsOpenbaoURL,
              secretsOpenbaoToken: v.secretsOpenbaoToken,
              schedulerEnabled: v.schedulerEnabled,
              schedulerMemoryEnabled: v.schedulerMemoryEnabled,
              schedulerMemoryInterval: v.schedulerMemoryInterval,
              channelTelegramEnabled: v.channelTelegramEnabled,
              channelTelegramToken: v.channelTelegramToken,
              channelDiscordEnabled: v.channelDiscordEnabled,
              channelDiscordToken: v.channelDiscordToken,
              channelWhatsAppEnabled: v.channelWhatsAppEnabled,
              channelWhatsAppPhoneId: v.channelWhatsAppPhoneId,
              channelWhatsAppApiToken: v.channelWhatsAppApiToken,
              channelTwilioEnabled: v.channelTwilioEnabled,
              channelTwilioAccountSid: v.channelTwilioAccountSid,
              channelTwilioAuthToken: v.channelTwilioAuthToken,
              channelTwilioFromNumber: v.channelTwilioFromNumber,
            },
          },
        }),
      });

      if (response.status === 401) {
        setNeedsAuth(true);
        return;
      }

      const data = await response.json();

      if (data.errors) {
        setSaveMessage({
          type: "error",
          text: data.errors[0]?.message || t("settings.saveError"),
        });
      } else if (!data.data?.updateConfig) {
        setSaveMessage({
          type: "error",
          text: t("settings.saveInvalidResponse"),
        });
      } else {
        setSaveMessage({
          type: "success",
          text: t("settings.saveSuccess"),
        });
        setTimeout(() => setSaveMessage(null), 3000);
        // Refrescar agent y config en vivo (p. ej. nombre en el header)
        void queryClient.refetchQueries({ queryKey: ["agent"] });
        void queryClient.refetchQueries({ queryKey: ["config"] });
      }
    } catch (error) {
      console.error("Failed to save configuration:", error);
      setSaveMessage({
        type: "error",
        text:
          error instanceof Error
            ? error.message
            : "Failed to save configuration",
      });
    } finally {
      setIsSaving(false);
    }
  };

  // Save a single workspace file
  const handleFileSave = async () => {
    const name = activeFile();
    const content = fileContents()[name] ?? "";
    try {
      setFileSaving(true);
      setFileSaveMsg(null);
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({
          query: WRITE_SYSTEM_FILE_MUTATION,
          variables: { name, content },
        }),
      });
      if (res.status === 401) {
        setNeedsAuth(true);
        return;
      }
      const data = await res.json();
      if (data?.data?.writeSystemFile?.success) {
        setFileSaveMsg({ type: "success", text: t("settings.fileSaved") });
        setTimeout(() => setFileSaveMsg(null), 2500);
      } else {
        setFileSaveMsg({
          type: "error",
          text: data?.data?.writeSystemFile?.error ?? t("settings.fileError"),
        });
      }
    } catch (e) {
      setFileSaveMsg({
        type: "error",
        text: e instanceof Error ? e.message : t("settings.fileError"),
      });
    } finally {
      setFileSaving(false);
    }
  };

  return (
    <AppShell activeTab="settings">
      <Show when={isLoading()}>
        <div class="settings-loading">
          <span class="material-symbols-outlined settings-loading__icon">settings</span>
          <p class="settings-loading__title">{t("settings.loading")}</p>
          <p class="settings-loading__hint">{t("settings.loadingHint")}</p>
        </div>
      </Show>
      <Show when={!isLoading()}>
      <div class="settings-view">
        <div class="settings-header">
          <h1>{t("settings.title")}</h1>
          <div class="settings-actions">
            {/* loading now shown full-screen above */}
            <Show when={isSaving()}>
              <span class="save-pending">{t("settings.saving")}</span>
            </Show>
            <Show when={saveMessage()?.type === "success"}>
              <span class="save-success">{saveMessage()?.text}</span>
            </Show>
            <Show when={saveMessage()?.type === "error"}>
              <span class="save-error">{saveMessage()?.text}</span>
            </Show>
            <button
              class="save-btn"
              onClick={handleSave}
              disabled={isSaving() || isLoading()}
            >
              {isSaving() ? t("settings.saving") : t("settings.saveChanges")}
            </button>
          </div>
        </div>

        <Show when={!isLoading()}>
          {/* Render each configuration group */}
          <For each={configGroups}>
            {(group) => (
              <section class="settings-section">
                <h2 class="section-title">{t(`settings.group.${group.id}` as "settings.group.general")}</h2>
                <div class="settings-list">
                  <For each={group.fields}>
                    {(fieldKey) => {
                      const schema = configSchema.properties[fieldKey];
                      if (!schema) return null;

                      return (
                        <SchemaField
                          field={fieldKey}
                          schema={schema}
                          values={formValues()}
                          onChange={handleFieldChange}
                        />
                      );
                    }}
                  </For>
                </div>
              </section>
            )}
          </For>
        </Show>

        {/* Workspace Files Editor */}
        <section class="settings-section workspace-editor settings-section--with-gap">
          <div class="workspace-editor__header">
            <h2 class="section-title">{t("settings.workspaceFiles")}</h2>
            <div class="workspace-editor__actions">
              <Show when={fileSaveMsg()?.type === "success"}>
                <span class="save-success">{fileSaveMsg()?.text}</span>
              </Show>
              <Show when={fileSaveMsg()?.type === "error"}>
                <span class="save-error">{fileSaveMsg()?.text}</span>
              </Show>
              <button
                class="save-btn"
                onClick={handleFileSave}
                disabled={fileSaving()}
              >
                {fileSaving() ? t("settings.saving") : t("common.save")}
              </button>
            </div>
          </div>
          <div class="workspace-editor__tabs">
            <For each={WORKSPACE_FILES}>
              {(name) => (
                <button
                  class={`workspace-editor__tab${activeFile() === name ? " active" : ""}`}
                  onClick={() => setActiveFile(name)}
                >
                  {name}
                </button>
              )}
            </For>
          </div>
          <textarea
            class="workspace-editor__textarea"
            value={fileContents()[activeFile()] ?? ""}
            onInput={(e) => {
              const name = activeFile();
              setFileContents((prev) => ({
                ...prev,
                [name]: e.currentTarget.value,
              }));
            }}
            spellcheck={false}
            placeholder={`# ${activeFile()}\n`}
          />
        </section>
      </div>

      {/* Documentation links - 3x2 table */}
      <section class="settings-section settings-section--spaced settings-doc-links">
        <h2 class="section-title">{t("settings.botDocsTitle")}</h2>
        <div class="settings-doc-links__grid">
          <For each={BOT_DOC_LINKS}>
            {(link) => (
              <a
                href={link.href}
                target="_blank"
                rel="noopener noreferrer"
                class="settings-doc-links__cell"
              >
                {t(link.labelKey)}
              </a>
            )}
          </For>
        </div>
      </section>
      </Show>
      <footer class="app-shell__footer">Made with &lt;3 by <a href="https://linkedin.com/in/neirth" target="_blank" rel="noopener noreferrer">@Neirth</a></footer>
    </AppShell>
  );
};

export default SettingsView;
