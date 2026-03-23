// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi, beforeEach } from 'vitest';

// ------------------------------------------------------------------ //
// Mock @openlobster/ui/graphql so tests never need a real HTTP server  //
// ------------------------------------------------------------------ //

var mockRequest = vi.fn();

vi.mock('@openlobster/ui/graphql', () => ({
  createGraphqlClient: (_endpoint: string, _getToken?: () => string | null) => ({
    request: mockRequest,
  }),
}));

// Import after mocks are in place
import { client, GRAPHQL_ENDPOINT } from './client';

describe('GraphQL Client', () => {
  beforeEach(() => {
    mockRequest.mockReset();
  });

  // ------------------------------------------------------------------ //
  // Export shape                                                         //
  // ------------------------------------------------------------------ //

  it('exports a client instance', () => {
    expect(client).toBeTruthy();
  });

  it('client has request method', () => {
    expect(typeof client.request).toBe('function');
  });

  it('exports GRAPHQL_ENDPOINT as a string', () => {
    expect(typeof GRAPHQL_ENDPOINT).toBe('string');
    expect(GRAPHQL_ENDPOINT.length).toBeGreaterThan(0);
  });

  it('GRAPHQL_ENDPOINT ends with /graphql', () => {
    expect(GRAPHQL_ENDPOINT.endsWith('/graphql')).toBe(true);
  });

  // ------------------------------------------------------------------ //
  // Wrapped request — success path                                       //
  // ------------------------------------------------------------------ //

  it('request passes through successful responses', async () => {
    const data = { foo: 'bar' };
    mockRequest.mockResolvedValueOnce(data);
    const result = await client.request('{ foo }');
    expect(result).toEqual(data);
  });

  it('request forwards arguments to the underlying client', async () => {
    mockRequest.mockResolvedValueOnce({});
    await client.request('query Q { field }', { var: 1 });
    expect(mockRequest).toHaveBeenCalledWith('query Q { field }', { var: 1 });
  });

  // ------------------------------------------------------------------ //
  // Wrapped request — 401 triggers needsAuth                            //
  // ------------------------------------------------------------------ //

  it('request re-throws non-401 errors', async () => {
    const err = Object.assign(new Error('server error'), {
      response: { status: 500 },
    });
    mockRequest.mockRejectedValueOnce(err);
    await expect(client.request('{ x }')).rejects.toThrow('server error');
  });

  it('request re-throws 401 error after setting needsAuth', async () => {
    // Import the store to inspect needsAuth signal
    const { needsAuth, setNeedsAuth } = await import('../stores/authStore');
    setNeedsAuth(false);

    const err = Object.assign(new Error('Unauthorized'), {
      response: { status: 401 },
    });
    mockRequest.mockRejectedValueOnce(err);

    await expect(client.request('{ x }')).rejects.toThrow('Unauthorized');
    expect(needsAuth()).toBe(true);
  });

  it('request re-throws errors without response property', async () => {
    const err = new Error('network failure');
    mockRequest.mockRejectedValueOnce(err);
    await expect(client.request('{ x }')).rejects.toThrow('network failure');
  });

  it('request does not set needsAuth for non-401 status codes', async () => {
    const { needsAuth, setNeedsAuth } = await import('../stores/authStore');
    setNeedsAuth(false);

    const err = Object.assign(new Error('forbidden'), {
      response: { status: 403 },
    });
    mockRequest.mockRejectedValueOnce(err);

    await expect(client.request('{ x }')).rejects.toThrow();
    expect(needsAuth()).toBe(false);
  });
});
