// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from "solid-js";
import { createSignal, onMount, onCleanup, Show } from "solid-js";
import { t } from "../../App";
import {
  createSubscriptionManager,
  type PairingRequestEvent,
} from "@openlobster/ui/hooks";
import { client } from "../../graphql/client";
import { useWsConnection } from "../../stores/wsStore";
import { needsAuth } from "../../stores/authStore";
import AccessTokenModal from "../AccessTokenModal/AccessTokenModal";
import PairingModal from "../PairingModal/PairingModal";

export interface AuthModalsProps {
  children: JSX.Element;
}

const AuthModals: Component<AuthModalsProps> = (props) => {
  const [pairingRequest, setPairingRequest] =
    createSignal<PairingRequestEvent | null>(null);

  const subscriptionMgr = createSubscriptionManager(client);
  const wsConnection = useWsConnection();

  let subscription: ReturnType<typeof subscriptionMgr.subscribe> | null = null;

  onMount(() => {
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
        console.log("Pairing request received:", event);
        setPairingRequest(event);
      },
      onConnected: () => {
        wsConnection.setConnected(true);
        console.log("Subscription connected");
      },
      onDisconnected: () => {
        wsConnection.setConnected(false);
        console.log("Subscription disconnected");
      },
      onError: (error) => {
        console.error("Subscription error:", error);
      },
    });
  });

  onCleanup(() => {
    subscription?.disconnect();
  });

  const handlePairingApprove = async (requestID: string, userID: string, displayName: string) => {
    console.log("Approve pairing:", requestID, "userID:", userID, "displayName:", displayName);
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
    setPairingRequest(null);
  };

  const handlePairingDeny = async (requestID: string, reason?: string) => {
    console.log("Deny pairing:", requestID, reason);
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
    setPairingRequest(null);
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
