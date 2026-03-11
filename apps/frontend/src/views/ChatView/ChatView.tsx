// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * ChatView — the Chat tab.
 *
 * Left panel: conversation list (280px).
 * Right panel: message thread + compose input.
 * Message list with keyset pagination (load older messages on scroll up).
 * Scroll is anchored to the bottom; new messages auto-scroll unless user has scrolled up.
 */

import type { Component } from 'solid-js';
import { createSignal, For, Show, Switch, Match, Suspense, createEffect, batch } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useConversations, useSubscriptions, useConfig } from '@openlobster/ui/hooks';
import { SEND_MESSAGE_MUTATION, DELETE_USER_MUTATION } from '@openlobster/ui/graphql/mutations';
import { MESSAGES_QUERY } from '@openlobster/ui/graphql/queries';
import type { Message } from '@openlobster/ui/types';
import { renderMarkdown } from '../../lib/markdown';
import { t } from '../../App';
import { client, GRAPHQL_ENDPOINT } from '../../graphql/client';
import AppShell from '../../components/AppShell/AppShell';
import './ChatView.css';

const PAGE_SIZE = 50;


const QUICK_EMOJIS = ['😀', '😂', '🔥', '✅', '🙏', '👍', '🎉', '🤖'];

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ── Message thread ────────────────────────────────────────────────────────────

interface MessageThreadProps {
  conversationId: string;
  onNewMessageCount: (n: number) => void;
  participantName?: string;
}

// Number of messages kept in the DOM at once. Enough to fill several screens
// without stressing the browser. The window slides as the user scrolls.
const RENDER_WINDOW = 120;

const MessageThread: Component<MessageThreadProps> = (props) => {
  const queryClient = useQueryClient();

  // Full accumulated list (oldest first). Only a slice is rendered.
  const [messages, setMessages] = createSignal<Message[]>([]);
  const [oldestCursor, setOldestCursor] = createSignal<string | undefined>(undefined);
  const [hasMore, setHasMore] = createSignal(true);
  const [loadingMore, setLoadingMore] = createSignal(false);

  // Index into messages() of the first item currently rendered.
  const [windowStart, setWindowStart] = createSignal(0);
  const visibleMessages = () => {
    const all = messages();
    return all.slice(windowStart(), windowStart() + RENDER_WINDOW);
  };

  let scrollEl: HTMLDivElement | null = null;
  let userScrolledUp = false;

  const config = useConfig(client);

  async function fetchPage(before?: string): Promise<Message[]> {
    const data = await client.request<{ messages: Message[] }>(MESSAGES_QUERY, {
      conversationId: props.conversationId,
      before: before ?? null,
      limit: PAGE_SIZE,
    });
    return data.messages ?? [];
  }

  function scrollToBottom() {
    if (scrollEl) scrollEl.scrollTop = scrollEl.scrollHeight;
  }

  // Anchor window to the last RENDER_WINDOW messages and scroll to bottom.
  function anchorToBottom() {
    const len = messages().length;
    setWindowStart(Math.max(0, len - RENDER_WINDOW));
    requestAnimationFrame(scrollToBottom);
  }

  // Initial load when conversationId changes
  createEffect(() => {
    const cid = props.conversationId;
    if (!cid) return;

    batch(() => {
      setMessages([]);
      setWindowStart(0);
      setOldestCursor(undefined);
      setHasMore(true);
      userScrolledUp = false;
    });

    fetchPage().then((page) => {
      setMessages(page);
      props.onNewMessageCount(page.length);
      if (page.length < PAGE_SIZE) setHasMore(false);
      if (page.length > 0) setOldestCursor(page[0].createdAt);
      anchorToBottom();
    });

    queryClient.setQueryData(['messages-append', cid], null);
  });

  async function loadOlder() {
    if (loadingMore() || !hasMore() || !oldestCursor()) return;
    setLoadingMore(true);
    const prevScrollHeight = scrollEl?.scrollHeight ?? 0;
    const page = await fetchPage(oldestCursor());
    if (page.length === 0) {
      setHasMore(false);
    } else {
      setMessages((prev) => {
        const ids = new Set(prev.map((m) => m.id));
        const fresh = page.filter((m) => !ids.has(m.id));
        if (fresh.length === 0) {
          // No new messages were returned (possible duplicate cursor) — stop
          // loading older to avoid repeated empty prepends that break the window.
          setHasMore(false);
          return prev;
        }
        // Update oldest cursor to the oldest of the fresh results.
        setOldestCursor(fresh[0].createdAt);
        // Prepend fresh messages and shift the window by the number of
        // actually prepended messages to preserve visible content.
        const newMsgs = [...fresh, ...prev];
        // Shift window back by fresh.length and then restore scroll position.
        setWindowStart((s) => s + fresh.length);
        requestAnimationFrame(() => {
          if (scrollEl) scrollEl.scrollTop = scrollEl.scrollHeight - prevScrollHeight;
        });
        return newMsgs;
      });
      if (page.length < PAGE_SIZE) setHasMore(false);
    }
    setLoadingMore(false);
  }

  function onScroll() {
    if (!scrollEl) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollEl;
    userScrolledUp = scrollHeight - scrollTop - clientHeight > 60;

    // Near top of rendered window — load older from server and slide window back.
    if (scrollTop < 120) void loadOlder();

    // Near bottom of rendered window — slide window forward so newer messages
    // in the full list stay visible.
    if (!userScrolledUp) {
      const len = messages().length;
      const needed = Math.max(0, len - RENDER_WINDOW);
      if (windowStart() < needed) setWindowStart(needed);
    }
  }

  // Expose append via query cache for WS handler
  createEffect(() => {
    const cid = props.conversationId;
    queryClient.setQueryData(['messages-append', cid], {
      append: (msg: Message) => {
        setMessages((prev) => {
          if (prev.some((m) => m.id === msg.id)) return prev;
          return [...prev, msg];
        });
        props.onNewMessageCount(messages().length + 1);
        if (!userScrolledUp) {
          anchorToBottom();
        }
      },
    });
  });

  function renderMessage(msg: Message, globalIndex: number) {
    const allMsgs = messages();
    const prevMsg = globalIndex > 0 ? allMsgs[globalIndex - 1] : undefined;
    const showMeta = !prevMsg || prevMsg.role !== msg.role;
    const senderLabel = () => {
      if (msg.role === 'tool') return t('chat.roleTool');
      if (msg.role === 'assistant' || msg.role === 'agent')
        return config.data?.agent?.name ?? config.data?.agentName ?? 'OpenLobster';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const m: any = msg;
      const perMsgName = m?.senderName ?? m?.sender?.name ?? m?.authorName ?? m?.from;
      if (perMsgName) return perMsgName;
      return props.participantName ? props.participantName : 'USER_' + msg.conversationId.slice(-4).toUpperCase();
    };

    return (
      <div
        class="msg"
        classList={{
          'msg--agent': msg.role === 'assistant' || msg.role === 'agent',
          'msg--user': msg.role === 'user',
          'msg--system': msg.role === 'system',
          'msg--tool': msg.role === 'tool',
        }}
      >
        <Show when={showMeta}>
          <div class="msg__meta">
            <span class="msg__sender">{senderLabel()}</span>
            <span class="msg__time">
              {new Date(msg.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
            </span>
          </div>
        </Show>

        {/* If attachments exist, render them first and use message content as caption below */}
        <Show when={(msg.attachments ?? []).length > 0}>
          <div class="msg__attachments">
            <For each={msg.attachments}>
              {(att) => (
                <div class="msg__attachment-file">
                  <span class="material-symbols-outlined">attach_file</span>
                  <span class="msg__attachment-name">{att.filename ?? att.mimeType ?? att.type}</span>
                </div>
              )}
            </For>
            <Show when={msg.content}>
              <div class="msg__attachment-caption" innerHTML={renderMarkdown(msg.content)} />
            </Show>
          </div>
        </Show>

        {/* If no attachments, render body normally */}
        <Show when={(msg.attachments ?? []).length === 0}>
          <div class="msg__body" innerHTML={renderMarkdown(msg.content)} />
        </Show>
      </div>
    );
  }

  return (
    <div
      class="chat-thread__messages"
      ref={(el) => {
        scrollEl = el;
        if (el && (import.meta.env.MODE === 'test' || process.env.NODE_ENV === 'test')) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          (el as any).__test_loadOlder = loadOlder;
        }
      }}
      onScroll={onScroll}
    >
      <Show when={hasMore()}>
        <div class="chat-thread__load-more">
          <Show when={loadingMore()} fallback={<span />}>
            <span class="chat-thread__loading-indicator">{t('chat.loadingMore')}</span>
          </Show>
        </div>
      </Show>

      <For each={visibleMessages()}>
        {(msg, localIndex) => renderMessage(msg, windowStart() + localIndex())}
      </For>
    </div>
  );
};

// ── Main ChatView ─────────────────────────────────────────────────────────────

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
    onMessageSent: (data: any) => {
      const payload = typeof data === 'string' ? JSON.parse(data) : data;
      if (String(payload.ChannelType || '').toLowerCase() === 'loopback') return;
      const conversationId = payload.ChannelID;
      if (!conversationId) return;

      const newMessage: Message = {
        id: payload.MessageID,
        conversationId,
        role: payload.Role || 'user',
        content: payload.Content || '',
        createdAt: payload.Timestamp || new Date().toISOString(),
        attachments: Array.isArray(payload.Attachments) ? payload.Attachments.map((a: any) => ({
          type: a.Type || a.type || '',
          filename: a.Filename || a.filename || undefined,
          mimeType: a.MIMEType || a.mimeType || a.mime_type || undefined,
        })) : undefined,
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
                            ? new Date(conv.lastMessageAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
                            : ''}
                        </div>
                      </div>
                    </button>
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
                  title={t('chat.deleteUser.button')}
                  onClick={() => { setConfirmName(''); setDeleteModalOpen(true); }}
                >
                  <span class="material-symbols-outlined">person_remove</span>
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

        {/* Delete user confirmation modal */}
        <Show when={deleteModalOpen()}>
          <div class="chat-delete-overlay" onClick={() => setDeleteModalOpen(false)}>
            <div class="chat-delete-modal" onClick={(e) => e.stopPropagation()}>
              <div class="chat-delete-modal__header">
                <span class="material-symbols-outlined chat-delete-modal__icon">person_remove</span>
                <h3 class="chat-delete-modal__title">{t('chat.deleteUser.title')}</h3>
              </div>
              <p class="chat-delete-modal__desc">{t('chat.deleteUser.description')}</p>
              <ul class="chat-delete-modal__list">
                <li>{t('chat.deleteUser.item.messages')}</li>
                <li>{t('chat.deleteUser.item.conversations')}</li>
                <li>{t('chat.deleteUser.item.permissions')}</li>
                <li>{t('chat.deleteUser.item.account')}</li>
              </ul>
              <p class="chat-delete-modal__confirm-label">
                {t('chat.deleteUser.confirmLabel')}
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
                  disabled={confirmName() !== (selectedConv() ? (selectedConv()!.isGroup && selectedConv()!.groupName ? selectedConv()!.groupName : selectedConv()!.participantName) : '') || deleteUser.isPending}
                  onClick={() => deleteUser.mutate({ conversationId: selectedId() })}
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
