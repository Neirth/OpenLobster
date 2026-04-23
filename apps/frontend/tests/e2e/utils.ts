import { Page } from '@playwright/test';

/**
 * Shared backend mocks for E2E tests to allow the frontend to load without a running Go backend.
 * Handles GraphQL, metrics, health, and other proxied endpoints.
 */
export async function stubBackend(page: Page) {
  // 1. Handle GraphQL operations
  await page.route('**/graphql', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();

    const body = route.request().postDataJSON();
    const query = typeof body?.query === 'string' ? body.query : '';

    const mockData: Record<string, any> = {
      GetConfig: {
        config: {
          wizardCompleted: true,
          agent: { name: 'Lobster', provider: 'openai' },
          capabilities: { browser: true, terminal: true },
          pluginDefaults: { ai: 'openai', memory: 'gml', secrets: 'json' }
        }
      },
      GetAgent: { agent: { id: '1', name: 'Lobster', status: 'ready', version: '1.0.0' } },
      GetMetrics: { metrics: { uptime: 3600, messagesSent: 10, messagesReceived: 5 } },
      GetChannels: { channels: [] },
      GetConversations: { conversations: [] }
    };

    for (const [operation, data] of Object.entries(mockData)) {
      if (query.includes(operation)) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data }),
        });
      }
    }

    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: {} }),
    });
  });

  // 2. Handle common non-GraphQL endpoints to prevent proxy 502/504 errors in CI
  const genericEndpoints = ['**/health', '**/metrics', '**/logs', '**/oauth/**'];
  for (const pattern of genericEndpoints) {
    await page.route(pattern, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'ok' }),
      });
    });
  }

  // 3. Mock WebSockets to prevent connection failures
  await page.route('**/ws', async (route) => {
    await route.abort(); // or fulfill with 101 if playwright supports it better, but aborting the HTTP upgrade is usually fine to prevent 502s
  });
}

/** Alias for backward compatibility with existing tests */
export const stubGraphQL = stubBackend;
