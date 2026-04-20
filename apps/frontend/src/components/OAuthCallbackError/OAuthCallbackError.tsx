// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Shown when the OAuth callback redirects back with an error.
 * Appears in the popup window, shows a modal with the error message,
 * notifies the opener via postMessage, and allows the user to close the window.
 */

import type { Component } from "solid-js";
import { onMount } from "solid-js";
import { t } from "../../App";
import "./OAuthCallbackError.css";

interface OAuthCallbackErrorProps {
  message: string;
  onClose: () => void;
}

const OAuthCallbackError: Component<OAuthCallbackErrorProps> = (props) => {
  onMount(() => {
    // Notify the opener so the Manage Server modal can show the error state
    if (window.opener) {
      window.opener.postMessage(
        { type: "oauth_error", error: props.message },
        "*",
      );
    }
  });

  return (
    <div class="oauth-callback-error">
      <div class="oauth-callback-error__modal">
        <span class="material-symbols-outlined oauth-callback-error__icon">
          error
        </span>
        <h1 class="oauth-callback-error__title">
          {t("mcps.oauthCallbackErrorTitle")}
        </h1>
        <p class="oauth-callback-error__message">{props.message}</p>
        <button class="oauth-callback-error__btn" onClick={() => props.onClose()}>
          {t("mcps.oauthCallbackErrorClose")}
        </button>
      </div>
    </div>
  );
};

export default OAuthCallbackError;
