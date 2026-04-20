// DOM types are available globally via TypeScript lib; no imports needed here.
// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

/**
 * authStore uses module-level createSignal, which means the signal is
 * shared across tests in the same process.  We reset localStorage and
 * re-import the module in a fresh module scope for each group of tests
 * via vi.resetModules().
 */

const TOKEN_KEY = 'openlobster_access_token';

describe('authStore', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.resetModules();
  });

  afterEach(() => {
    localStorage.clear();
  });

  // ------------------------------------------------------------------ //
  // getStoredToken                                                       //
  // ------------------------------------------------------------------ //

  it('getStoredToken returns null when localStorage is empty', async () => {
    const { getStoredToken } = await import('./authStore');
    expect(getStoredToken()).toBeNull();
  });

  it('getStoredToken returns stored token', async () => {
    localStorage.setItem(TOKEN_KEY, 'abc123');
    const { getStoredToken } = await import('./authStore');
    expect(getStoredToken()).toBe('abc123');
  });

  it('getStoredToken returns null when localStorage throws', async () => {
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('unavailable');
    });
    const { getStoredToken } = await import('./authStore');
    expect(getStoredToken()).toBeNull();
    spy.mockRestore();
  });

  // ------------------------------------------------------------------ //
  // saveToken                                                            //
  // ------------------------------------------------------------------ //

  it('saveToken persists token to localStorage', async () => {
    const { saveToken } = await import('./authStore');
    saveToken('my-token');
    expect(localStorage.getItem(TOKEN_KEY)).toBe('my-token');
  });

  it('saveToken sets needsAuth to false', async () => {
    const { saveToken, needsAuth, setNeedsAuth } = await import('./authStore');
    setNeedsAuth(true);
    expect(needsAuth()).toBe(true);
    saveToken('tok');
    expect(needsAuth()).toBe(false);
  });

  it('saveToken does not throw when localStorage throws', async () => {
    const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });
    const { saveToken } = await import('./authStore');
    expect(() => saveToken('tok')).not.toThrow();
    spy.mockRestore();
  });

  // ------------------------------------------------------------------ //
  // clearToken                                                           //
  // ------------------------------------------------------------------ //

  it('clearToken removes token from localStorage', async () => {
    localStorage.setItem(TOKEN_KEY, 'existing');
    const { clearToken } = await import('./authStore');
    clearToken();
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull();
  });

  it('clearToken sets needsAuth to true', async () => {
    const { clearToken, needsAuth } = await import('./authStore');
    clearToken();
    expect(needsAuth()).toBe(true);
  });

  it('clearToken does not throw when localStorage throws', async () => {
    const spy = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('unavailable');
    });
    const { clearToken } = await import('./authStore');
    expect(() => clearToken()).not.toThrow();
    spy.mockRestore();
  });

  // ------------------------------------------------------------------ //
  // needsAuth signal initial state                                       //
  // ------------------------------------------------------------------ //

  it('needsAuth starts as false', async () => {
    const { needsAuth } = await import('./authStore');
    expect(needsAuth()).toBe(false);
  });

  it('syncNeedsAuthFromStorage clears needsAuth when token exists', async () => {
    localStorage.setItem(TOKEN_KEY, 'abc123');
    const { needsAuth, setNeedsAuth, syncNeedsAuthFromStorage } = await import('./authStore');
    setNeedsAuth(true);
    syncNeedsAuthFromStorage();
    expect(needsAuth()).toBe(false);
  });

  it('syncNeedsAuthFromStorage keeps needsAuth when token is missing', async () => {
    const { needsAuth, setNeedsAuth, syncNeedsAuthFromStorage } = await import('./authStore');
    setNeedsAuth(true);
    syncNeedsAuthFromStorage();
    expect(needsAuth()).toBe(true);
  });

  // ------------------------------------------------------------------ //
  // setNeedsAuth                                                         //
  // ------------------------------------------------------------------ //

  it('setNeedsAuth can set needsAuth to true', async () => {
    const { needsAuth, setNeedsAuth } = await import('./authStore');
    setNeedsAuth(true);
    expect(needsAuth()).toBe(true);
  });

  it('setNeedsAuth can toggle needsAuth back to false', async () => {
    const { needsAuth, setNeedsAuth } = await import('./authStore');
    setNeedsAuth(true);
    setNeedsAuth(false);
    expect(needsAuth()).toBe(false);
  });

  // ------------------------------------------------------------------ //
  // Round-trip: save then clear                                         //
  // ------------------------------------------------------------------ //

  it('round-trip: saveToken then clearToken leaves no token', async () => {
    const { saveToken, clearToken, getStoredToken } = await import('./authStore');
    saveToken('round-trip-token');
    expect(getStoredToken()).toBe('round-trip-token');
    clearToken();
    expect(getStoredToken()).toBeNull();
  });

  // ------------------------------------------------------------------ //
  // validateTokenOnServer                                                 //
  // ------------------------------------------------------------------ //

  it('validateTokenOnServer returns true when response is 200', async () => {
    const { validateTokenOnServer } = await import('./authStore');
    global.fetch = vi.fn().mockResolvedValue({ status: 200 });
    const result = await validateTokenOnServer('valid-tok');
    expect(result).toBe(true);
  });

  it('validateTokenOnServer returns false when response is 401', async () => {
    const { validateTokenOnServer } = await import('./authStore');
    global.fetch = vi.fn().mockResolvedValue({ status: 401 });
    const result = await validateTokenOnServer('invalid-tok');
    expect(result).toBe(false);
  });

  it('validateTokenOnServer returns true on network error (optimistic save)', async () => {
    const { validateTokenOnServer } = await import('./authStore');
    global.fetch = vi.fn().mockRejectedValue(new Error('Network failure'));
    const result = await validateTokenOnServer('any-tok');
    expect(result).toBe(true);
  });
});

