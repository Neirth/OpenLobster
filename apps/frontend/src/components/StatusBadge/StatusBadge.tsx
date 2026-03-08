// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * StatusBadge renders a colored dot with an optional label.
 * Color reflects the semantic connection status.
 *
 * @param status - 'online' | 'offline' | 'degraded' | 'unknown'
 * @param label  - Optional text to show beside the dot
 */

import type { Component } from 'solid-js';
import type { ConnectionStatus } from '@openlobster/ui/types';
import './StatusBadge.css';

interface StatusBadgeProps {
  status: ConnectionStatus;
  label?: string;
}

const STATUS_COLORS: Record<ConnectionStatus, string> = {
  online: 'var(--color-success)',
  offline: 'var(--color-error)',
  degraded: 'var(--color-warning)',
  unknown: 'var(--color-text-muted)',
  unauthorized: 'var(--color-warning)',
};

const StatusBadge: Component<StatusBadgeProps> = (props) => {
  return (
    <span class="status-badge">
      <span
        class="status-badge__dot"
        style={{ background: STATUS_COLORS[props.status] ?? STATUS_COLORS.unknown }}
      />
      {props.label && (
        <span class="status-badge__label">{props.label}</span>
      )}
    </span>
  );
};

export default StatusBadge;
