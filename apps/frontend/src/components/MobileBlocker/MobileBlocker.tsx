// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from "solid-js";
import { createSignal, onMount, Show } from "solid-js";
import { t } from "../../App";
import "./MobileBlocker.css";

/**
 * MobileBlocker component prevents mobile devices from accessing the application.
 * Displays a fullscreen message instructing users to access from a desktop browser.
 */
const MobileBlocker: Component<{ children: JSX.Element }> = (props) => {
  const [isMobile, setIsMobile] = createSignal(false);

  onMount(() => {
    const checkMobile = () => {
      // Check viewport width (tablets in portrait and phones)
      const isSmallScreen = window.innerWidth < 1024;

      // Check user agent for mobile devices
      const userAgent = navigator.userAgent.toLowerCase();
      const mobileKeywords = [
        "android",
        "webos",
        "iphone",
        "ipad",
        "ipod",
        "blackberry",
        "windows phone",
      ];
      const isMobileUA = mobileKeywords.some((keyword) =>
        userAgent.includes(keyword)
      );

      // Check if touch-only device (no mouse)
      const isTouchOnly =
        "ontouchstart" in window &&
        !window.matchMedia("(pointer: fine)").matches;

      setIsMobile(isSmallScreen || isMobileUA || isTouchOnly);
    };

    checkMobile();
    window.addEventListener("resize", checkMobile);

    return () => window.removeEventListener("resize", checkMobile);
  });

  return (
    <Show when={!isMobile()} fallback={
      <div class="mobile-blocker">
        <div class="mobile-blocker__content">
          <h1 class="mobile-blocker__title">{t("mobile.blocked.title")}</h1>
          <p class="mobile-blocker__message">{t("mobile.blocked.message")}</p>
        </div>
      </div>
    }>
      {props.children}
    </Show>
  );
};

export default MobileBlocker;
