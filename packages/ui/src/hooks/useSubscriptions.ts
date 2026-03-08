// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createSignal, onCleanup, createEffect } from "solid-js";
import type { GraphQLClient } from "graphql-request";

export interface PairingRequestEvent {
  requestID: string;
  code: string;
  channelID: string;
  channelType: string;
  displayName: string;
  timestamp: string;
}

export interface SubscriptionEvent {
  type: string;
  timestamp: string;
  data: unknown;
}

type EventHandler<T> = (event: T) => void;

interface UseSubscriptionsOptions {
  url: string;
  onPairingRequest?: EventHandler<PairingRequestEvent>;
  onMessageSent?: EventHandler<any>;
  onEvent?: EventHandler<SubscriptionEvent>;
  onError?: (error: Event) => void;
  onConnected?: () => void;
  onDisconnected?: () => void;
}

interface SubscriptionMessage {
  type: string;
  id?: string;
  payload?: {
    type?: string;
    data?: unknown;
  };
}

export function useSubscriptions(options: UseSubscriptionsOptions) {
  const [isConnected, setIsConnected] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  let ws: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let reconnectAttempts = 0;
  const maxReconnectAttempts = 5;

  const connect = () => {
    if (ws?.readyState === WebSocket.OPEN) return;

    const wsUrl = options.url.replace(/^http/, "ws");

    try {
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        setIsConnected(true);
        setError(null);
        reconnectAttempts = 0;
        options.onConnected?.();

        // Send connection init
        ws?.send(JSON.stringify({ type: "connection_init" }));
      };

      ws.onmessage = (event) => {
        try {
          const msg: SubscriptionMessage = JSON.parse(event.data);
          handleMessage(msg);
        } catch (e) {
          console.error("Failed to parse WebSocket message:", e);
        }
      };

      ws.onerror = (e) => {
        console.error("WebSocket error:", e);
        setError("Connection error");
        options.onError?.(e as Event);
      };

      ws.onclose = () => {
        setIsConnected(false);
        options.onDisconnected?.();

        // Attempt to reconnect
        if (reconnectAttempts < maxReconnectAttempts) {
          reconnectAttempts++;
          const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
          reconnectTimer = setTimeout(connect, delay);
        }
      };
    } catch (e) {
      setError("Failed to create WebSocket connection");
      console.error("WebSocket connection error:", e);
    }
  };

  const handleMessage = (msg: SubscriptionMessage) => {
    switch (msg.type) {
      case "connection_ack":
        // Connected successfully, subscribe to events
        subscribeToEvents();
        break;

      case "next":
        if (
          msg.payload?.type === "pairing_requested" &&
          options.onPairingRequest
        ) {
          const raw = msg.payload.data;
          const data = (typeof raw === "string" ? JSON.parse(raw) : raw) as PairingRequestEvent;
          options.onPairingRequest(data);
        } else if (msg.payload?.type === "message_sent" && options.onMessageSent) {
          const raw = msg.payload.data;
          const data = typeof raw === "string" ? JSON.parse(raw) : raw;
          options.onMessageSent(data);
        }
        
        // Generic event handler for all events
        if (msg.payload?.type && options.onEvent) {
          const raw = msg.payload.data;
          const data = raw ? (typeof raw === "string" ? JSON.parse(raw) : raw) : null;
          options.onEvent({
            type: msg.payload.type,
            timestamp: new Date().toISOString(),
            data,
          } as SubscriptionEvent);
        }
        break;

      case "error":
        console.error("Subscription error:", msg.payload);
        setError("Subscription error");
        break;
    }
  };

  const subscribeToEvents = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;

    // Subscribe to pairing events
    ws.send(
      JSON.stringify({
        type: "start",
        id: "pairing-1",
        query: "pairing_requested",
      }),
    );

    // Subscribe to message sent events for real-time chat
    ws.send(
      JSON.stringify({
        type: "start",
        id: "message-sent-1",
        query: "message_sent",
      }),
    );
  };

  const disconnect = () => {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }

    if (ws) {
      ws.close();
      ws = null;
    }

    setIsConnected(false);
  };

  const sendResponse = (
    eventType: string,
    response: { requestID: string; approved: boolean; reason?: string },
  ) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.error("WebSocket not connected");
      return;
    }

    ws.send(
      JSON.stringify({
        type: "start",
        id: `${eventType}-response`,
        query: `mutation { ${eventType}(requestID: "${response.requestID}", approved: ${response.approved}${response.reason ? `, reason: "${response.reason}"` : ""}) }`,
      }),
    );
  };

  // Auto-connect on mount
  createEffect(() => {
    connect();
  });

  // Cleanup on unmount
  onCleanup(() => {
    disconnect();
  });

  return {
    isConnected,
    error,
    connect,
    disconnect,
    sendResponse,
  };
}

/**
 * Derives the WebSocket subscription URL from the GraphQL endpoint.
 * Backend exposes custom protocol at /ws (not /graphql which uses graphql-ws).
 */
function getSubscriptionUrl(graphqlClient: GraphQLClient): string {
  const base = (graphqlClient as unknown as { url: string }).url;
  return base.replace(/\/graphql\/?$/, "/ws");
}

/**
 * Creates a subscription manager that can be used across the app.
 * Should be called once at the app root level.
 */
export function createSubscriptionManager(client: GraphQLClient) {
  const url = getSubscriptionUrl(client);

  return {
    subscribe(options: Omit<UseSubscriptionsOptions, "url">) {
      return useSubscriptions({
        url,
        ...options,
      });
    },
  };
}
