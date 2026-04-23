import { test, expect } from '@playwright/test';
import { stubGraphQL } from './utils';

test('should trigger graphql calls on navigation', async ({ page }) => {
  const graphqlRequests: string[] = [];
  
  // Use our shared stub to avoid connection errors, but also track calls
  await stubGraphQL(page);
  
  page.on('request', request => {
    if (request.url().includes('/graphql')) {
      graphqlRequests.push(request.url());
    }
  });

  await page.goto('/');
  
  // Navigate to Tasks
  await page.click('a[href="/tasks"]');
  await page.waitForTimeout(500);
  
  // Navigate to Chat
  await page.click('a[href="/chat"]');
  await page.waitForTimeout(500);

  expect(graphqlRequests.length).toBeGreaterThan(0);
});
