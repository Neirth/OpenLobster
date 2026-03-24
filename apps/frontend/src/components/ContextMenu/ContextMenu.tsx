// DOM types are available globally via TypeScript lib; no imports needed here.
// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * ContextMenu — right-click context menu wrapper backed by the Popover API.
 *
 * The menu element uses `popover="auto"` so the browser handles:
 *   - Top-layer rendering (no z-index juggling)
 *   - Light-dismiss (click outside closes automatically)
 *   - Escape key closes without extra listeners
 *
 * The popover is portal-rendered into <body> to avoid invalid DOM nesting
 * (e.g. when the trigger wraps a <li> inside a <ul>).
 *
 * Usage:
 *   <ContextMenu items={[{ label: 'Edit', icon: 'edit', onClick: ... }]}>
 *     <li>...</li>
 *   </ContextMenu>
 */

import type { Component, JSXElement } from 'solid-js';
import { For, Show, onMount, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import './ContextMenu.css';

export interface ContextMenuItem {
  label: string;
  icon?: string;
  onClick: () => void;
  danger?: boolean;
}

interface ContextMenuProps {
  items: ContextMenuItem[];
  children: JSXElement;
}

const ContextMenu: Component<ContextMenuProps> = (props) => {
  let menuEl: HTMLUListElement | undefined;
  let pendingRightClick = false;

  function handleMouseDown(e: MouseEvent) {
    if (e.button === 2) pendingRightClick = true;
  }

  function handleMouseUp(e: MouseEvent) {
    if (e.button !== 2 || !pendingRightClick) return;
    pendingRightClick = false;
    if (!menuEl) return;

    // Position at (0,0) first so the browser can compute the rendered size,
    // then clamp to keep it fully inside the viewport.
    menuEl.style.left = '0';
    menuEl.style.top = '0';
    menuEl.showPopover();

    const { offsetWidth: w, offsetHeight: h } = menuEl;
    const x = Math.min(e.clientX, window.innerWidth - w - 4);
    const y = Math.min(e.clientY, window.innerHeight - h - 4);
    menuEl.style.left = `${x}px`;
    menuEl.style.top = `${y}px`;
  }

  function preventNativeMenu(e: MouseEvent) {
    e.preventDefault();
  }

  function select(item: ContextMenuItem) {
    menuEl?.hidePopover();
    item.onClick();
  }

  onMount(() => {
    // Ensure the popover is hidden on first mount regardless of browser defaults.
    if (menuEl && menuEl.matches(':popover-open')) menuEl.hidePopover();
  });

  onCleanup(() => {
    if (menuEl && menuEl.matches(':popover-open')) menuEl.hidePopover();
  });

  return (
    <>
      <div
        class="ctx-trigger"
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
        onContextMenu={preventNativeMenu}
      >
        {props.children}
      </div>

      {/* Portal into <body> prevents invalid nesting (e.g. <ul> inside <ul>) */}
      <Portal mount={document.body}>
        <ul class="ctx-menu" popover="auto" ref={menuEl}>
          <For each={props.items}>
            {(item) => (
              <li
                class="ctx-menu__item"
                classList={{ 'ctx-menu__item--danger': item.danger }}
                onClick={() => select(item)}
              >
                <Show when={item.icon}>
                  <span class="material-symbols-outlined ctx-menu__icon">{item.icon}</span>
                </Show>
                {item.label}
              </li>
            )}
          </For>
        </ul>
      </Portal>
    </>
  );
};

export default ContextMenu;
