// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from "solid-js";
import { createMemo, createSignal, For, Show, onMount } from "solid-js";
import { t } from "../../App";
import { getStoredToken } from "../../stores/authStore";
import { GRAPHQL_ENDPOINT } from "../../graphql/client";
import {
  PLUGINS_QUERY,
} from "@openlobster/ui/graphql/queries";
import {
  UPDATE_CONFIG_MUTATION,
  RELOAD_PLUGINS_MUTATION,
  SET_PLUGIN_ENABLED_MUTATION,
  UPDATE_PLUGIN_CONFIG_MUTATION,
} from "@openlobster/ui/graphql/mutations";
import "./PluginsSection.css";

interface Plugin {
  id: string;
  name: string;
  version: string;
  description: string;
  pluginType: string;
  schemaJson: string;
  configJson: string;
  enabled: boolean;
  available: boolean;
  lastError?: string | null;
  builtin: boolean;
}

interface SchemaFieldDefinition {
  key: string;
  title: string;
  type: string;
  format?: string;
  description?: string;
  defaultValue?: unknown;
  enumValues?: Array<string | number | boolean>;
  required: boolean;
}

interface PluginDefaults {
  ai: string;
  memory: string;
  secrets: string;
  audio: string;
}

interface PluginsSectionProps {
  defaultAiPluginId?: string;
  defaultMemoryPluginId?: string;
  defaultSecretsPluginId?: string;
  defaultAudioPluginId?: string;
  onDefaultsChange?: (defaults: PluginDefaults) => void;
}

function graphqlHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

const PLUGIN_TYPE_ICON: Record<string, string> = {
  ai: "psychology",
  messaging: "chat",
  memory: "database",
  secrets: "key",
  tool: "build",
  audio: "headphones",
};

const OPENAI_PLUGIN_ID = "openlobster-ai-openai";
const OPENAI_ENDPOINT_FIELD = "endpoint";
const OPENAI_BASE_URL_FIELD = "base_url";
const OPENAI_CUSTOM_ENDPOINT = "custom";

const PLUGIN_DEFAULT_FIELD_BY_TYPE: Record<string, keyof PluginDefaults> = {
  ai: "ai",
  memory: "memory",
  secrets: "secrets",
  audio: "audio",
};

type UpdateConfigInput = {
  pluginDefaultAi?: string;
  pluginDefaultMemory?: string;
  pluginDefaultSecrets?: string;
  pluginDefaultAudio?: string;
};

function pluginDefaultFieldForType(pluginType: string): keyof PluginDefaults | null {
  return PLUGIN_DEFAULT_FIELD_BY_TYPE[pluginType] ?? null;
}

function updateConfigInputForPluginDefault(field: keyof PluginDefaults, pluginID: string): UpdateConfigInput {
  switch (field) {
    case "ai":
      return { pluginDefaultAi: pluginID };
    case "memory":
      return { pluginDefaultMemory: pluginID };
    case "secrets":
      return { pluginDefaultSecrets: pluginID };
    case "audio":
      return { pluginDefaultAudio: pluginID };
    default:
      return {};
  }
}

const PluginsSection: Component<PluginsSectionProps> = (props) => {
  const [plugins, setPlugins] = createSignal<Plugin[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [reloading, setReloading] = createSignal(false);
  const [message, setMessage] = createSignal<{ type: "success" | "error"; text: string } | null>(null);
  const [expandedPlugin, setExpandedPlugin] = createSignal<string | null>(null);
  const [configValues, setConfigValues] = createSignal<Record<string, Record<string, unknown>>>({});
  const [savingPlugin, setSavingPlugin] = createSignal<string | null>(null);
  const [togglingPlugin, setTogglingPlugin] = createSignal<string | null>(null);
  const [defaultingPlugin, setDefaultingPlugin] = createSignal<string | null>(null);

  const pluginDefaults = createMemo<PluginDefaults>(() => ({
    ai: props.defaultAiPluginId ?? "",
    memory: props.defaultMemoryPluginId ?? "",
    secrets: props.defaultSecretsPluginId ?? "",
    audio: props.defaultAudioPluginId ?? "",
  }));

  const showMessage = (type: "success" | "error", text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const parseSchema = (schemaJson: string): {
    properties?: Record<string, {
      title?: string;
      type?: string;
      format?: string;
      description?: string;
      default?: unknown;
      enum?: Array<string | number | boolean>;
    }>;
    required?: string[];
  } | null => {
    try {
      return JSON.parse(schemaJson) as {
        properties?: Record<string, {
          title?: string;
          type?: string;
          format?: string;
          description?: string;
          default?: unknown;
          enum?: Array<string | number | boolean>;
        }>;
        required?: string[];
      };
    } catch {
      return null;
    }
  };

  const getSchemaFields = (schemaJson: string): SchemaFieldDefinition[] => {
    const schema = parseSchema(schemaJson);
    if (!schema?.properties) return [];
    const required: string[] = schema.required ?? [];
    return Object.entries(schema.properties).map(([key, def]) => ({
      key,
      title: def.title ?? key,
      type: def.type ?? "string",
      format: def.format,
      description: def.description,
      defaultValue: def.default,
      enumValues: def.enum,
      required: required.includes(key),
    }));
  };

  const parseConfigJSON = (configJson: string): Record<string, unknown> => {
    try {
      const parsed = JSON.parse(configJson) as unknown;
      if (typeof parsed === "object" && parsed !== null) {
        return parsed as Record<string, unknown>;
      }
      return {};
    } catch {
      return {};
    }
  };

  const withSchemaDefaults = (plugin: Plugin): Record<string, unknown> => {
    const cfg = parseConfigJSON(plugin.configJson);
    const fields = getSchemaFields(plugin.schemaJson);
    const out: Record<string, unknown> = { ...cfg };
    for (const field of fields) {
      if (out[field.key] !== undefined) {
        continue;
      }
      if (field.defaultValue !== undefined) {
        out[field.key] = field.defaultValue;
        continue;
      }
      if ((field.enumValues?.length ?? 0) > 0) {
        out[field.key] = field.enumValues?.[0];
      }
    }
    return out;
  };

  const applyPluginsFromServer = (nextPlugins: Plugin[]) => {
    setPlugins(nextPlugins);
    const nextValues: Record<string, Record<string, unknown>> = {};
    for (const plugin of nextPlugins) {
      nextValues[plugin.id] = withSchemaDefaults(plugin);
    }
    setConfigValues(nextValues);
  };

  const loadPlugins = async () => {
    try {
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({ query: PLUGINS_QUERY }),
      });
      const data = await res.json();
      if (data.errors) {
        showMessage("error", data.errors[0]?.message ?? "Failed to load plugins");
        return;
      }
      applyPluginsFromServer(data.data?.plugins ?? []);
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Failed to load plugins");
    } finally {
      setLoading(false);
    }
  };

  onMount(loadPlugins);

  const handleReload = async () => {
    setReloading(true);
    try {
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({ query: RELOAD_PLUGINS_MUTATION }),
      });
      const data = await res.json();
      if (data.errors) {
        showMessage("error", data.errors[0]?.message ?? "Reload failed");
        return;
      }
      applyPluginsFromServer(data.data?.reloadPlugins ?? []);
      showMessage("success", t("plugins.reloaded"));
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Reload failed");
    } finally {
      setReloading(false);
    }
  };

  const toggleExpand = (id: string) => {
    setExpandedPlugin(prev => (prev === id ? null : id));
  };

  const getConfigValue = (pluginId: string, key: string): unknown => {
    return configValues()[pluginId]?.[key];
  };

  const setConfigValue = (pluginId: string, key: string, value: unknown) => {
    setConfigValues(prev => ({
      ...prev,
      [pluginId]: { ...(prev[pluginId] ?? {}), [key]: value },
    }));
  };

  const asTextValue = (value: unknown): string => {
    if (value === undefined || value === null) {
      return "";
    }
    return String(value);
  };

  const coerceFieldValue = (rawValue: unknown, field: SchemaFieldDefinition): unknown => {
    if (rawValue === undefined || rawValue === null) {
      return rawValue;
    }
    switch (field.type) {
      case "boolean": {
        if (typeof rawValue === "boolean") {
          return rawValue;
        }
        if (typeof rawValue === "string") {
          const lowered = rawValue.trim().toLowerCase();
          if (lowered === "true" || lowered === "1") return true;
          if (lowered === "false" || lowered === "0") return false;
        }
        return Boolean(rawValue);
      }
      case "integer": {
        if (typeof rawValue === "number") {
          return Math.trunc(rawValue);
        }
        const parsed = Number.parseInt(String(rawValue), 10);
        return Number.isNaN(parsed) ? rawValue : parsed;
      }
      case "number": {
        if (typeof rawValue === "number") {
          return rawValue;
        }
        const parsed = Number.parseFloat(String(rawValue));
        return Number.isNaN(parsed) ? rawValue : parsed;
      }
      default:
        return String(rawValue);
    }
  };

  const buildConfigPayload = (pluginId: string): Record<string, unknown> => {
    const plugin = plugins().find((p) => p.id === pluginId);
    if (!plugin) {
      return {};
    }
    const fields = getSchemaFields(plugin.schemaJson);
    const current = configValues()[pluginId] ?? {};
    const payload: Record<string, unknown> = parseConfigJSON(plugin.configJson);

    for (const field of fields) {
      const rawValue = current[field.key];
      if (rawValue === undefined) {
        if (field.defaultValue !== undefined && payload[field.key] === undefined) {
          payload[field.key] = field.defaultValue;
        }
        continue;
      }

      const value = coerceFieldValue(rawValue, field);
      if (typeof value === "string" && value.trim() === "" && !field.required) {
        delete payload[field.key];
        continue;
      }
      payload[field.key] = value;
    }

    if (plugin.id === OPENAI_PLUGIN_ID) {
      const endpoint = String(payload[OPENAI_ENDPOINT_FIELD] ?? "").trim().toLowerCase();
      if (endpoint !== OPENAI_CUSTOM_ENDPOINT) {
        delete payload[OPENAI_BASE_URL_FIELD];
      }
    }

    return payload;
  };

  const shouldRenderField = (plugin: Plugin, field: SchemaFieldDefinition): boolean => {
    if (plugin.id !== OPENAI_PLUGIN_ID) {
      return true;
    }
    if (field.key !== OPENAI_BASE_URL_FIELD) {
      return true;
    }

    const endpoint = asTextValue(getConfigValue(plugin.id, OPENAI_ENDPOINT_FIELD)).trim().toLowerCase();
    return endpoint === OPENAI_CUSTOM_ENDPOINT;
  };

  const persistPluginEnabled = async (pluginId: string, enabled: boolean): Promise<void> => {
    const res = await fetch(GRAPHQL_ENDPOINT, {
      method: "POST",
      headers: graphqlHeaders(),
      body: JSON.stringify({
        query: SET_PLUGIN_ENABLED_MUTATION,
        variables: { pluginId, enabled },
      }),
    });
    const data = await res.json();
    if (data.errors) {
      throw new Error(data.errors[0]?.message ?? "Save failed");
    }
    if (!data.data?.setPluginEnabled) {
      throw new Error("Save failed");
    }
  };

  const persistPluginDefault = async (defaultField: keyof PluginDefaults, pluginId: string): Promise<void> => {
    const res = await fetch(GRAPHQL_ENDPOINT, {
      method: "POST",
      headers: graphqlHeaders(),
      body: JSON.stringify({
        query: UPDATE_CONFIG_MUTATION,
        variables: {
          input: updateConfigInputForPluginDefault(defaultField, pluginId),
        },
      }),
    });
    const data = await res.json();
    if (data.errors) {
      throw new Error(data.errors[0]?.message ?? "Save failed");
    }
    if (!data.data?.updateConfig) {
      throw new Error("Save failed");
    }
  };

  const handleSetEnabled = async (pluginId: string, enabled: boolean) => {
    setTogglingPlugin(pluginId);
    try {
      await persistPluginEnabled(pluginId, enabled);
      setPlugins(prev => prev.map((p) => (p.id === pluginId ? { ...p, enabled } : p)));
      showMessage("success", t("plugins.stateSaved"));
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Save failed");
    } finally {
      setTogglingPlugin(null);
    }
  };

  const handleSetDefault = async (plugin: Plugin) => {
    const defaultField = pluginDefaultFieldForType(plugin.pluginType);
    if (!defaultField) {
      return;
    }
    if (pluginDefaults()[defaultField] === plugin.id) {
      return;
    }

    setDefaultingPlugin(plugin.id);
    try {
      await persistPluginDefault(defaultField, plugin.id);

      const sameTypePlugins = plugins().filter((candidate) => candidate.pluginType === plugin.pluginType);
      await Promise.all(
        sameTypePlugins.map((candidate) => persistPluginEnabled(candidate.id, candidate.id === plugin.id)),
      );

      setPlugins((prev) => prev.map((candidate) => {
        if (candidate.pluginType !== plugin.pluginType) {
          return candidate;
        }
        return {
          ...candidate,
          enabled: candidate.id === plugin.id,
        };
      }));

      if (props.onDefaultsChange) {
        props.onDefaultsChange({
          ...pluginDefaults(),
          [defaultField]: plugin.id,
        });
      }

      showMessage("success", t("plugins.defaultSaved"));
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Save failed");
    } finally {
      setDefaultingPlugin(null);
    }
  };

  const handleSaveConfig = async (pluginId: string) => {
    setSavingPlugin(pluginId);
    try {
      const cfg = buildConfigPayload(pluginId);
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({
          query: UPDATE_PLUGIN_CONFIG_MUTATION,
          variables: { pluginId, configJson: JSON.stringify(cfg) },
        }),
      });
      const data = await res.json();
      if (data.errors) {
        showMessage("error", data.errors[0]?.message ?? "Save failed");
        return;
      }
      setPlugins((prev) => prev.map((p) => {
        if (p.id !== pluginId) {
          return p;
        }
        return { ...p, configJson: JSON.stringify(cfg) };
      }));
      showMessage("success", t("plugins.configSaved"));
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Save failed");
    } finally {
      setSavingPlugin(null);
    }
  };

  const renderFieldInput = (plugin: Plugin, field: SchemaFieldDefinition): JSX.Element => {
    const currentValue = getConfigValue(plugin.id, field.key);

    if (field.type === "boolean") {
      return (
        <label class="toggle-switch" aria-label={field.title}>
          <input
            type="checkbox"
            checked={Boolean(currentValue)}
            onChange={(e) => setConfigValue(plugin.id, field.key, e.currentTarget.checked)}
          />
          <span class="toggle-slider" />
        </label>
      );
    }

    if ((field.enumValues?.length ?? 0) > 0) {
      return (
        <select
          class="settings-input"
          value={asTextValue(currentValue)}
          onChange={(e) => setConfigValue(plugin.id, field.key, e.currentTarget.value)}
        >
          <For each={field.enumValues ?? []}>
            {(optionValue) => (
              <option value={String(optionValue)}>{String(optionValue)}</option>
            )}
          </For>
        </select>
      );
    }

    if (field.type === "integer" || field.type === "number") {
      return (
        <input
          class="settings-input"
          type="number"
          step={field.type === "integer" ? "1" : "any"}
          placeholder={asTextValue(field.defaultValue)}
          value={asTextValue(currentValue)}
          onInput={(e) => setConfigValue(plugin.id, field.key, e.currentTarget.value)}
        />
      );
    }

    return (
      <input
        class="settings-input"
        type={field.format === "password" ? "password" : "text"}
        placeholder={asTextValue(field.defaultValue)}
        value={asTextValue(currentValue)}
        onInput={(e) => setConfigValue(plugin.id, field.key, e.currentTarget.value)}
      />
    );
  };

  return (
    <section class="settings-section plugins-section">
      <div class="plugins-section__header">
        <h2 class="section-title">{t("plugins.title")}</h2>
        <div class="plugins-section__actions">
          <Show when={message()?.type === "success"}>
            <span class="save-success">{message()?.text}</span>
          </Show>
          <Show when={message()?.type === "error"}>
            <span class="save-error">{message()?.text}</span>
          </Show>
          <button
            class="save-btn"
            onClick={handleReload}
            disabled={reloading()}
          >
            <span class="material-symbols-outlined" aria-hidden={true}>refresh</span>
            {reloading() ? t("plugins.reloading") : t("plugins.reload")}
          </button>
        </div>
      </div>

      <p class="plugins-section__description">{t("plugins.description")}</p>

      <Show when={loading()}>
        <div class="plugins-section__loading">
          <span class="material-symbols-outlined">hourglass_empty</span>
          <span>{t("plugins.loading")}</span>
        </div>
      </Show>

      <Show when={!loading() && plugins().length === 0}>
        <div class="plugins-section__empty">
          <span class="material-symbols-outlined plugins-section__empty-icon">extension_off</span>
          <p>{t("plugins.empty")}</p>
          <p class="plugins-section__empty-hint">{t("plugins.emptyHint")}</p>
        </div>
      </Show>

      <Show when={!loading() && plugins().length > 0}>
        <div class="plugins-list">
          <For each={plugins()}>
            {(plugin) => {
              const icon = PLUGIN_TYPE_ICON[plugin.pluginType] ?? "extension";
              const fields = getSchemaFields(plugin.schemaJson);
              const isExpanded = () => expandedPlugin() === plugin.id;
              const isSaving = () => savingPlugin() === plugin.id;
              const isToggling = () => togglingPlugin() === plugin.id;
              const isDefaulting = () => defaultingPlugin() === plugin.id;
              const isActive = () => plugin.enabled && plugin.available;
              const defaultField = pluginDefaultFieldForType(plugin.pluginType);
              const isDefault = () => defaultField !== null && pluginDefaults()[defaultField] === plugin.id;
              const showsDefaultBadge = () => defaultField !== null && isDefault();
              const showsRuntimeBadge = () => defaultField === null;
              const showsEnabledToggle = () => defaultField === null;

              return (
                <div class="plugin-card" classList={{ "plugin-card--expanded": isExpanded() }}>
                  <button
                    class="plugin-card__header"
                    onClick={() => toggleExpand(plugin.id)}
                    aria-expanded={isExpanded()}
                  >
                    <div class="plugin-card__icon-wrap">
                      <span class="material-symbols-outlined plugin-card__icon">{icon}</span>
                    </div>
                    <div class="plugin-card__info">
                      <span class="plugin-card__name">{plugin.name}</span>
                      <span class="plugin-card__meta">
                        <span class="plugin-card__type">{plugin.pluginType}</span>
                        <span class="plugin-card__version">v{plugin.version}</span>
                      </span>
                      <span class="plugin-card__desc">{plugin.description}</span>
                    </div>
                    <div class="plugin-card__status">
                      <Show when={showsDefaultBadge()}>
                        <span class="plugin-card__badge plugin-card__badge--default">
                          {t("plugins.defaultBadge")}
                        </span>
                      </Show>
                      <Show when={!showsDefaultBadge() && showsRuntimeBadge()}>
                        <span
                          class="plugin-card__badge"
                          classList={{ "plugin-card__badge--active": isActive() }}
                        >
                          {isActive() ? t("plugins.active") : t("plugins.inactive")}
                        </span>
                      </Show>
                      <Show when={plugin.builtin}>
                        <span class="plugin-card__badge plugin-card__badge--builtin">{t("plugins.builtin")}</span>
                      </Show>
                      <span class="material-symbols-outlined plugin-card__chevron">
                        {isExpanded() ? "expand_less" : "expand_more"}
                      </span>
                    </div>
                  </button>

                  <Show when={isExpanded() && fields.length > 0}>
                    <div class="plugin-card__body">
                      <Show when={showsEnabledToggle()}>
                        <div class="setting-item plugin-card__enabled-row">
                          <div class="setting-info">
                            <span class="setting-label">{t("plugins.enabledLabel")}</span>
                            <p class="setting-description">{t("plugins.enabledDesc")}</p>
                          </div>
                          <label class="toggle-switch" aria-label={t("plugins.enabledLabel")}>
                            <input
                              type="checkbox"
                              checked={plugin.enabled}
                              disabled={isToggling()}
                              onChange={(e) => handleSetEnabled(plugin.id, e.currentTarget.checked)}
                            />
                            <span class="toggle-slider" />
                          </label>
                          <span class="plugin-card__enabled-value">
                            {plugin.enabled ? t("common.enabled") : t("common.disabled")}
                          </span>
                        </div>
                      </Show>

                      <Show when={plugin.lastError}>
                        <p class="save-error">{plugin.lastError}</p>
                      </Show>
                      <p class="plugin-card__config-title">{t("plugins.configuration")}</p>
                      <For each={fields.filter((field) => shouldRenderField(plugin, field))}>
                        {(field) => (
                          <div class="setting-item">
                            <div class="setting-info">
                              <span class="setting-label">
                                {field.title}
                                <Show when={field.required}>
                                  <span class="plugin-card__required"> *</span>
                                </Show>
                              </span>
                              <Show when={field.description}>
                                <p class="setting-description">{field.description}</p>
                              </Show>
                            </div>
                            {renderFieldInput(plugin, field)}
                          </div>
                        )}
                      </For>
                      <div class="plugin-card__footer">
                        <Show when={defaultField !== null}>
                          <button
                            class="save-btn"
                            classList={{ "plugin-card__default-btn--selected": isDefault() }}
                            onClick={() => handleSetDefault(plugin)}
                            disabled={isSaving() || isToggling() || isDefaulting() || isDefault()}
                          >
                            {isDefaulting() ? t("settings.saving") : t("plugins.setDefault")}
                          </button>
                        </Show>
                        <button
                          class="save-btn"
                          onClick={() => handleSaveConfig(plugin.id)}
                          disabled={isSaving() || isToggling() || isDefaulting()}
                        >
                          {isSaving() ? t("settings.saving") : t("common.save")}
                        </button>
                      </div>
                    </div>
                  </Show>

                  <Show when={isExpanded() && fields.length === 0}>
                    <div class="plugin-card__body plugin-card__body--no-config">
                      <Show when={showsEnabledToggle()}>
                        <div class="setting-item plugin-card__enabled-row">
                          <div class="setting-info">
                            <span class="setting-label">{t("plugins.enabledLabel")}</span>
                            <p class="setting-description">{t("plugins.enabledDesc")}</p>
                          </div>
                          <label class="toggle-switch" aria-label={t("plugins.enabledLabel")}>
                            <input
                              type="checkbox"
                              checked={plugin.enabled}
                              disabled={isToggling()}
                              onChange={(e) => handleSetEnabled(plugin.id, e.currentTarget.checked)}
                            />
                            <span class="toggle-slider" />
                          </label>
                          <span class="plugin-card__enabled-value">
                            {plugin.enabled ? t("common.enabled") : t("common.disabled")}
                          </span>
                        </div>
                      </Show>
                      <Show when={defaultField !== null}>
                        <div class="plugin-card__footer">
                          <button
                            class="save-btn"
                            classList={{ "plugin-card__default-btn--selected": isDefault() }}
                            onClick={() => handleSetDefault(plugin)}
                            disabled={isToggling() || isDefaulting() || isDefault()}
                          >
                            {isDefaulting() ? t("settings.saving") : t("plugins.setDefault")}
                          </button>
                        </div>
                      </Show>
                      <p>{t("plugins.noConfig")}</p>
                    </div>
                  </Show>
                </div>
              );
            }}
          </For>
        </div>
      </Show>
    </section>
  );
};

export default PluginsSection;
