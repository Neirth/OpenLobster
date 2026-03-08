// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */
import { test } from '@playwright/test';

test.skip('capture all view screenshots', async ({ page }) => {
  const views = [
    { path: '/', name: 'dashboard' },
    { path: '/chat', name: 'chat' },
    { path: '/tasks', name: 'tasks' },
    { path: '/memory', name: 'memory' },
    { path: '/mcps', name: 'mcps' },
    { path: '/skills', name: 'skills' },
    { path: '/settings', name: 'settings' },
  ];

  for (const view of views) {
    // eslint-disable-next-line no-console
    console.log(`\nCapturing screenshot for ${view.name}...`);
    await page.goto(view.path);

    // Wait for main content to load
    await page.waitForSelector('.app-shell', { timeout: 5000 }).catch(() => {});

    // eslint-disable-next-line no-console
    console.log(`✓ Screenshot test skipped for: ${view.name}`);
  }
});
