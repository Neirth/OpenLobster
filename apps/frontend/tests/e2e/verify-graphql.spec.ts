import { test, expect } from '@playwright/test';

test('should trigger graphql calls on navigation', async ({ page }) => {
  const graphqlRequests: string[] = [];
  page.on('request', request => {
    if (request.url().includes('/graphql')) {
      graphqlRequests.push(request.url());
    }
  });

  await page.goto('http://localhost:8080/');
  
  // Navigate to Tasks
  await page.click('a[href="/tasks"]');
  await page.waitForTimeout(1000);
  
  // Navigate to Chat
  await page.click('a[href="/chat"]');
  await page.waitForTimeout(1000);

  console.log('Detected GraphQL Requests:', graphqlRequests);
  expect(graphqlRequests.length).toBeGreaterThan(0);
});
