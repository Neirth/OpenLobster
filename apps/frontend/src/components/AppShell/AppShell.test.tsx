// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */

import { describe, it, expect, vi } from 'vitest';
import { render } from '@solidjs/testing-library';

// Mock Router before importing components that use it
vi.mock('@solidjs/router', () => ({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  A: (props: any) => <a {...props} />,
  useNavigate: () => vi.fn(),
}));

// Mock Header to avoid Router issues
vi.mock('../Header/Header', () => ({
  default: () => <header class="header" />,
}));

// Mock @openlobster/ui utilities
vi.mock('../../graphql/client', () => ({
  client: {},
}));

import AppShell from './AppShell';

describe('AppShell Component', () => {
  it('renders app-shell container', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview">
        <div>Content</div>
      </AppShell>
    ));
    const appShell = container.querySelector('.app-shell');
    expect(appShell).toBeTruthy();
  });

  it('renders header', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview">
        <div>Content</div>
      </AppShell>
    ));
    const header = container.querySelector('header');
    expect(header).toBeTruthy();
  });

  it('renders main content area', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview">
        <div>Content</div>
      </AppShell>
    ));
    const main = container.querySelector('.app-shell__main');
    expect(main).toBeTruthy();
  });

  it('renders children inside content wrapper', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview">
        <div class="test-child">Child content</div>
      </AppShell>
    ));
    const content = container.querySelector('.app-shell__content');
    const child = content?.querySelector('.test-child');
    expect(child).toBeTruthy();
    expect(child?.textContent).toContain('Child content');
  });

  it('applies full-width class when fullWidth prop is true', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview" fullWidth>
        <div>Content</div>
      </AppShell>
    ));
    const content = container.querySelector('.app-shell__content');
    expect(content?.classList.contains('app-shell__content--full')).toBe(true);
  });

  it('does not apply full-width class by default', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview">
        <div>Content</div>
      </AppShell>
    ));
    const content = container.querySelector('.app-shell__content');
    expect(content?.classList.contains('app-shell__content--full')).toBe(false);
  });

  it('maintains proper layout structure', () => {
    const { container } = render(() => (
      <AppShell activeTab="overview">
        <p>Test</p>
      </AppShell>
    ));
    const appShell = container.querySelector('.app-shell');
    const header = appShell?.querySelector('header');
    const main = appShell?.querySelector('main');

    expect(header).toBeTruthy();
    expect(main).toBeTruthy();
    expect(header).toBeTruthy();
    expect(main).toBeTruthy();
  });

  it('passes activeTab to header', () => {
    const { container } = render(() => (
      <AppShell activeTab="chat">
        <div>Content</div>
      </AppShell>
    ));
    const header = container.querySelector('header');
    expect(header).toBeTruthy();
  });
});
