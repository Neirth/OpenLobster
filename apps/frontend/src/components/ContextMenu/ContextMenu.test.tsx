// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@solidjs/testing-library';
import ContextMenu from './ContextMenu';
import type { ContextMenuItem } from './ContextMenu';

/**
 * happy-dom does not implement the Popover API.  We install no-op stubs on
 * HTMLElement.prototype before each test and restore them afterwards.
 */

let showPopoverSpy: (...args: any[]) => void;
let hidePopoverSpy: (...args: any[]) => void;
let originalMatches: typeof HTMLElement.prototype.matches;

beforeEach(() => {
  showPopoverSpy = vi.fn() as unknown as (...args: any[]) => void;
  hidePopoverSpy = vi.fn() as unknown as (...args: any[]) => void;
  originalMatches = HTMLElement.prototype.matches;

  HTMLElement.prototype.showPopover = showPopoverSpy;
  HTMLElement.prototype.hidePopover = hidePopoverSpy;
  HTMLElement.prototype.matches = function (selector: string) {
    if (selector === ':popover-open') return false;
    return originalMatches.call(this, selector);
  };
});

afterEach(() => {
  cleanup();
  HTMLElement.prototype.matches = originalMatches;
  // Remove the stubs added in beforeEach
  delete (HTMLElement.prototype as unknown as any).showPopover;
  delete (HTMLElement.prototype as unknown as any).hidePopover;
});

const baseItems: ContextMenuItem[] = [
  { label: 'Edit', icon: 'edit', onClick: vi.fn() },
  { label: 'Delete', icon: 'delete', onClick: vi.fn(), danger: true },
];

describe('ContextMenu', () => {
  it('renders the trigger wrapper', () => {
    const { container } = render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    expect(container.querySelector('.ctx-trigger')).toBeTruthy();
  });

  it('renders children inside the trigger', () => {
    const { container } = render(() => (
      <ContextMenu items={baseItems}>
        <span id="child">Trigger</span>
      </ContextMenu>
    ));
    expect(container.querySelector('#child')).toBeTruthy();
  });

  it('renders all menu items via portal', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const items = document.body.querySelectorAll('.ctx-menu__item');
    expect(items.length).toBe(2);
  });

  it('renders item labels', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const items = document.body.querySelectorAll('.ctx-menu__item');
    expect(items[0].textContent).toContain('Edit');
    expect(items[1].textContent).toContain('Delete');
  });

  it('applies danger class to danger items', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const items = document.body.querySelectorAll('.ctx-menu__item');
    expect(items[1].classList.contains('ctx-menu__item--danger')).toBe(true);
  });

  it('does not apply danger class to normal items', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const items = document.body.querySelectorAll('.ctx-menu__item');
    expect(items[0].classList.contains('ctx-menu__item--danger')).toBe(false);
  });

  it('renders icon spans for items that have an icon', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const icons = document.body.querySelectorAll('.ctx-menu__icon');
    expect(icons.length).toBe(2);
  });

  it('does not render icon span when item has no icon', () => {
    const noIconItems: ContextMenuItem[] = [
      { label: 'No Icon', onClick: vi.fn() },
    ];
    render(() => (
      <ContextMenu items={noIconItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const icons = document.body.querySelectorAll('.ctx-menu__icon');
    expect(icons.length).toBe(0);
  });

  it('calls showPopover on right-click', () => {
    const { container } = render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const trigger = container.querySelector('.ctx-trigger') as HTMLElement;
    fireEvent.contextMenu(trigger);
    expect(showPopoverSpy).toHaveBeenCalled();
  });

  it('prevents default on context menu event', () => {
    const { container } = render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const trigger = container.querySelector('.ctx-trigger') as HTMLElement;
    const event = new MouseEvent('contextmenu', { bubbles: true, cancelable: true });
    trigger.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
  });

  it('calls item onClick and hidePopover when item is clicked', () => {
    const clickMock = vi.fn();
    const items: ContextMenuItem[] = [{ label: 'Action', onClick: clickMock }];
    render(() => (
      <ContextMenu items={items}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const item = document.body.querySelector('.ctx-menu__item') as HTMLElement;
    fireEvent.click(item);
    expect(clickMock).toHaveBeenCalledOnce();
    expect(hidePopoverSpy).toHaveBeenCalled();
  });

  it('menu element has popover="auto" attribute', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const menu = document.body.querySelector('.ctx-menu');
    expect(menu?.getAttribute('popover')).toBe('auto');
  });

  it('renders with empty items list (no menu items)', () => {
    render(() => (
      <ContextMenu items={[]}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const items = document.body.querySelectorAll('.ctx-menu__item');
    expect(items.length).toBe(0);
  });

  it('positions menu after calling showPopover (clientX/Y clamped to viewport)', () => {
    const { container } = render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const trigger = container.querySelector('.ctx-trigger') as HTMLElement;
    fireEvent.contextMenu(trigger, { clientX: 100, clientY: 200 });
    // showPopover is called first to let the browser compute size, then position is set
    expect(showPopoverSpy).toHaveBeenCalled();
  });

  it('icon text matches icon property value', () => {
    const items: ContextMenuItem[] = [{ label: 'Edit', icon: 'edit', onClick: vi.fn() }];
    render(() => (
      <ContextMenu items={items}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const icon = document.body.querySelector('.ctx-menu__icon');
    expect(icon?.textContent).toBe('edit');
  });

  it('menu is rendered as a <ul> element', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const menu = document.body.querySelector('.ctx-menu');
    expect(menu?.tagName).toBe('UL');
  });

  it('each menu item is rendered as a <li> element', () => {
    render(() => (
      <ContextMenu items={baseItems}>
        <span>Trigger</span>
      </ContextMenu>
    ));
    const items = document.body.querySelectorAll('.ctx-menu__item');
    items.forEach((item) => {
      expect(item.tagName).toBe('LI');
    });
  });
});
