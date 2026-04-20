// Copyright (c) OpenLobster contributors. See LICENSE for details.
import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const screenshotsDir = path.join(__dirname, '../../docs/playwright');

/** Intercept GraphQL so the app skips FirstBootWizard and shows the main shell (header).
 * Without this, the config fetch fails or returns wizardCompleted: false and the header never mounts. */
async function stubConfigWizardCompleted(page: import('@playwright/test').Page) {
  await page.route('**/graphql', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const body = route.request().postDataJSON();
    const query = typeof body?.query === 'string' ? body.query : '';
    if (query.includes('GetConfig') && query.includes('wizardCompleted')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { config: { wizardCompleted: true } } }),
      });
    }
    return route.continue();
  });
}

test.describe('Screenshot Capture', () => {
  test.beforeAll(async () => {
    if (!fs.existsSync(screenshotsDir)) {
      fs.mkdirSync(screenshotsDir, { recursive: true });
    }
  });

  const views = [
    { path: '/', name: 'dashboard', selector: 'header' },
    { path: '/chat', name: 'chat', selector: 'header' },
    { path: '/tasks', name: 'tasks', selector: 'header' },
    { path: '/memory', name: 'memory', selector: 'header' },
    { path: '/mcps', name: 'mcps', selector: 'header' },
    { path: '/skills', name: 'skills', selector: 'header' },
    { path: '/settings', name: 'settings', selector: 'header' },
  ];

  for (const view of views) {
    test(`capture ${view.name} screenshot`, async ({ page }) => {
      await stubConfigWizardCompleted(page);
      await page.goto(view.path);
      // Wait for the app to mount and render content (header only exists after wizard is skipped)
      await page.waitForLoadState('networkidle');
      await page.waitForSelector('#app', { state: 'attached', timeout: 15000 });
      await page.waitForSelector(view.selector, { state: 'visible', timeout: 15000 });
      // Extra wait for fonts/images to load
      await page.waitForTimeout(500);

      const screenshotPath = path.join(screenshotsDir, `${view.name}.png`);
      await page.screenshot({ path: screenshotPath, fullPage: true });
      expect(fs.existsSync(screenshotPath)).toBe(true);
    });
  }
});
