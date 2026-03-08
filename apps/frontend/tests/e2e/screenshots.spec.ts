// Copyright (c) OpenLobster contributors. See LICENSE for details.
import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const screenshotsDir = path.join(__dirname, '../../docs/playwright');

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
      await page.goto(view.path);
      // Wait for the app to mount and render content
      await page.waitForSelector('#app', { state: 'attached' });
      await page.waitForSelector(view.selector, { state: 'visible', timeout: 10000 });
      // Extra wait for fonts/images to load
      await page.waitForTimeout(500);

      const screenshotPath = path.join(screenshotsDir, `${view.name}.png`);
      await page.screenshot({ path: screenshotPath, fullPage: true });
      expect(fs.existsSync(screenshotPath)).toBe(true);
    });
  }
});
