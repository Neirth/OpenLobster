// DOM types are available globally via TypeScript lib; no imports needed here.
// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

/**
 * themeStore holds module-level signals, so we use vi.resetModules()
 * before each test to get a clean import with fresh signal state.
 */

const THEME_KEY = 'openlobster_theme';

describe('themeStore', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.resetModules();
    // Ensure matchMedia returns a neutral default (no light preference)
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === '(prefers-color-scheme: light)' ? false : false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });

  // ------------------------------------------------------------------ //
  // getStoredTheme                                                       //
  // ------------------------------------------------------------------ //

  it('getStoredTheme returns null when localStorage is empty', async () => {
    const { getStoredTheme } = await import('./themeStore');
    expect(getStoredTheme()).toBeNull();
  });

  it('getStoredTheme returns "dark" when stored', async () => {
    localStorage.setItem(THEME_KEY, 'dark');
    const { getStoredTheme } = await import('./themeStore');
    expect(getStoredTheme()).toBe('dark');
  });

  it('getStoredTheme returns "light" when stored', async () => {
    localStorage.setItem(THEME_KEY, 'light');
    const { getStoredTheme } = await import('./themeStore');
    expect(getStoredTheme()).toBe('light');
  });

  it('getStoredTheme returns null for invalid stored value', async () => {
    localStorage.setItem(THEME_KEY, 'solarized');
    const { getStoredTheme } = await import('./themeStore');
    expect(getStoredTheme()).toBeNull();
  });

  it('getStoredTheme returns null when localStorage throws', async () => {
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage error');
    });
    const { getStoredTheme } = await import('./themeStore');
    expect(getStoredTheme()).toBeNull();
    spy.mockRestore();
  });

  // ------------------------------------------------------------------ //
  // setTheme                                                             //
  // ------------------------------------------------------------------ //

  it('setTheme persists to localStorage', async () => {
    const { setTheme } = await import('./themeStore');
    setTheme('dark');
    expect(localStorage.getItem(THEME_KEY)).toBe('dark');
  });

  it('setTheme sets data-theme on documentElement', async () => {
    const { setTheme } = await import('./themeStore');
    setTheme('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('setTheme updates effectiveTheme signal', async () => {
    const { setTheme, effectiveTheme } = await import('./themeStore');
    setTheme('light');
    expect(effectiveTheme()).toBe('light');
  });

  it('setTheme does not throw when localStorage throws', async () => {
    const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });
    const { setTheme } = await import('./themeStore');
    expect(() => setTheme('dark')).not.toThrow();
    spy.mockRestore();
  });

  // ------------------------------------------------------------------ //
  // effectiveTheme                                                       //
  // ------------------------------------------------------------------ //

  it('effectiveTheme defaults to system theme (dark) when no stored theme', async () => {
    // matchMedia already mocked to return dark
    const { effectiveTheme } = await import('./themeStore');
    expect(effectiveTheme()).toBe('dark');
  });

  it('effectiveTheme uses stored theme over system when available', async () => {
    localStorage.setItem(THEME_KEY, 'light');
    // System is dark (from our mock), but stored is light
    const { effectiveTheme } = await import('./themeStore');
    expect(effectiveTheme()).toBe('light');
  });

  // ------------------------------------------------------------------ //
  // setSystemTheme                                                       //
  // ------------------------------------------------------------------ //

  it('setSystemTheme updates effectiveTheme when no stored theme', async () => {
    const { setSystemTheme, effectiveTheme } = await import('./themeStore');
    setSystemTheme('light');
    expect(effectiveTheme()).toBe('light');
  });

  it('setSystemTheme does not override stored theme', async () => {
    localStorage.setItem(THEME_KEY, 'dark');
    const { setTheme, setSystemTheme, effectiveTheme } = await import('./themeStore');
    setTheme('dark');
    setSystemTheme('light');
    // Stored theme wins
    expect(effectiveTheme()).toBe('dark');
  });

  // ------------------------------------------------------------------ //
  // initTheme                                                            //
  // ------------------------------------------------------------------ //

  it('initTheme sets data-theme from stored preference', async () => {
    localStorage.setItem(THEME_KEY, 'light');
    const { initTheme } = await import('./themeStore');
    initTheme();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('initTheme sets data-theme from system when no stored preference', async () => {
    // matchMedia returns dark by default in our mock
    const { initTheme } = await import('./themeStore');
    initTheme();
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('initTheme uses light system preference when matchMedia says light', async () => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === '(prefers-color-scheme: light)',
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
    vi.resetModules();
    const { initTheme } = await import('./themeStore');
    initTheme();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });
});
