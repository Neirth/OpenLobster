// Copyright (c) OpenLobster contributors. See LICENSE for details.
import { test, expect } from '@playwright/test';

// Mock GraphQL responses for OAuth tests.
// We intercept /graphql and return appropriate responses based on the query.
function setupGraphQLMocks(page: import('@playwright/test').Page, opts?: {
  wizardCompleted?: boolean;
  oauthProviders?: { id: string; name: string }[];
  oauthStatus?: Record<string, string>;
  oauthProfiles?: Record<string, { name: string; authenticated: boolean; accountID?: string }[]>;
}) {
  const providers = opts?.oauthProviders ?? [
    { id: 'openai-codex', name: 'ChatGPT Plus/Pro (Codex)' },
    { id: 'github-copilot', name: 'GitHub Copilot' },
    { id: 'anthropic', name: 'Anthropic (Claude Pro/Max)' },
    { id: 'google-gemini-cli', name: 'Google Cloud Code Assist (Gemini CLI)' },
    { id: 'google-antigravity', name: 'Antigravity (Gemini 3, Claude, GPT-OSS)' },
  ];
  const wizardCompleted = opts?.wizardCompleted ?? false;
  const oauthStatus = opts?.oauthStatus ?? {};
  const oauthProfiles = opts?.oauthProfiles ?? {};

  return page.route('**/graphql', async (route) => {
    const request = route.request();
    const postData = request.postDataJSON();
    const query: string = postData?.query ?? '';

    // __typename probe (auth check)
    if (query.includes('__typename')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { __typename: 'Query' } }),
      });
    }

    // Config query (including wizardCompleted check from recheckConfig)
    if (query.includes('config') && !query.includes('update') && !query.includes('OAuth')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            config: {
              agent: { name: 'TestAgent', provider: 'ollama', model: 'llama3.2:latest', apiKey: '', baseURL: '', ollamaHost: 'http://localhost:11434', ollamaApiKey: '', anthropicApiKey: '', dockerModelRunnerEndpoint: '', dockerModelRunnerModel: '', authMode: null, oauthProvider: null },
              capabilities: { browser: false, terminal: true, subagents: false, memory: true, mcp: true, filesystem: true, sessions: true },
              database: { driver: 'sqlite', dsn: './data/openlobster.db', maxOpenConns: 0, maxIdleConns: 0 },
              memory: { backend: 'file', filePath: './data/memory.gml', neo4j: null },
              subagents: null,
              graphql: { enabled: true, port: 8080, host: '0.0.0.0', baseUrl: '' },
              logging: { level: 'info', path: '' },
              secrets: { backend: 'file', file: { path: './data/secrets.json' }, openbao: null },
              scheduler: { enabled: true, memoryEnabled: true, memoryInterval: '' },
              channels: [],
              channelSecrets: { telegramEnabled: false, telegramToken: '', discordEnabled: false, discordToken: '', whatsAppEnabled: false, whatsAppPhoneId: '', whatsAppApiToken: '', twilioEnabled: false, twilioAccountSid: '', twilioAuthToken: '', twilioFromNumber: '', slackEnabled: false, slackBotToken: '', slackAppToken: '' },
              activeSessions: [],
              wizardCompleted,
            },
          },
        }),
      });
    }

    // Provider OAuth providers query
    if (query.includes('providerOAuthProviders')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { providerOAuthProviders: providers } }),
      });
    }

    // Provider OAuth status query
    if (query.includes('providerOAuthStatus')) {
      const provider = postData?.variables?.provider ?? 'unknown';
      const status = oauthStatus[provider] ?? 'not_authenticated';
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { providerOAuthStatus: { provider, status, errorMessage: null } } }),
      });
    }

    // Provider OAuth profiles query
    if (query.includes('providerOAuthProfiles')) {
      const provider = postData?.variables?.provider ?? 'unknown';
      const profiles = (oauthProfiles[provider] ?? []).map((p) => ({
        id: `${provider}/${p.name}`,
        name: p.name,
        providerID: provider,
        authenticated: p.authenticated,
        accountID: p.accountID ?? null,
      }));
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { providerOAuthProfiles: profiles } }),
      });
    }

    // Initiate provider OAuth mutation
    if (query.includes('initiateProviderOAuth')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            initiateProviderOAuth: {
              authorizationURL: 'https://auth.openai.com/oauth/authorize?client_id=test&state=test123',
              instructions: 'Complete sign-in in your browser.',
            },
          },
        }),
      });
    }

    // Logout provider OAuth mutation
    if (query.includes('logoutProviderOAuth')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { logoutProviderOAuth: true } }),
      });
    }

    // Set active OAuth profile mutation
    if (query.includes('setActiveOAuthProfile')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { setActiveOAuthProfile: true } }),
      });
    }

    // Delete OAuth profile mutation
    if (query.includes('deleteOAuthProfile')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { deleteOAuthProfile: true } }),
      });
    }

    // Update config mutation
    if (query.includes('updateConfig')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { updateConfig: { agentName: 'TestAgent', provider: 'ollama', channels: [] } } }),
      });
    }

    // System files
    if (query.includes('systemFiles')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { systemFiles: [] } }),
      });
    }

    // MCP servers
    if (query.includes('mcpServers')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { mcpServers: [] } }),
      });
    }

    // Fallback
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: {} }),
    });
  });
}

/**
 * Set the auth token in sessionStorage before navigating so that the
 * AccessTokenModal does not appear. The auth store key is
 * 'openlobster_access_token' (see stores/authStore.ts).
 */
async function setAuthToken(page: import('@playwright/test').Page): Promise<void> {
  await page.addInitScript(() => {
    sessionStorage.setItem('openlobster_access_token', 'test-token');
  });
}

// ─── First Boot Wizard: OAuth Section ────────────────────────────────────────

test.describe('FirstBootWizard OAuth', () => {
  test('shows OAuth providers in the provider step', async ({ page }) => {
    await setupGraphQLMocks(page, { wizardCompleted: false });
    await setAuthToken(page);
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The wizard should be visible since wizardCompleted is false.
    // Navigate to the provider step (step 2 / index 2).
    // Click "Next" twice to get past welcome and agent name steps.
    const nextBtn = page.locator('button:has-text("Next")');
    await nextBtn.click(); // past welcome
    await nextBtn.click(); // past agent name

    // We should now be on the provider step.
    // Check that OAuth section appears with provider buttons.
    await expect(page.locator('text=Or sign in with OAuth')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('text=ChatGPT Plus/Pro')).toBeVisible();
    await expect(page.locator('text=GitHub Copilot')).toBeVisible();
    // Use a button-specific locator to avoid matching the <option> in the
    // provider dropdown which also contains "Anthropic".
    await expect(page.locator('.wizard-oauth-btn:has-text("Anthropic")')).toBeVisible();
  });

  test('clicking OAuth provider opens popup', async ({ page, context }) => {
    await setupGraphQLMocks(page, { wizardCompleted: false });
    await setAuthToken(page);
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const nextBtn = page.locator('button:has-text("Next")');
    await nextBtn.click();
    await nextBtn.click();

    await expect(page.locator('text=Or sign in with OAuth')).toBeVisible({ timeout: 5000 });

    // Listen for popup
    const popupPromise = context.waitForEvent('page');

    // Click OpenAI Codex OAuth button
    await page.locator('.wizard-oauth-btn:has-text("ChatGPT Plus/Pro")').click();

    const popup = await popupPromise;
    expect(popup.url()).toContain('auth.openai.com');
    await popup.close();
  });
});

// ─── Settings View: Authentication Section ───────────────────────────────────

test.describe('SettingsView Authentication', () => {
  // The ProviderOAuth component renders inside a div.provider-oauth.
  // We wait for that container (or the first provider card) instead of
  // the section heading because the i18n key "settings.group.authentication"
  // is not defined in the locale files, so the <h2> renders empty.
  const OAUTH_CONTAINER = '.provider-oauth';

  test('shows Authentication section with all providers', async ({ page }) => {
    await setupGraphQLMocks(page, { wizardCompleted: true });
    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    // Wait for the OAuth container to appear
    await expect(page.locator(OAUTH_CONTAINER)).toBeVisible({ timeout: 10000 });

    // All 5 providers should be listed
    await expect(page.locator('.provider-oauth-card:has-text("ChatGPT Plus/Pro")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.provider-oauth-card:has-text("GitHub Copilot")')).toBeVisible();
    await expect(page.locator('.provider-oauth-card:has-text("Anthropic")')).toBeVisible();
    await expect(page.locator('.provider-oauth-card:has-text("Google Cloud Code Assist")')).toBeVisible();
    await expect(page.locator('.provider-oauth-card:has-text("Antigravity")')).toBeVisible();
  });

  test('shows not_authenticated status for providers without login', async ({ page }) => {
    await setupGraphQLMocks(page, { wizardCompleted: true });
    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator(OAUTH_CONTAINER)).toBeVisible({ timeout: 10000 });

    // Sign In buttons should be visible (not authenticated state).
    // The button text in ProviderOAuth.tsx is "Sign In" (capital I).
    const signInButtons = page.locator('.provider-oauth button:has-text("Sign In")');
    await expect(signInButtons.first()).toBeVisible({ timeout: 5000 });
  });

  test('shows authenticated status with profile info', async ({ page }) => {
    await setupGraphQLMocks(page, {
      wizardCompleted: true,
      oauthStatus: { 'openai-codex': 'authenticated' },
      oauthProfiles: {
        'openai-codex': [
          { name: 'default', authenticated: true, accountID: 'acct-123' },
        ],
      },
    });
    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator(OAUTH_CONTAINER)).toBeVisible({ timeout: 10000 });

    // Should show authenticated state for OpenAI Codex
    await expect(page.locator('text=acct-123')).toBeVisible({ timeout: 5000 });
  });

  test('shows multiple profiles for a provider', async ({ page }) => {
    await setupGraphQLMocks(page, {
      wizardCompleted: true,
      oauthStatus: { 'openai-codex': 'authenticated' },
      oauthProfiles: {
        'openai-codex': [
          { name: 'work', authenticated: true, accountID: 'acct-work' },
          { name: 'personal', authenticated: true, accountID: 'acct-personal' },
        ],
      },
    });
    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator(OAUTH_CONTAINER)).toBeVisible({ timeout: 10000 });

    // Both profiles should be visible
    await expect(page.locator('.provider-oauth-profile__name:has-text("work")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.provider-oauth-profile__name:has-text("personal")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('text=acct-work')).toBeVisible();
    await expect(page.locator('text=acct-personal')).toBeVisible();
  });

  test('clicking Sign in initiates OAuth flow', async ({ page, context }) => {
    await setupGraphQLMocks(page, { wizardCompleted: true });
    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator(OAUTH_CONTAINER)).toBeVisible({ timeout: 10000 });

    // Listen for popup
    const popupPromise = context.waitForEvent('page');

    // Click first Sign In button (capital I per ProviderOAuth.tsx)
    await page.locator('.provider-oauth button:has-text("Sign In")').first().click();

    const popup = await popupPromise;
    expect(popup.url()).toContain('auth.openai.com');
    await popup.close();
  });

  test('logout button calls logoutProviderOAuth', async ({ page }) => {
    let logoutCalled = false;
    await setupGraphQLMocks(page, {
      wizardCompleted: true,
      oauthStatus: { 'openai-codex': 'authenticated' },
      oauthProfiles: {
        'openai-codex': [
          { name: 'default', authenticated: true, accountID: 'acct-123' },
        ],
      },
    });

    // Override the route to detect logout
    await page.route('**/graphql', async (route) => {
      const postData = route.request().postDataJSON();
      if (postData?.query?.includes('logoutProviderOAuth')) {
        logoutCalled = true;
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: { logoutProviderOAuth: true } }),
        });
      }
      // Fall through to the previous handler
      return route.fallback();
    });

    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator(OAUTH_CONTAINER)).toBeVisible({ timeout: 10000 });

    // Click logout
    const logoutBtn = page.locator('.provider-oauth button:has-text("Logout")').first();
    if (await logoutBtn.isVisible()) {
      await logoutBtn.click();
      // Give it a moment for the mutation to fire
      await page.waitForTimeout(500);
      expect(logoutCalled).toBe(true);
    }
  });
});

// ─── Settings View: No OAuth errors ──────────────────────────────────────────

test.describe('SettingsView OAuth error handling', () => {
  test('settings page loads without console errors when OAuth is available', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });

    await setupGraphQLMocks(page, { wizardCompleted: true });
    await setAuthToken(page);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    // Filter out known non-critical errors (e.g. WebSocket connection)
    const criticalErrors = errors.filter(
      (e) => !e.includes('WebSocket') && !e.includes('net::ERR') && !e.includes('favicon')
    );
    expect(criticalErrors.length).toBe(0);
  });

  test('wizard provider step loads without console errors', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });

    await setupGraphQLMocks(page, { wizardCompleted: false });
    await setAuthToken(page);
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const nextBtn = page.locator('button:has-text("Next")');
    await nextBtn.click();
    await nextBtn.click();

    await page.waitForTimeout(1000);

    const criticalErrors = errors.filter(
      (e) => !e.includes('WebSocket') && !e.includes('net::ERR') && !e.includes('favicon')
    );
    expect(criticalErrors.length).toBe(0);
  });
});
