// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { t } from "../../App";
import "./ErrorView.css";

interface ErrorViewProps {
  code: 400 | 403 | 404 | 500;
  fullscreen?: boolean;
}

/**
 * ErrorView component displays error pages.
 * Can be rendered fullscreen (404, 500) or within AppShell (400, 403).
 */
const ErrorView: Component<ErrorViewProps> = (props) => {
  return (
    <div class={(props.fullscreen ?? false) ? "error-view--fullscreen" : "error-view"}>
      <div class="error-content">
        <h1 class="error-code">{t(`error.${props.code}.title`)}</h1>
        <p class="error-message">{t(`error.${props.code}.message`)}</p>
      </div>
    </div>
  );
};

export const Error400 = () => {
  return <ErrorView code={400} fullscreen={false} />;
};

export const Error403 = () => {
  return <ErrorView code={403} fullscreen={false} />;
};

export const Error404 = () => {
  return <ErrorView code={404} fullscreen={true} />;
};

export const Error500 = () => {
  return <ErrorView code={500} fullscreen={true} />;
};

export default ErrorView;
