// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, Show } from "solid-js";
import { t } from "../../App";
import { saveToken } from "../../stores/authStore";
import "./AccessTokenModal.css";

/**
 * Full-screen access-token gate.
 *
 * Shown when the backend returns 401. The backdrop is solid black (not
 * translucent) so the rest of the UI is completely hidden. The user must
 * enter the correct token before they can continue. The token is persisted
 * in sessionStorage and cleared on tab close.
 */
const AccessTokenModal: Component = () => {
  const [token, setToken] = createSignal("");
  const [error, setError] = createSignal(false);

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    const value = token().trim();
    if (!value) {
      setError(true);
      return;
    }
    saveToken(value);
    setToken("");
    setError(false);
  };

  const handleInput = (e: Event) => {
    setToken((e.target as HTMLInputElement).value);
    if (error()) setError(false);
  };

  return (
    <div class="access-token-overlay">
      <div class="access-token-modal">
        <div class="access-token-icon">
          <span class="material-symbols-outlined">lock</span>
        </div>

        <h1 class="access-token-title">{t("accessToken.title")}</h1>
        <p class="access-token-description">
          {t("accessToken.description1")}
          <code>graphql.auth_token</code>
          {t("accessToken.description2")}
          <code>OPENLOBSTER_GRAPHQL_AUTH_TOKEN</code>
          {t("accessToken.description3")}
        </p>

        <form class="access-token-form" onSubmit={handleSubmit}>
          <div class="access-token-field">
            <input
              type="password"
              class={`access-token-input${error() ? " access-token-input--error" : ""}`}
              placeholder={t("accessToken.placeholder")}
              value={token()}
              onInput={handleInput}
              autocomplete="off"
              autofocus
            />
            <Show when={error()}>
              <span class="access-token-error">{t("accessToken.errorEmpty")}</span>
            </Show>
          </div>

          <button type="submit" class="access-token-submit">
            <span class="material-symbols-outlined">arrow_forward</span>
            {t("accessToken.unlock")}
          </button>
        </form>
      </div>
    </div>
  );
};

export default AccessTokenModal;
