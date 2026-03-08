// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * AppShell wraps every page with the shared Header and the main content area.
 *
 * @param activeTab - Tab identifier forwarded to Header for highlighting
 * @param children  - Page content rendered inside the scrollable main area
 */

import type { Component, JSX } from "solid-js";
import Header, { type TabId } from "../Header/Header";
import { client } from "../../graphql/client";
import "./AppShell.css";

interface AppShellProps {
  activeTab: TabId;
  children: JSX.Element;
  fullWidth?: boolean;
  fullHeight?: boolean;
}

const AppShell: Component<AppShellProps> = (props) => {
  return (
    <div class="app-shell">
      <Header activeTab={props.activeTab} graphqlClient={client} />
      <main class="app-shell__main">
        <div
          class="app-shell__content"
          classList={{
            "app-shell__content--full": props.fullWidth,
            "app-shell__content--full-height": props.fullHeight,
          }}
        >
          {props.children}
        </div>
      </main>
      {/* Footer is view-specific; individual views include it when needed */}
    </div>
  );
};

export default AppShell;
