// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, ParentComponent } from "solid-js";
import { createSignal, createEffect, onMount, onCleanup, Show, lazy, Suspense } from "solid-js";
import { translator, resolveTemplate } from "@solid-primitives/i18n";
import { useQueryClient } from "@tanstack/solid-query";
import { Router, Route, useLocation } from "@solidjs/router";
import en from "./locales/en.json";
import es from "./locales/es.json";
import zh from "./locales/zh.json";
const ChatView = lazy(() => import("./views/ChatView/ChatView"));
const DashboardView = lazy(() => import("./views/DashboardView/DashboardView"));
const TasksView = lazy(() => import("./views/TasksView/TasksView"));
const MemoryView = lazy(() => import("./views/MemoryView/MemoryView"));
const McpsView = lazy(() => import("./views/McpsView/McpsView"));
const SkillsView = lazy(() => import("./views/SkillsView/SkillsView"));
const SettingsView = lazy(() => import("./views/SettingsView/SettingsView"));
const Error404 = lazy(() => import("./views/ErrorView/ErrorView").then((m) => ({ default: m.Error404 })));
import AuthModals from "./components/AuthModals";
import BrowserCheck from "./components/BrowserCheck";
import MobileBlocker from "./components/MobileBlocker";
import OAuthCallbackError from "./components/OAuthCallbackError/OAuthCallbackError";
import FirstBootWizard from "./components/FirstBootWizard";
import { GRAPHQL_ENDPOINT } from "./graphql/client";
import { getStoredToken } from "./stores/authStore";
import { effectiveTheme, setSystemTheme } from "./stores/themeStore";
import "@openlobster/ui/styles/tokens.css";
import "@openlobster/ui/styles/reset.css";
import "./styles/global.css";

// Exported so that AccessTokenModal can trigger a re-check after saving a token.
export const [configLoaded, setConfigLoaded] = createSignal(false);
export const [showWizard, setShowWizard] = createSignal(true);

export async function recheckConfig(): Promise<void> {
  try {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    const token = getStoredToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
    const res = await fetch(GRAPHQL_ENDPOINT, {
      method: "POST",
      headers,
      body: JSON.stringify({ query: `query GetConfig { config { wizardCompleted } }` }),
    });
    const data = await res.json();
    const completed = data?.data?.config?.wizardCompleted === true;
    setShowWizard(!completed);
  } catch {
    setShowWizard(true);
  } finally {
    setConfigLoaded(true);
  }
}

type Locale = "en" | "es" | "zh";

const dicts: Record<Locale, Record<string, string>> = {
  en: en as Record<string, string>,
  es: es as Record<string, string>,
  zh: zh as Record<string, string>,
};

function detectBrowserLocale(): Locale {
  if (typeof navigator === "undefined") return "en";
  const lang = navigator.language || navigator.languages?.[0] || "en";
  const shortLang = lang.split("-")[0];
  if (shortLang === "zh") return "zh";
  return dicts[shortLang as Locale] ? (shortLang as Locale) : "en";
}

const [locale, setLocale] = createSignal<Locale>(detectBrowserLocale());
export const t = translator(() => dicts[locale()], resolveTemplate);
export { locale, setLocale };

const App: ParentComponent = (props) => {
  const location = useLocation();
  const queryClient = useQueryClient();

  createEffect(() => {
    const path = location.pathname;
    if (!path) return;
    void queryClient.invalidateQueries();
  });

  return <>{props.children}</>;
};

const Root: Component = () => {
  createEffect(() => {
    document.documentElement.setAttribute("data-theme", effectiveTheme());
  });

  // When OAuth callback fails, backend redirects to /?oauth_callback=error&message=...
  // In that case (popup window), show a modal instead of the normal app
  const [oauthError, _setOauthError] = createSignal<{ message: string } | null>(
    (() => {
      if (typeof window === "undefined") return null;
      const params = new URLSearchParams(window.location.search);
      const status = params.get("oauth_callback");
      const message = params.get("message") ?? "";
      if (window.opener && status === "error" && message) return { message: decodeURIComponent(message) };
      return null;
    })(),
  );

  onMount(() => {
    const m = window.matchMedia("(prefers-color-scheme: light)");
    setSystemTheme(m.matches ? "light" : "dark");
    const handler = () => setSystemTheme(m.matches ? "light" : "dark");
    m.addEventListener("change", handler);
    onCleanup(() => m.removeEventListener("change", handler));
  });

  onMount(async () => {
    if (oauthError()) {
      window.history.replaceState({}, "", window.location.pathname || "/");
    }
    await recheckConfig();
  });

  const handleWizardComplete = () => {
    setShowWizard(false);
  };

  return (
    <Show
      when={!oauthError()}
      fallback={
        <OAuthCallbackError
          message={oauthError()!.message}
          onClose={() => window.close()}
        />
      }
    >
      <BrowserCheck>
        <MobileBlocker>
          <AuthModals>
            <Show
              when={!configLoaded() || !showWizard()}
              fallback={<FirstBootWizard onComplete={handleWizardComplete} />}
            >
              <Router root={App}>
                <Suspense fallback={<div class="app-loading" />}>
                  <Route path="/" component={DashboardView} />
                  <Route path="/chat" component={ChatView} />
                  <Route path="/tasks" component={TasksView} />
                  <Route path="/memory" component={MemoryView} />
                  <Route path="/mcps" component={McpsView} />
                  <Route path="/skills" component={SkillsView} />
                  <Route path="/settings" component={SettingsView} />
                  <Route path="*" component={Error404} />
                </Suspense>
              </Router>
            </Show>
          </AuthModals>
        </MobileBlocker>
      </BrowserCheck>
    </Show>
  );
};

export default Root;
