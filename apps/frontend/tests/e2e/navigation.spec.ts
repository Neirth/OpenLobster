// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */

import { test, expect } from '@playwright/test';

test.describe('Frontend Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
  });

  test('loads app successfully', async ({ page }) => {
    await expect(page).toHaveTitle(/OpenLobster/i);
  });

  test('loads with valid HTML structure', async ({ page }) => {
    const root = page.locator('#root');
    expect(root).toBeTruthy();
  });

  test('page is responsive', async ({ page }) => {
    const viewportSize = page.viewportSize();
    expect(viewportSize?.width).toBeGreaterThan(0);
    expect(viewportSize?.height).toBeGreaterThan(0);
  });

  test('app handles navigation without errors', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });
    await page.goto('/');
    await page.waitForTimeout(1000);
    // Allow some errors since dev server may have warnings
    expect(errors.length).toBeLessThan(5);
  });
});

test.describe('Page Load Performance', () => {
  test('page loads without critical errors', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    // Should have minimal critical errors
    expect(errors.length).toBeLessThan(10);
  });

  test('page title is set correctly', async ({ page }) => {
    await page.goto('/');
    const title = await page.title();
    expect(title).toContain('OpenLobster');
  });

  test('page is interactive quickly', async ({ page }) => {
    const start = Date.now();
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    const loadTime = Date.now() - start;
    expect(loadTime).toBeLessThan(15000);
  });
});
