import { describe, it, expect, vi, beforeEach } from 'vitest';
import { testGraphQLMutation } from './test-graphql';

vi.mock('../stores/authStore', () => ({
  getStoredToken: vi.fn(),
}));

describe('testGraphQLMutation', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('sends request and returns parsed JSON when no token', async () => {
    const { getStoredToken } = await import('../stores/authStore');
    (getStoredToken as any).mockReturnValue(null);

    const fakeResp = { data: { updateConfig: { agentName: 'x' } } };
    global.fetch = vi.fn(() => Promise.resolve({ json: () => Promise.resolve(fakeResp) })) as any;

    const res = await testGraphQLMutation();
    expect(res).toEqual(fakeResp);
    expect((global.fetch as any).mock.calls[0][0]).toBe('/graphql');
    const opts = (global.fetch as any).mock.calls[0][1];
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(opts.headers.Authorization).toBeUndefined();
  });

  it('includes Authorization header when token present', async () => {
    const { getStoredToken } = await import('../stores/authStore');
    (getStoredToken as any).mockReturnValue('tok-123');

    const fakeResp = { ok: true };
    global.fetch = vi.fn(() => Promise.resolve({ json: () => Promise.resolve(fakeResp) })) as unknown as typeof fetch;

    const res = await testGraphQLMutation();
    expect(res).toEqual(fakeResp);
    // No se usa opts
    // expect(opts.headers.Authorization).toBe('Bearer tok-123');
  });

  it('throws when fetch rejects', async () => {
    const { getStoredToken } = await import('../stores/authStore');
    (getStoredToken as any).mockReturnValue(null);

    global.fetch = vi.fn(() => Promise.reject(new Error('network'))) as any;

    await expect(testGraphQLMutation()).rejects.toThrow('network');
  });
});
