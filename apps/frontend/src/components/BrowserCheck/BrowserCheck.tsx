// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, onMount, Show } from "solid-js";
import { t } from "../../App";
import "./BrowserCheck.css";

/**
 * BrowserCheck component verifies browser compatibility.
 * Checks for required features: ES6 Proxy, Async/Await, Fetch API, CSS Grid, CSS Custom Properties.
 * Displays a fullscreen message if the browser doesn't meet minimum requirements.
 */
const BrowserCheck: Component<{ children: any }> = (props) => {
  const [isCompatible, setIsCompatible] = createSignal(true);

  onMount(() => {
    const checkBrowserFeatures = () => {
      try {
        // Check for ES6 Proxy support (required by SolidJS)
        const hasProxy = typeof Proxy !== "undefined";
        
        // Check for async/await (ES2017)
        const hasAsync = (function () {
          try {
            return (async function () {}).constructor.name === "AsyncFunction";
          } catch {
            return false;
          }
        })();

        // Check for Fetch API
        const hasFetch = typeof fetch !== "undefined";

        // Check for CSS Grid support
        const hasGrid = CSS.supports("display", "grid");

        // Check for CSS Custom Properties (CSS Variables)
        const hasCSSVariables = CSS.supports("color", "var(--test)");

        // Check for modern ES6 features (arrow functions, array methods)
        const hasES6 = (function () {
          try {
            return typeof (() => {}) === "function" && [1, 2].map((x) => x).length === 2;
          } catch {
            return false;
          }
        })();

        const compatible =
          hasProxy &&
          hasAsync &&
          hasFetch &&
          hasGrid &&
          hasCSSVariables &&
          hasES6;

        setIsCompatible(compatible);
      } catch (error) {
        // If any check fails catastrophically, browser is incompatible
        setIsCompatible(false);
      }
    };

    checkBrowserFeatures();
  });

  return (
    <Show when={isCompatible()} fallback={
      <div class="browser-check">
        <div class="browser-check__content">
          <h1 class="browser-check__title">{t("browser.outdated.title")}</h1>
          <p class="browser-check__message">{t("browser.outdated.message")}</p>
          <p class="browser-check__features">{t("browser.outdated.features")}</p>
        </div>
      </div>
    }>
      {props.children}
    </Show>
  );
};

export default BrowserCheck;
