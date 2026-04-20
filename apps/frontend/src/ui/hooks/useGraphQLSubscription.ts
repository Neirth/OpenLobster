// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createSignal, onCleanup, createEffect } from 'solid-js';
import { createClient, Client } from 'graphql-ws';

interface UseGraphQLSubscriptionOptions<T> {
  url: string;
  query: string;
  variables?: Record<string, unknown>;
  onData?: (data: T) => void;
  onError?: (error: Error) => void;
  onConnected?: () => void;
  onDisconnected?: () => void;
}

export function useGraphQLSubscription<T extends object>(options: UseGraphQLSubscriptionOptions<T>) {
  const [data, setData] = createSignal<T | null>(null);
  const [error, setError] = createSignal<Error | null>(null);
  const [isConnected, setIsConnected] = createSignal(false);

  let client: Client | null = null;
  let unsubscribe: (() => void) | null = null;

  const subscribe = () => {
    if (!client) {
      // Convert http/https to ws/wss
      const wsUrl = options.url
        .replace(/^http:/, 'ws:')
        .replace(/^https:/, 'wss:');

      client = createClient({
        url: wsUrl,
        on: {
          connected: () => {
            setIsConnected(true);
            options.onConnected?.();
          },
          error: (err) => {
            const error = new Error(String(err));
            setError(error);
            options.onError?.(error);
          },
          closed: () => {
            setIsConnected(false);
            options.onDisconnected?.();
          },
        },
      });
    }

    unsubscribe = client.subscribe(
      {
        query: options.query,
        variables: options.variables,
      },
      {
        next: (message: unknown) => {
          const m = message as { type?: unknown; payload?: { data?: unknown } } | null;
          if (m?.type === 'next' && m.payload?.data && typeof m.payload.data === 'object') {
            const first = Object.values(m.payload.data as Record<string, unknown>)[0];
            const subscriptionData = first as T;
            // eslint-disable-next-line @typescript-eslint/no-unsafe-function-type
            setData(subscriptionData as Exclude<T, Function>);
            options.onData?.(subscriptionData);
          }
        },
        error: (err: unknown) => {
          const error = new Error(String(err));
          setError(error);
          options.onError?.(error);
        },
        complete: () => {
          setIsConnected(false);
          options.onDisconnected?.();
        },
      },
    );
  };

  const unsubscribeAndCleanup = () => {
    if (unsubscribe) {
      unsubscribe();
      unsubscribe = null;
    }
    if (client) {
      client.dispose();
      client = null;
    }
    setIsConnected(false);
  };

  createEffect(() => {
    subscribe();
    return unsubscribeAndCleanup;
  });

  onCleanup(() => {
    unsubscribeAndCleanup();
  });

  return {
    data,
    error,
    isConnected,
    unsubscribe: unsubscribeAndCleanup,
  };
}
