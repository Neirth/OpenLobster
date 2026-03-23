// DOM types are available globally via TypeScript lib; no imports needed here.
// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

/**
 * authStore uses module-level createSignal, which means the signal is
 * shared across tests in the same process.  We reset sessionStorage and
 * re-import the module in a fresh module scope for each group of tests
 * via vi.resetModules().
 */

const TOKEN_KEY = 'openlobster_access_token';

describe('authStore', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.resetModules();
  });

  afterEach(() => {
    sessionStorage.clear();
  });

  // ------------------------------------------------------------------ //
  // getStoredToken                                                       //
  // ------------------------------------------------------------------ //

  it('getStoredToken returns null when sessionStorage is empty', async () => {
    const { getStoredToken } = await import('./authStore');
    expect(getStoredToken()).toBeNull();
  });

  it('getStoredToken returns stored token', async () => {
    sessionStorage.setItem(TOKEN_KEY, 'abc123');
    const { getStoredToken } = await import('./authStore');
    expect(getStoredToken()).toBe('abc123');
  });

  it('getStoredToken returns null when sessionStorage throws', async () => {
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

  it('saveToken persists token to sessionStorage', async () => {
    const { saveToken } = await import('./authStore');
    saveToken('my-token');
    expect(sessionStorage.getItem(TOKEN_KEY)).toBe('my-token');
  });

  it('saveToken sets needsAuth to false', async () => {
    const { saveToken, needsAuth, setNeedsAuth } = await import('./authStore');
    setNeedsAuth(true);
    expect(needsAuth()).toBe(true);
    saveToken('tok');
    expect(needsAuth()).toBe(false);
  });

  it('saveToken does not throw when sessionStorage throws', async () => {
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

  it('clearToken removes token from sessionStorage', async () => {
    sessionStorage.setItem(TOKEN_KEY, 'existing');
    const { clearToken } = await import('./authStore');
    clearToken();
    expect(sessionStorage.getItem(TOKEN_KEY)).toBeNull();
  });

  it('clearToken sets needsAuth to true', async () => {
    const { clearToken, needsAuth } = await import('./authStore');
    clearToken();
    expect(needsAuth()).toBe(true);
  });

  it('clearToken does not throw when sessionStorage throws', async () => {
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
});
