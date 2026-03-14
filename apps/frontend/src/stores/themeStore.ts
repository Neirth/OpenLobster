// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Theme store: persisted theme (light/dark) with system fallback.
 *
 * - getStoredTheme()  reads from localStorage; null = use system
 * - effectiveTheme()  reactive signal: stored theme or system preference
 * - setTheme(theme)   saves to localStorage and updates the stored signal
 * - initTheme()       call before first paint to set data-theme on <html>
 */

import { createSignal, createMemo } from "solid-js";

const THEME_KEY = "openlobster_theme";

export type Theme = "light" | "dark";

export function getStoredTheme(): Theme | null {
  if (typeof window === "undefined") return null;
  try {
    const v = localStorage.getItem(THEME_KEY);
    if (v === "light" || v === "dark") return v;
    return null;
  } catch {
    return null;
  }
}

function getSystemTheme(): Theme {
  if (typeof window === "undefined") return "dark";
  return window.matchMedia("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
}

const [storedTheme, setStoredTheme] = createSignal<Theme | null>(
  getStoredTheme(),
);

const [systemThemeSignal, setSystemThemeSignal] = createSignal<Theme>(
  getSystemTheme(),
);

/** Only exported for initializing system preference listener (e.g. in App). */
export function setSystemTheme(theme: Theme): void {
  setSystemThemeSignal(theme);
}

export const effectiveTheme = createMemo(
  () => storedTheme() ?? systemThemeSignal(),
);

export function setTheme(theme: Theme): void {
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    /* ignore */
  }
  setStoredTheme(theme);
}

/**
 * Call once before render to avoid flash. Sets data-theme on documentElement
 * from stored preference or system.
 */
export function initTheme(): void {
  const theme = getStoredTheme() ?? getSystemTheme();
  document.documentElement.setAttribute("data-theme", theme);
}
