// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */

import { describe, it, expect, vi } from 'vitest';

// Mock the UI library
vi.mock('@openlobster/ui/graphql', () => ({
  createGraphqlClient: (endpoint: string) => ({
    endpoint,
    request: vi.fn(),
  }),
}));

import { client } from './client';

describe('GraphQL Client', () => {
  it('exports a client instance', () => {
    expect(client).toBeTruthy();
  });

  it('client has request method', () => {
    expect(typeof client.request).toBe('function');
  });

  it('client is properly configured', () => {
    expect(client).toBeTruthy();
    expect(client.request).toBeDefined();
  });
});
