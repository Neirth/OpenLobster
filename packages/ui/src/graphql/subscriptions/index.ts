// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Shared GraphQL subscription strings used by both the web frontend and the terminal.
 *
 * Import with:
 *   import { MESSAGE_SUBSCRIPTION } from '@openlobster/ui/graphql/subscriptions';
 */

export const MESSAGE_SUBSCRIPTION = /* GraphQL */ `
  subscription OnMessageReceived {
    onMessageReceived {
      type
      timestamp
      data
    }
  }
`;
