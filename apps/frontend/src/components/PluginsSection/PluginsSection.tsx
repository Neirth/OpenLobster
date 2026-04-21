// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, For, Show, onMount } from "solid-js";
import { t } from "@/App";
import { getStoredToken } from "@/stores/authStore";
import { GRAPHQL_ENDPOINT } from "@/graphql/constants";
import {
  PLUGINS_QUERY,
} from "@/graphql/queries";
import {
  RELOAD_PLUGINS_MUTATION,
  UPDATE_PLUGIN_CONFIG_MUTATION,
  UPDATE_CONFIG_MUTATION,
} from "@/graphql/mutations";
import {
  CONFIG_QUERY,
} from "@/graphql/queries";
import "./PluginsSection.css";

interface Plugin {
  id: string;
  name: string;
  version: string;
  description: string;
  pluginType: string;
  schemaJson: string;
  enabled: boolean;
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
  tool: "build",
};

const PluginsSection: Component = () => {
  const [plugins, setPlugins] = createSignal<Plugin[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [reloading, setReloading] = createSignal(false);
  const [message, setMessage] = createSignal<{ type: "success" | "error"; text: string } | null>(null);
  const [expandedPlugin, setExpandedPlugin] = createSignal<string | null>(null);
  const [configValues, setConfigValues] = createSignal<Record<string, Record<string, string>>>({});
  const [savingPlugin, setSavingPlugin] = createSignal<string | null>(null);
  const [pluginDefaults, setPluginDefaults] = createSignal<{ai?: string, memory?: string, secrets?: string, audio?: string}>({});
  const [settingDefault, setSettingDefault] = createSignal<string | null>(null);

  const showMessage = (type: "success" | "error", text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const loadPlugins = async () => {
    try {
      // Fetch plugins and config defaults in parallel
      const [pluginsRes, configRes] = await Promise.all([
        fetch(GRAPHQL_ENDPOINT, {
          method: "POST",
          headers: graphqlHeaders(),
          body: JSON.stringify({ query: PLUGINS_QUERY }),
        }),
        fetch(GRAPHQL_ENDPOINT, {
          method: "POST",
          headers: graphqlHeaders(),
          body: JSON.stringify({ query: CONFIG_QUERY }),
        })
      ]);

      const pluginsData = await pluginsRes.json();
      const configData = await configRes.json();

      if (pluginsData.errors) {
        showMessage("error", pluginsData.errors[0]?.message ?? "Failed to load plugins");
      } else {
        setPlugins(pluginsData.data?.plugins ?? []);
      }

      if (!configData.errors && configData.data?.config) {
        const defaults = configData.data.config.pluginDefaults;
        setPluginDefaults({
          ai: defaults.ai,
          memory: defaults.memory,
          secrets: defaults.secrets,
          audio: defaults.audio,
        });
      }
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Failed to load requirements");
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
      setPlugins(data.data?.reloadPlugins ?? []);
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

  const getConfigValue = (pluginId: string, key: string): string => {
    return configValues()[pluginId]?.[key] ?? "";
  };

  const setConfigValue = (pluginId: string, key: string, value: string) => {
    setConfigValues(prev => ({
      ...prev,
      [pluginId]: { ...(prev[pluginId] ?? {}), [key]: value },
    }));
  };

  const handleSaveConfig = async (pluginId: string) => {
    setSavingPlugin(pluginId);
    try {
      const cfg = configValues()[pluginId] ?? {};
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
      showMessage("success", t("plugins.configSaved"));
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Save failed");
    } finally {
      setSavingPlugin(null);
    }
  };

  const handleSetDefault = async (pluginId: string, type: string) => {
    setSettingDefault(pluginId);
    const fieldMapping: Record<string, string> = {
      ai: "pluginDefaultAi",
      memory: "pluginDefaultMemory",
      secrets: "pluginDefaultSecrets",
      audio: "pluginDefaultAudio",
    };
    const field = fieldMapping[type];
    if (!field) return; // Not a category that supports defaults

    try {
      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers: graphqlHeaders(),
        body: JSON.stringify({
          query: UPDATE_CONFIG_MUTATION,
          variables: { input: { [field]: pluginId } },
        }),
      });
      const data = await res.json();
      if (data.errors) {
        showMessage("error", data.errors[0]?.message ?? "Failed to set default");
        return;
      }
      
      const newDefaults = data.data.updateConfig;
      setPluginDefaults({
        ai: newDefaults.pluginDefaultAi,
        memory: newDefaults.pluginDefaultMemory,
        secrets: newDefaults.pluginDefaultSecrets,
        audio: newDefaults.pluginDefaultAudio,
      });
      showMessage("success", t("plugins.defaultSaved"));
    } catch (e) {
      showMessage("error", e instanceof Error ? e.message : "Request failed");
    } finally {
      setSettingDefault(null);
    }
  };

  const parseSchema = (schemaJson: string) => {
    try {
      return JSON.parse(schemaJson);
    } catch {
      return null;
    }
  };

  const getSchemaFields = (schemaJson: string): Array<{ key: string; title: string; type: string; description?: string; default?: string; required: boolean }> => {
    const schema = parseSchema(schemaJson);
    if (!schema?.properties) return [];
    const required: string[] = schema.required ?? [];
    return Object.entries(schema.properties as Record<string, { title?: string; type?: string; description?: string; default?: string }>).map(
      ([key, def]) => ({
        key,
        title: def.title ?? key,
        type: def.type ?? "string",
        description: def.description,
        default: def.default,
        required: required.includes(key),
      })
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
                      <Show when={
                        (plugin.pluginType === "ai" && pluginDefaults().ai === plugin.id) ||
                        (plugin.pluginType === "memory" && pluginDefaults().memory === plugin.id) ||
                        (plugin.pluginType === "secrets" && pluginDefaults().secrets === plugin.id) ||
                        (plugin.pluginType === "audio" && pluginDefaults().audio === plugin.id)
                      }>
                        <span class="plugin-card__badge plugin-card__badge--default">
                          {t("plugins.defaultBadge")}
                        </span>
                      </Show>
                      <span
                        class="plugin-card__badge"
                        classList={{ "plugin-card__badge--active": plugin.enabled }}
                      >
                        {plugin.enabled ? t("plugins.active") : t("plugins.inactive")}
                      </span>
                      <span class="material-symbols-outlined plugin-card__chevron">
                        {isExpanded() ? "expand_less" : "expand_more"}
                      </span>
                    </div>
                  </button>

                  <Show when={isExpanded() && fields.length > 0}>
                    <div class="plugin-card__body">
                      <p class="plugin-card__config-title">{t("plugins.configuration")}</p>
                      <For each={fields}>
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
                            <input
                              class="settings-input"
                              type={field.key.includes("token") || field.key.includes("key") || field.key.includes("password") || field.key.includes("secret") ? "password" : "text"}
                              placeholder={field.default ?? ""}
                              value={getConfigValue(plugin.id, field.key)}
                              onInput={(e) => setConfigValue(plugin.id, field.key, e.currentTarget.value)}
                            />
                          </div>
                        )}
                      </For>
                      <div class="plugin-card__footer">
                        <Show when={
                          ["ai", "memory", "secrets", "audio"].includes(plugin.pluginType) && 
                          pluginDefaults()[plugin.pluginType as keyof ReturnType<typeof pluginDefaults>] !== plugin.id
                        }>
                          <button
                            class="save-btn save-btn--secondary"
                            onClick={() => handleSetDefault(plugin.id, plugin.pluginType)}
                            disabled={settingDefault() === plugin.id}
                          >
                            <span class="material-symbols-outlined" aria-hidden={true}>bookmark</span>
                            {settingDefault() === plugin.id ? t("settings.saving") : t("plugins.setDefault")}
                          </button>
                        </Show>
                        <button
                          class="save-btn"
                          onClick={() => handleSaveConfig(plugin.id)}
                          disabled={isSaving()}
                        >
                          <span class="material-symbols-outlined" aria-hidden={true}>save</span>
                          {isSaving() ? t("settings.saving") : t("common.save")}
                        </button>
                      </div>
                    </div>
                  </Show>

                  <Show when={isExpanded() && fields.length === 0}>
                    <div class="plugin-card__body plugin-card__body--no-config">
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
