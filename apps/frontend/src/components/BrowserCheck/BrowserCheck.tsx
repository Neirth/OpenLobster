// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from "solid-js";
import { t } from "../../App";
import "./BrowserCheck.css";
import { checkBrowserFeatures } from "./checkBrowserFeatures";

/**
 * BrowserCheck component verifies browser compatibility.
 * Checks for required features: ES6 Proxy, Async/Await, Fetch API, CSS Grid, CSS Custom Properties.
 * Displays a fullscreen message if the browser doesn't meet minimum requirements.
 */
/* checkBrowserFeatures moved to ./checkBrowserFeatures.ts */

const BrowserCheck: Component<{ children: JSX.Element }> = (props) => {

  // Evaluate features at render time to respect test setup that mutates
  // `globalThis.CSS.supports` before rendering the component.
  const compatible = checkBrowserFeatures();

  return (
    <>
      {!compatible ? (
        <div class="browser-check">
          <div class="browser-check__content">
            <h1 class="browser-check__title">{t("browser.outdated.title")}</h1>
            <p class="browser-check__message">{t("browser.outdated.message")}</p>
            <p class="browser-check__features">{t("browser.outdated.features")}</p>
          </div>
        </div>
      ) : (
        props.children
      )}
    </>
  );
};

export default BrowserCheck;
