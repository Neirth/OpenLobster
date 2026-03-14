// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, createResource, createEffect, For, Show } from "solid-js";
import { t } from "../../App";
import Modal from "../../components/Modal";
import { client } from "../../graphql/client";
import "./PairingModal.css";

const USERS_QUERY = `
  query GetUsers {
    users {
      id
      primaryID
    }
  }
`;

interface UserRecord {
  id: string;
  primaryID: string;
}

export interface PairingModalProps {
  isOpen: boolean;
  onClose: () => void;
  onApprove: (requestID: string, userID: string, displayName: string) => void;
  onDeny: (requestID: string, reason?: string) => void;
  request: {
    requestID: string;
    code: string;
    channelID: string;
    channelType: string;
    displayName: string;
    timestamp: string;
  } | null;
}

const CHANNEL_ICONS: Record<string, string> = {
  telegram: "send",
  discord: "forum",
  whatsapp: "chat",
  twilio: "phone",
};

const PairingModal: Component<PairingModalProps> = (props) => {
  const [mode, setMode] = createSignal<"existing" | "new">("existing");
  const [selectedUserID, setSelectedUserID] = createSignal<string>("");
  const [displayName, setDisplayName] = createSignal<string>("");

  // Pre-fill the display name whenever a new request arrives.
  createEffect(() => {
    if (props.request) {
      setDisplayName(props.request.displayName || props.request.channelID);
    }
  });

  const [users] = createResource<UserRecord[], boolean>(
    () => props.isOpen,
    async (open: boolean) => {
      if (!open) return [];
      try {
        const data = await client.request<{ users: UserRecord[] }>(USERS_QUERY);
        return data.users ?? [];
      } catch {
        return [];
      }
    },
  );

  const reset = () => {
    setMode("existing");
    setSelectedUserID("");
    setDisplayName("");
  };

  const handleApprove = () => {
    if (!props.request) return;
    const userID = mode() === "existing" ? selectedUserID() : "";
    props.onApprove(props.request.requestID, userID, displayName());
    reset();
    props.onClose();
  };

  const handleDeny = () => {
    if (!props.request) return;
    props.onDeny(props.request.requestID);
    reset();
    props.onClose();
  };

  const canApprove = () =>
    mode() === "new" || selectedUserID() !== "";

  const channelIcon = () =>
    CHANNEL_ICONS[props.request?.channelType ?? ""] ?? "devices";

  return (
    <Modal
      isOpen={props.isOpen}
      onClose={props.onClose}
      title={t("pairing.title")}
    >
      <Show when={props.request}>
        {(req) => (
          <div class="pm">
            {/* Info rows */}
            <div class="pm-info">
              <div class="pm-row">
                <span class="pm-label">{t("pairing.channel")}</span>
                <span class="pm-value pm-channel">
                  <span class="material-symbols-outlined pm-channel-icon">
                    {channelIcon()}
                  </span>
                  {req().channelType}
                </span>
              </div>
              <div class="pm-row">
                <span class="pm-label">{t("pairing.user")}</span>
                <span class="pm-value pm-username">
                  {req().displayName || req().channelID}
                </span>
              </div>
              <div class="pm-row">
                <span class="pm-label">{t("pairing.code")}</span>
                <span class="pm-value pm-code">{req().code}</span>
              </div>
            </div>

            {/* Mode tabs */}
            <div class="pm-section">
              <p class="pm-section-title">{t("pairing.linkToUser")}</p>
              <div class="pm-tabs">
                <button
                  class={`pm-tab ${mode() === "existing" ? "pm-tab--active" : ""}`}
                  onClick={() => { setMode("existing"); setSelectedUserID(""); }}
                >
                  <span class="material-symbols-outlined">person</span>
                  {t("pairing.existingUser")}
                </button>
                <button
                  class={`pm-tab ${mode() === "new" ? "pm-tab--active" : ""}`}
                  onClick={() => setMode("new")}
                >
                  <span class="material-symbols-outlined">person_add</span>
                  {t("pairing.newUser")}
                </button>
              </div>

              <Show when={mode() === "existing"}>
                <select
                  class="pm-select"
                  value={selectedUserID()}
                  onChange={(e) => setSelectedUserID(e.currentTarget.value)}
                >
                  <option value="">{t("pairing.selectUser")}</option>
                  <For each={users()}>
                    {(user) => (
                      <option value={user.id}>
                        {user.primaryID || user.id}
                      </option>
                    )}
                  </For>
                </select>
              </Show>

              <Show when={mode() === "new"}>
                <div class="pm-field">
                  <label class="pm-field-label" for="pm-display-name">
                    {t("pairing.displayName")}
                  </label>
                  <input
                    id="pm-display-name"
                    class="pm-input"
                    type="text"
                    placeholder={req().displayName || req().channelID}
                    value={displayName()}
                    onInput={(e) => setDisplayName(e.currentTarget.value)}
                  />
                  <p class="pm-field-hint">
                    {t("pairing.displayNameHint")}
                  </p>
                </div>
                <p class="pm-hint">
                  {t("pairing.newUserHint")}
                </p>
              </Show>
            </div>

            {/* Actions */}
            <div class="pm-actions">
              <button class="pm-btn pm-btn--deny" onClick={handleDeny}>
                {t("pairing.deny")}
              </button>
              <button
                class="pm-btn pm-btn--approve"
                onClick={handleApprove}
                disabled={!canApprove()}
              >
                {t("pairing.approve")}
              </button>
            </div>
          </div>
        )}
      </Show>
    </Modal>
  );
};

export default PairingModal;
