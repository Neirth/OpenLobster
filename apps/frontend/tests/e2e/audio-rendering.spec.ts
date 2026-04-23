import { test, expect } from '@playwright/test';

test('should render premium voice note and NOT show voice.ogg attachment when audio metadata is present', async ({ page }) => {
  await page.route('**/graphql', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const body = route.request().postDataJSON();
    const query = typeof body?.query === 'string' ? body.query : '';

    if (query.includes('GetConfig')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { config: { wizardCompleted: true, agent: { name: 'Lobster' }, pluginDefaults: { audio: 'elevenlabs' } } } }),
      });
    }

    if (query.includes('GetConversations')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            conversations: [
              {
                id: 'conv-1',
                channelId: 'channel-1',
                channelName: 'Telegram',
                isGroup: false,
                participantId: 'user-1',
                participantName: 'Sergio',
                lastMessageAt: new Date().toISOString(),
                unreadCount: 0
              }
            ]
          }
        }),
      });
    }

    if (query.includes('GetMessages')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            messages: [
              {
                id: 'msg-audio-1',
                conversationId: 'conv-1',
                role: 'user',
                content: '',
                createdAt: new Date().toISOString(),
                attachments: [
                  {
                    type: 'voice',
                    url: 'http://fake/voice.ogg',
                    filename: 'voice.ogg',
                    mimeType: 'audio/ogg'
                  }
                ],
                audio: {
                  transcription: 'Eh, ¿estás funcional ya?',
                  format: 'audio/ogg',
                  durationMs: 4000
                }
              }
            ]
          }
        }),
      });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: {} }) });
  });

  await page.goto('/chat');
  
  const convRow = page.locator('.conv-row').first();
  await expect(convRow).toBeVisible({ timeout: 15000 });
  await convRow.click();
  
  const voiceNoteBox = page.locator('.msg__audio');
  await expect(voiceNoteBox).toBeVisible({ timeout: 15000 });
  
  // EXPECTED TO FAIL: .msg__attachments should NOT be visible
  const attachmentBox = page.locator('.msg__attachments');
  await expect(attachmentBox).not.toBeVisible({ timeout: 3000 });
});

test('should render generic attachment if audio metadata is missing', async ({ page }) => {
  await page.route('**/graphql', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const body = route.request().postDataJSON();
    const query = typeof body?.query === 'string' ? body.query : '';

    if (query.includes('GetConfig')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { config: { wizardCompleted: true } } }) });
    }

    if (query.includes('GetConversations')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            conversations: [{ id: 'conv-1', channelId: 'c1', channelName: 'C1', isGroup: false, participantId: 'u1', participantName: 'Sergio', lastMessageAt: new Date().toISOString(), unreadCount: 0 }]
          }
        }),
      });
    }

    if (query.includes('GetMessages')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            messages: [
              {
                id: 'msg-file-1',
                conversationId: 'conv-1',
                role: 'user',
                content: '',
                createdAt: new Date().toISOString(),
                attachments: [
                  {
                    type: 'file',
                    url: 'http://fake/document.pdf',
                    filename: 'document.pdf',
                    mimeType: 'application/pdf'
                  }
                ],
                audio: null
              }
            ]
          }
        }),
      });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: {} }) });
  });

  await page.goto('/chat');
  const convRow = page.locator('.conv-row').first();
  await expect(convRow).toBeVisible({ timeout: 15000 });
  await convRow.click();
  
  const attachmentBox = page.locator('.msg__attachments');
  await expect(attachmentBox).toBeVisible({ timeout: 10000 });
  const voiceNoteBox = page.locator('.msg__audio');
  await expect(voiceNoteBox).not.toBeVisible();
});
