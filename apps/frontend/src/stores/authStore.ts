// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Reactive auth store for the access-token gate.
 *
 * - getStoredToken()  reads the token from sessionStorage (cleared on tab close)
 * - saveToken()       persists the token and hides the modal
 * - clearToken()      removes the token and forces the modal back
 * - needsAuth         reactive signal — true when a 401 has been received
 * - setNeedsAuth      lets the GraphQL client trigger the modal
 */

import { createSignal } from 'solid-js';

const TOKEN_KEY = 'openlobster_access_token';

export function getStoredToken(): string | null {
  try {
    return sessionStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function saveToken(token: string): void {
  try {
    sessionStorage.setItem(TOKEN_KEY, token);
  } catch {}
  setNeedsAuth(false);
}

export function clearToken(): void {
  try {
    sessionStorage.removeItem(TOKEN_KEY);
  } catch {}
  setNeedsAuth(true);
}

export const [needsAuth, setNeedsAuth] = createSignal(false);
