// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */

import { test, expect } from '@playwright/test';

test.describe('Frontend Application', () => {
  test('application loads successfully', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Verify page title
    const title = await page.title();
    expect(title).toContain('OpenLobster');
  });

  test('root element exists', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const root = await page.locator('#app').count();
    expect(root).toBe(1);
  });

  test('page loads without hanging', async ({ page }) => {
    const start = Date.now();
    await page.goto('/');
    const duration = Date.now() - start;

    // Should load in reasonable time
    expect(duration).toBeLessThan(10000);
  });

  test('network requests complete', async ({ page }) => {
    let failedRequests = 0;
    page.on('response', (response) => {
      // Only count server errors (5xx), not client errors (4xx) which might be expected
      if (response.status() >= 500) {
        failedRequests++;
      }
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');

    expect(failedRequests).toBe(0);
  });
});
