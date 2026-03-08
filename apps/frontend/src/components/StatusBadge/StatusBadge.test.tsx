// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */

import { describe, it, expect, vi } from 'vitest';
import { render } from '@solidjs/testing-library';
import StatusBadge from './StatusBadge';

// Mock @openlobster/ui/types
vi.mock('@openlobster/ui/types', () => ({}));

describe('StatusBadge Component', () => {
  it('renders status badge container', () => {
    const { container } = render(() => <StatusBadge status="online" />);
    const badge = container.querySelector('.status-badge');
    expect(badge).toBeTruthy();
  });

  it('renders status dot', () => {
    const { container } = render(() => <StatusBadge status="online" />);
    const dot = container.querySelector('.status-badge__dot');
    expect(dot).toBeTruthy();
  });

  it('renders with online status', () => {
    const { container } = render(() => <StatusBadge status="online" />);
    const dot = container.querySelector('.status-badge__dot') as HTMLElement;
    expect(dot?.style.background).toContain('--color-success');
  });

  it('renders with offline status', () => {
    const { container } = render(() => <StatusBadge status="offline" />);
    const dot = container.querySelector('.status-badge__dot') as HTMLElement;
    expect(dot?.style.background).toContain('--color-error');
  });

  it('renders with degraded status', () => {
    const { container } = render(() => <StatusBadge status="degraded" />);
    const dot = container.querySelector('.status-badge__dot') as HTMLElement;
    expect(dot?.style.background).toContain('--color-warning');
  });

  it('renders with unknown status', () => {
    const { container } = render(() => <StatusBadge status="unknown" />);
    const dot = container.querySelector('.status-badge__dot') as HTMLElement;
    expect(dot?.style.background).toContain('--color-text-muted');
  });

  it('renders label when provided', () => {
    const { container } = render(() => <StatusBadge status="online" label="Connected" />);
    const label = container.querySelector('.status-badge__label');
    expect(label).toBeTruthy();
    expect(label?.textContent).toContain('Connected');
  });

  it('does not render label when not provided', () => {
    const { container } = render(() => <StatusBadge status="online" />);
    const label = container.querySelector('.status-badge__label');
    expect(label).toBeFalsy();
  });

  it('renders dot and label together', () => {
    const { container } = render(() => <StatusBadge status="offline" label="Disconnected" />);
    const dot = container.querySelector('.status-badge__dot');
    const label = container.querySelector('.status-badge__label');
    expect(dot).toBeTruthy();
    expect(label).toBeTruthy();
    expect(label?.textContent).toContain('Disconnected');
  });

  it('maintains semantic structure', () => {
    const { container } = render(() => <StatusBadge status="online" label="Ready" />);
    const badge = container.querySelector('.status-badge');
    expect(badge?.querySelectorAll('.status-badge__dot').length).toBe(1);
  });

  it('does not render label when label is empty string', () => {
    const { container } = render(() => <StatusBadge status="online" label="" />);
    expect(container.querySelector('.status-badge__label')).toBeFalsy();
  });
});
