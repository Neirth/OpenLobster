// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Reactive auth store for the access-token gate.
 *
 * - getStoredToken()  reads the token from localStorage (persisted across tabs/restarts)
 * - saveToken()       persists the token and hides the modal
 * - clearToken()      removes the token and forces the modal back
 * - needsAuth         reactive signal — true when a 401 has been received
 * - setNeedsAuth      lets the GraphQL client trigger the modal
 */

import { createSignal } from 'solid-js';
import { GRAPHQL_ENDPOINT } from '../graphql/config';

const TOKEN_KEY = 'openlobster_access_token';

/**
 * Performs a lightweight authenticated request to the backend to verify the token.
 * Returns true if the token is valid, false if 401 or network error.
 */
export async function validateTokenOnServer(token: string): Promise<boolean> {
  setIsValidating(true);
  try {
    const res = await fetch(GRAPHQL_ENDPOINT, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${token.trim()}`,
      },
      body: JSON.stringify({ query: "{ __typename }" }),
    });

    return res.status !== 401;
  } catch {
    // Red inalcanzable: guardado optimista. Si el token es incorrecto,
    // el backend devolverá 401 en la siguiente petición.
    return true;
  } finally {
    setIsValidating(false);
  }
}

export function getStoredToken(): string | null {

  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function syncNeedsAuthFromStorage(): void {
  if (getStoredToken()) {
    setNeedsAuth(false);
  }
}

export function saveToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch { /* localStorage unavailable (private mode, SSR) */ }
  setNeedsAuth(false);
}

export function clearToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch { /* localStorage unavailable (private mode, SSR) */ }
  setNeedsAuth(true);
}

export const [needsAuth, setNeedsAuth] = createSignal(false);
export const [isValidating, setIsValidating] = createSignal(false);

