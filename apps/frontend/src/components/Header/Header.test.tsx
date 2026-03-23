import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@solidjs/testing-library';

vi.mock('@solidjs/router', () => ({
  A: (props: any) => <a {...props} />,
}));

vi.mock('@openlobster/ui/hooks', () => ({
  useAgent: (_client?: any) => ({ data: { name: 'AgentX', version: '1.2.3' } }),
}));

vi.mock('../../stores/wsStore', () => ({
  wsConnected: () => true,
}));

const mockPending = [
  { requestID: 'r1', channelType: 'telegram', displayName: 'Alice' },
];

const openPairingMock = vi.fn();

vi.mock('../../stores/pairingStore', () => ({
  pendingPairingsQueue: () => mockPending,
  openPairingRequest: (req: any) => openPairingMock(req),
}));

import Header from './Header';

describe('Header Component', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('renders brand and tabs and agent info', () => {
    const { container } = render(() => <Header activeTab="chat" />);
    expect(container.querySelector('.header__wordmark')?.textContent).toBeTruthy();
    expect(container.querySelector('.header__tab--active')).toBeTruthy();
    expect(container.querySelector('.header__agent-name')?.textContent).toBe('AgentX');
    expect(container.querySelector('.header__version')?.textContent).toContain('1.2.3');
  });

  it('shows pending pairings badge and opens dropdown', async () => {
    const { container, findByText } = render(() => <Header activeTab="overview" />);
    // pending badge should be present
    const badge = container.querySelector('.header__pairing-badge');
    expect(badge?.textContent).toBe('1');

    // click to open dropdown
    const btn = container.querySelector('.header__pairing-btn') as HTMLElement;
    await fireEvent.click(btn);
    // item should be visible
    expect(await findByText('Alice')).toBeTruthy();

    // clicking the item should call openPairingRequest (mocked)
    const item = container.querySelector('.header__pairing-item') as HTMLElement;
    await fireEvent.click(item);
    expect(openPairingMock).toHaveBeenCalled();
  });
});
// Copyright (c) OpenLobster contributors. See LICENSE for details.
