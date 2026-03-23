// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * ChatView — the Chat tab.
 *
 * Left panel: conversation list (280px).
 * Right panel: message thread + compose input.
 */

import type { Component } from 'solid-js';
import { createSignal, For, Show, Suspense } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useConversations, useSubscriptions } from '@openlobster/ui/hooks';
import { SEND_MESSAGE_MUTATION, DELETE_USER_MUTATION, DELETE_GROUP_MUTATION } from '@openlobster/ui/graphql/mutations';
import type { Message } from '@openlobster/ui/types';
import { t } from '../../App';
import { client, GRAPHQL_ENDPOINT } from '../../graphql/client';
import AppShell from '../../components/AppShell';
import MessageThread from '../../components/MessageThread/MessageThread';
import ContextMenu from '../../components/ContextMenu/ContextMenu';
import { formatChatTime } from '../../utils/formatChatTime';
import './ChatView.css';

const QUICK_EMOJIS = ['😀', '😂', '🔥', '✅', '🙏', '👍', '🎉', '🤖'];

const ChatView: Component = () => {
  const [selectedId, setSelectedId] = createSignal('');
  const [draft, setDraft] = createSignal('');
  const [emojiPickerOpen, setEmojiPickerOpen] = createSignal(false);
  const [deleteModalOpen, setDeleteModalOpen] = createSignal(false);
  const [confirmName, setConfirmName] = createSignal('');
  const [msgCount, setMsgCount] = createSignal(0);

  const conversations = useConversations(client);
  const queryClient = useQueryClient();

  // Same base as GraphQL client: current origin + path, so subscriptions work on any domain
  const wsUrl = GRAPHQL_ENDPOINT.startsWith('/')
    ? GRAPHQL_ENDPOINT.replace(/\/graphql\/?$/, '/ws')
    : GRAPHQL_ENDPOINT.replace(/\/graphql\/?$/, '/ws').replace(/^https/, 'wss').replace(/^http/, 'ws');

  useSubscriptions({
    url: wsUrl,
    onMessageSent: (data: unknown) => {
      const payload =
        typeof data === 'string'
          ? (JSON.parse(data) as Record<string, unknown>)
          : (data as Record<string, unknown>);
      if (String(payload['ChannelType'] ?? '').toLowerCase() === 'loopback') return;
      const conversationId = payload['ChannelID'] as string | undefined;
      if (!conversationId) return;

      const newMessage: Message = {
        id: payload['MessageID'] as string,
        conversationId,
        role: ((payload['Role'] as string) || 'user') as Message['role'],
        content: (payload['Content'] as string) || '',
        createdAt: (payload['Timestamp'] as string) || new Date().toISOString(),
        attachments: Array.isArray(payload['Attachments'])
          ? (payload['Attachments'] as Record<string, unknown>[]).map((a) => ({
              type: (a['Type'] as string) || (a['type'] as string) || '',
              filename: (a['Filename'] as string) || (a['filename'] as string) || undefined,
              mimeType:
                (a['MIMEType'] as string) ||
                (a['mimeType'] as string) ||
                (a['mime_type'] as string) ||
                undefined,
            }))
          : undefined,
      };

      const slot = queryClient.getQueryData<{ append: (m: Message) => void }>(['messages-append', conversationId]);
      if (slot?.append) {
        slot.append(newMessage);
      }

      void queryClient.invalidateQueries({ queryKey: ['conversations'] });
    },
  });

  const sendMsg = createMutation(() => ({
    mutationFn: (vars: { conversationId: string; content: string }) =>
      client.request(SEND_MESSAGE_MUTATION, vars),
    onSuccess: () => {
      setDraft('');
    },
  }));

  const deleteUser = createMutation(() => ({
    mutationFn: (vars: { conversationId: string }) =>
      client.request(DELETE_USER_MUTATION, vars),
    onSuccess: () => {
      setDeleteModalOpen(false);
      setConfirmName('');
      setSelectedId('');
      void queryClient.invalidateQueries({ queryKey: ['conversations'] });
    },
  }));

  const deleteGroup = createMutation(() => ({
    mutationFn: (vars: { conversationId: string }) =>
      client.request(DELETE_GROUP_MUTATION, vars),
    onSuccess: () => {
      setDeleteModalOpen(false);
      setConfirmName('');
      setSelectedId('');
      void queryClient.invalidateQueries({ queryKey: ['conversations'] });
    },
  }));

  function handleSend() {
    const content = draft().trim();
    if (!content || !selectedId()) return;
    // Clear the input immediately for responsive UX, mutation uses captured `content`.
    setDraft('');
    sendMsg.mutate({ conversationId: selectedId(), content });
    setEmojiPickerOpen(false);
  }

  function handleKeyDown(e: KeyboardEvent & { ctrlKey: boolean; metaKey: boolean }) {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      handleSend();
    }
  }

  function insertEmoji(emoji: string) {
    setDraft((prev) => `${prev}${emoji}`);
  }

  const selectedConv = () => conversations.data?.find((c) => c.id === selectedId());

  return (
    <AppShell activeTab="chat" fullHeight>
      <Show when={!conversations.isLoading && conversations.data && conversations.data.length === 0}>
        <div class="chat-empty">
          <span class="material-symbols-outlined chat-empty__icon">smart_toy</span>
          <p class="chat-empty__title">{t('chat.noConversations')}</p>
          <p class="chat-empty__hint">{t('chat.noConversationsHint')}</p>
        </div>
      </Show>
      <Show when={!(!conversations.isLoading && conversations.data && conversations.data.length === 0)}>
        <div class="chat-layout">
          {/* Left: conversation list */}
          <aside class="chat-sidebar">
            <div class="chat-sidebar__header">
              <span class="chat-sidebar__title">{t('chat.conversations')}</span>
            </div>
            <div class="chat-sidebar__list">
              <Suspense>
                <For each={conversations.data} fallback={null}>
                  {(conv) => (
                    <ContextMenu items={[
                      { label: conv.isGroup ? t('chat.deleteGroup.button') : t('chat.deleteUser.button'), icon: conv.isGroup ? 'group_remove' : 'person_remove', danger: true, onClick: () => { setSelectedId(conv.id); setConfirmName(''); setDeleteModalOpen(true); } },
                    ]}>
                      <button
                        class="conv-row"
                        classList={{ 'conv-row--active': selectedId() === conv.id }}
                        onClick={() => setSelectedId(conv.id)}
                      >
                        <Show
                          when={conv.isGroup}
                          fallback={
                            <span class="conv-row__avatar">
                              {conv.participantName.charAt(0).toUpperCase()}
                            </span>
                          }
                        >
                          <span class="conv-row__avatar conv-row__avatar--group" aria-label={conv.participantName}>
                            <span class="conv-row__avatar-back" />
                            <span class="conv-row__avatar-front">
                              {conv.participantName.charAt(0).toUpperCase()}
                            </span>
                          </span>
                        </Show>
                        <div class="conv-row__body">
                          <div class="conv-row__top">
                            <span class="conv-row__name">{conv.isGroup && conv.groupName ? conv.groupName : conv.participantName}</span>
                          </div>
                          <div class="conv-row__preview">
                            {conv.lastMessageAt
                              ? formatChatTime(conv.lastMessageAt)
                              : ''}
                          </div>
                        </div>
                      </button>
                    </ContextMenu>
                  )}
                </For>
              </Suspense>
            </div>
          </aside>

          {/* Right: message thread */}
          <div class="chat-thread">
            <Show
              when={selectedId()}
              fallback={
                <div class="chat-thread__empty">
                  <span class="material-symbols-outlined chat-thread__empty-icon">forum</span>
                  <p>{t('chat.selectConversation')}</p>
                </div>
              }
            >
              {/* Thread header */}
              <div class="chat-thread__header">
                <span class="chat-thread__participant">
                  {selectedConv() ? (selectedConv()!.isGroup && selectedConv()!.groupName ? selectedConv()!.groupName : selectedConv()!.participantName) : selectedId()}
                </span>
                <Show when={selectedConv() && !selectedConv()!.isGroup}>
                  <span class="chat-thread__channel-badge">{selectedConv()?.channelName}</span>
                </Show>
                <span class="chat-thread__msg-count">
                  {msgCount()} {t('chat.messages')}
                </span>
                <button
                  class="chat-thread__delete-btn"
                  title={selectedConv()?.isGroup ? t('chat.deleteGroup.button') : t('chat.deleteUser.button')}
                  onClick={() => { setConfirmName(''); setDeleteModalOpen(true); }}
                >
                  <span class="material-symbols-outlined">
                    {selectedConv()?.isGroup ? 'group_remove' : 'person_remove'}
                  </span>
                </button>
              </div>

              {/* Virtualised messages */}
              <MessageThread
                conversationId={selectedId()}
                onNewMessageCount={setMsgCount}
                participantName={selectedConv()?.participantName}
              />

              {/* Compose */}
              <div class="chat-thread__compose">
                <input
                  class="compose-input"
                  type="text"
                  placeholder={t('chat.typeMessageHint')}
                  value={draft()}
                  onInput={(e) => setDraft(e.currentTarget.value)}
                  onKeyDown={handleKeyDown}
                />

                <div class="compose-actions">
                  <button
                    type="button"
                    class="compose-icon-btn"
                    onClick={() => setEmojiPickerOpen((prev) => !prev)}
                    title={t('chat.insertEmoji')}
                  >
                    <span class="material-symbols-outlined compose-icon">emoji_emotions</span>
                  </button>

                  <Show when={emojiPickerOpen()}>
                    <div class="emoji-picker">
                      <For each={QUICK_EMOJIS}>
                        {(emoji) => (
                          <button type="button" class="emoji-picker__item" onClick={() => insertEmoji(emoji)}>
                            {emoji}
                          </button>
                        )}
                      </For>
                    </div>
                  </Show>

                  <button
                    class="compose-send"
                    onClick={handleSend}
                    disabled={!draft().trim() || sendMsg.isPending}
                  >
                    {t('chat.send')}
                    <span class="material-symbols-outlined" style={{ 'font-size': '14px' }}>send</span>
                  </button>
                </div>
              </div>
            </Show>
          </div>
        </div>

        {/* Delete user/group confirmation modal */}
        <Show when={deleteModalOpen()}>
          <div class="chat-delete-overlay" onClick={() => setDeleteModalOpen(false)}>
            <div class="chat-delete-modal" onClick={(e) => e.stopPropagation()}>
              <div class="chat-delete-modal__header">
                <span class="material-symbols-outlined chat-delete-modal__icon">
                  {selectedConv()?.isGroup ? 'group_remove' : 'person_remove'}
                </span>
                <h3 class="chat-delete-modal__title">
                  {selectedConv()?.isGroup ? t('chat.deleteGroup.title') : t('chat.deleteUser.title')}
                </h3>
              </div>
              <p class="chat-delete-modal__desc">
                {selectedConv()?.isGroup ? t('chat.deleteGroup.description') : t('chat.deleteUser.description')}
              </p>
              <ul class="chat-delete-modal__list">
                <li>{t('chat.deleteUser.item.messages')}</li>
                <Show when={!selectedConv()?.isGroup}>
                  <li>{t('chat.deleteUser.item.conversations')}</li>
                  <li>{t('chat.deleteUser.item.permissions')}</li>
                  <li>{t('chat.deleteUser.item.account')}</li>
                </Show>
                <Show when={selectedConv()?.isGroup}>
                  <li>{t('chat.deleteGroup.item.conversations')}</li>
                  <li>{t('chat.deleteGroup.item.members')}</li>
                </Show>
              </ul>
              <p class="chat-delete-modal__confirm-label">
                {selectedConv()?.isGroup ? t('chat.deleteGroup.confirmLabel') : t('chat.deleteUser.confirmLabel')}
                <strong> {selectedConv() ? (selectedConv()!.isGroup && selectedConv()!.groupName ? selectedConv()!.groupName : selectedConv()!.participantName) : ''}</strong>
              </p>
              <input
                class="chat-delete-modal__input"
                type="text"
                placeholder={selectedConv() ? (selectedConv()!.isGroup && selectedConv()!.groupName ? selectedConv()!.groupName : selectedConv()!.participantName) : ''}
                value={confirmName()}
                onInput={(e) => setConfirmName(e.currentTarget.value)}
              />
              <div class="chat-delete-modal__actions">
                <button class="btn-modal-cancel" onClick={() => setDeleteModalOpen(false)}>
                  {t('chat.deleteUser.cancel')}
                </button>
                <button
                  class="btn-modal-confirm"
                  disabled={confirmName() !== (selectedConv() ? (selectedConv()!.isGroup && selectedConv()!.groupName ? selectedConv()!.groupName : selectedConv()!.participantName) : '') || (selectedConv()?.isGroup ? deleteGroup.isPending : deleteUser.isPending)}
                  onClick={() => {
                    if (selectedConv()?.isGroup) {
                      deleteGroup.mutate({ conversationId: selectedId() });
                    } else {
                      deleteUser.mutate({ conversationId: selectedId() });
                    }
                  }}
                >
                  <span class="material-symbols-outlined">delete_forever</span>
                  {t('chat.deleteUser.confirm')}
                </button>
              </div>
            </div>
          </div>
        </Show>
      </Show>
    </AppShell>
  );
};

export default ChatView;
