// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from "solid-js";
import { createSignal, onMount, onCleanup, Show } from "solid-js";
import { t } from "../../App";
import {
  createSubscriptionManager,
  type PairingRequestEvent,
} from "@/ui/hooks";
import { client } from "../../graphql/client";
import { GRAPHQL_ENDPOINT } from "../../graphql/config";
import { useWsConnection } from "../../stores/wsStore";
import { needsAuth, getStoredToken, syncNeedsAuthFromStorage } from "../../stores/authStore";
import { pendingPairingsQueue, setPendingPairingsQueue, setOpenPairingRequestHandler } from "../../stores/pairingStore";
import AccessTokenModal from "../AccessTokenModal/AccessTokenModal";
import PairingModal from "../PairingModal/PairingModal";

const PENDING_PAIRINGS_QUERY = `
  query GetPendingPairings {
    pendingPairings {
      code
      channelID
      channelType
      platformUserName
      status
      createdAt
      expiresAt
    }
  }
`;

export interface AuthModalsProps {
  children: JSX.Element;
}

const AuthModals: Component<AuthModalsProps> = (props) => {
  const [pairingRequest, setPairingRequest] =
    createSignal<PairingRequestEvent | null>(null);

  const subscriptionMgr = createSubscriptionManager(client);
  const wsConnection = useWsConnection();

  let subscription: ReturnType<typeof subscriptionMgr.subscribe> | null = null;

  const fetchPendingPairings = async () => {
    try {
      const headers: Record<string, string> = { "Content-Type": "application/json" };
      const token = getStoredToken();
      if (token) headers["Authorization"] = `Bearer ${token}`;

      const res = await fetch(GRAPHQL_ENDPOINT, {
        method: "POST",
        headers,
        body: JSON.stringify({ query: PENDING_PAIRINGS_QUERY }),
      });
      if (!res.ok) return;
      const data = await res.json();
      const list: Array<{
        code: string;
        channelID: string;
        channelType: string;
        platformUserName: string;
        status: string;
        createdAt: string;
        expiresAt: string;
      }> = data?.data?.pendingPairings ?? [];

      const pendingOnly = list.filter((p) => p.status === "pending");
      const asEvents: PairingRequestEvent[] = pendingOnly.map((p) => ({
        requestID: p.code,
        code: p.code,
        channelID: p.channelID,
        channelType: p.channelType,
        displayName: p.platformUserName,
        timestamp: p.createdAt,
      }));

      if (asEvents.length > 0) {
        setPendingPairingsQueue(asEvents);
      }
    } catch (e) {
      console.error("Failed to fetch pending pairings:", e);
    }
  };

  onMount(() => {
    syncNeedsAuthFromStorage();

    // Register the handler so Header can open the modal for a specific request.
    setOpenPairingRequestHandler((req) => setPairingRequest(req));

    // Probe the backend immediately. If it responds with 401 the client
    // wrapper sets needsAuth(true) and the AccessTokenModal appears.
    // If a token was saved in a previous session it will be sent automatically.
    client
      .request(`{ __typename }`)
      .catch(() => {
        // 401 is handled inside the client wrapper; any other error is ignored here.
      });

    subscription = subscriptionMgr.subscribe({
      onPairingRequest: (event) => {
        setPendingPairingsQueue((prev) => {
          const exists = prev.some((p) => p.requestID === event.requestID);
          return exists ? prev : [...prev, event];
        });
        setPairingRequest((cur) => cur ?? event);
      },
      onConnected: () => {
        wsConnection.setConnected(true);
        // Fetch pairings that arrived while disconnected
        void fetchPendingPairings();
      },
      onDisconnected: () => {
        wsConnection.setConnected(false);
      },
      onError: (error) => {
        console.error("Subscription error:", error);
      },
    });
  });

  onCleanup(() => {
    subscription?.disconnect();
  });

  const dismissCurrentPairing = (requestID: string) => {
    setPendingPairingsQueue((prev) => prev.filter((p) => p.requestID !== requestID));
    setPairingRequest(() => {
      const remaining = pendingPairingsQueue().filter((p) => p.requestID !== requestID);
      return remaining.length > 0 ? remaining[0] : null;
    });
  };

  const handlePairingApprove = async (requestID: string, userID: string, displayName: string) => {
    try {
      await client.request(
        `
        mutation ApprovePairing($code: String!, $userID: String, $displayName: String) {
          approvePairing(code: $code, userID: $userID, displayName: $displayName) {
            success
            pairing {
              code
              status
            }
          }
        }
      `,
        { code: requestID, userID: userID || null, displayName },
      );
    } catch (e) {
      console.error("Failed to approve pairing:", e);
    }
    dismissCurrentPairing(requestID);
  };

  const handlePairingDeny = async (requestID: string, reason?: string) => {
    try {
      await client.request(
        `
        mutation DenyPairing($code: String!, $reason: String) {
          denyPairing(code: $code, reason: $reason) {
            success
            code
            reason
          }
        }
      `,
        { code: requestID, reason },
      );
    } catch (e) {
      console.error("Failed to deny pairing:", e);
    }
    dismissCurrentPairing(requestID);
  };

  return (
    <>
      {props.children}

      {/* Full-screen access-token gate — shown when backend returns 401 */}
      <Show when={needsAuth()}>
        <AccessTokenModal />
      </Show>

      <PairingModal
        isOpen={pairingRequest() !== null}
        onClose={() => setPairingRequest(null)}
        onApprove={handlePairingApprove}
        onDeny={handlePairingDeny}
        request={pairingRequest()}
      />

      {/* Connection status indicator (only in dev) */}
      <Show when={import.meta.env.DEV}>
        <div
          style={{
            position: "fixed",
            bottom: "8px",
            right: "8px",
            padding: "4px 8px",
            "border-radius": "4px",
            "font-size": "11px",
            "font-family": "var(--font-mono)",
            background: wsConnection.isConnected()
              ? "var(--color-success)"
              : "var(--color-error)",
            color: "#fff",
            "z-index": 9999,
          }}
        >
          {wsConnection.isConnected() ? t("auth.wsConnected") : t("auth.wsDisconnected")}
        </div>
      </Show>
    </>
  );
};

export default AuthModals;
