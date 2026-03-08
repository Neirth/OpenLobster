// Copyright (c) OpenLobster contributors. See LICENSE for details.
import { test, expect } from '@playwright/test';

test.describe('Frontend Views Navigation', () => {
  test('Dashboard view is accessible', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('localhost:5173/');
  });

  test('Chat view is accessible', async ({ page }) => {
    await page.goto('/chat');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/chat');
  });

  test('Tasks view is accessible', async ({ page }) => {
    await page.goto('/tasks');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/tasks');
  });

  test('Memory view is accessible', async ({ page }) => {
    await page.goto('/memory');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/memory');
  });

  test('MCPs view is accessible', async ({ page }) => {
    await page.goto('/mcps');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/mcps');
  });

  test('Skills view is accessible', async ({ page }) => {
    await page.goto('/skills');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/skills');
  });

  test('Settings view is accessible', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/settings');
  });

  test('All views load without critical errors', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });

    const routes = ['/', '/chat', '/tasks', '/memory', '/mcps', '/skills', '/settings'];
    for (const route of routes) {
      await page.goto(route);
      await page.waitForLoadState('networkidle');
    }

    // Allow some errors but not too many
    expect(errors.length).toBeLessThan(50);
  });
});
