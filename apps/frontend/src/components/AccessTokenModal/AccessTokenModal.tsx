// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, Show } from "solid-js";
import { t, recheckConfig } from "../../App";
import { saveToken, validateTokenOnServer } from "../../stores/authStore";
import "./AccessTokenModal.css";

/**
 * Full-screen access-token gate.
 *
 * Shown when the backend returns 401. The backdrop is solid black (not
 * translucent) so the rest of the UI is completely hidden. The user must
 * enter the correct token before they can continue. The token is persisted
 * in localStorage and remains until cleared.
 */
const AccessTokenModal: Component = () => {
  const [token, setToken] = createSignal("");
  const [errorStatus, setErrorStatus] = createSignal<"none" | "empty" | "invalid">("none");
  const [isLoading, setIsLoading] = createSignal(false);

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    const value = token().trim();
    
    if (!value) {
      setErrorStatus("empty");
      return;
    }

    setIsLoading(true);
    try {
      const isValid = await validateTokenOnServer(value);

      if (isValid) {
        saveToken(value);
        setToken("");
        setErrorStatus("none");
        void recheckConfig();
      } else {
        setErrorStatus("invalid");
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleInput = (e: Event) => {
    setToken((e.target as HTMLInputElement).value);
    if (errorStatus() !== "none") setErrorStatus("none");
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
              class={`access-token-input${errorStatus() !== "none" ? " access-token-input--error" : ""}`}
              placeholder={t("accessToken.placeholder")}
              value={token()}
              onInput={handleInput}
              autocomplete="off"
              autofocus
              disabled={isLoading()}
            />
            <Show when={errorStatus() !== "none"}>
              <span class="access-token-error">
                {errorStatus() === "empty" ? t("accessToken.errorEmpty") : t("accessToken.errorInvalid")}
              </span>
            </Show>
          </div>

          <button 
            type="submit" 
            class={`access-token-submit${isLoading() ? " access-token-submit--loading" : ""}`}
            disabled={isLoading()}
          >
            <Show 
              when={!isLoading()} 
              fallback={<span class="access-token-spinner" />}
            >
              <span class="material-symbols-outlined">arrow_forward</span>
              {t("accessToken.unlock")}
            </Show>
          </button>
        </form>

      </div>
    </div>
  );
};

export default AccessTokenModal;
