// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect } from 'vitest';
import { createGraphqlClient } from './client';

describe('createGraphqlClient', () => {
  it('returns a GraphQLClient instance', () => {
    const client = createGraphqlClient('http://localhost:8080/graphql');
    expect(client).toBeDefined();
    expect(typeof client.request).toBe('function');
  });

  it('stores the provided URL', () => {
    const url = 'http://localhost:8080/graphql';
    const client = createGraphqlClient(url);
    // graphql-request exposes the URL on the client instance
    expect((client as unknown as { url: string }).url).toBe(url);
  });

  it('creates independent client instances for different URLs', () => {
    const clientA = createGraphqlClient('http://localhost:8080/graphql');
    const clientB = createGraphqlClient('http://localhost:9090/graphql');
    expect(clientA).not.toBe(clientB);
    expect((clientA as unknown as { url: string }).url).not.toBe(
      (clientB as unknown as { url: string }).url,
    );
  });

  it('creates independent instances on repeated calls with the same URL', () => {
    const url = 'http://localhost:8080/graphql';
    const clientA = createGraphqlClient(url);
    const clientB = createGraphqlClient(url);
    // Factory pattern — each call returns a new instance
    expect(clientA).not.toBe(clientB);
  });

  it('works with a relative URL (web frontend proxy pattern)', () => {
    const client = createGraphqlClient('/graphql');
    expect(client).toBeDefined();
    expect(typeof client.request).toBe('function');
  });
});
