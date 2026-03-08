// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * ChatView — the Chat tab.
 *
 * Left panel: conversation list (280px).
 * Right panel: message thread + compose input.
 * Virtualised message list with keyset pagination (load older messages on scroll up).
 * Scroll is anchored to the bottom; new messages auto-scroll unless user has scrolled up.
 */

import type { Component } from 'solid-js';
import { createSignal, For, Show, Suspense, createEffect, batch, onMount, onCleanup } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useConversations, useSubscriptions, useConfig } from '@openlobster/ui/hooks';
import { SEND_MESSAGE_MUTATION, DELETE_USER_MUTATION } from '@openlobster/ui/graphql/mutations';
import { MESSAGES_QUERY } from '@openlobster/ui/graphql/queries';
import type { Message } from '@openlobster/ui/types';
import { renderMarkdown } from '../../lib/markdown';
import { t } from '../../App';
import { client } from '../../graphql/client';
import AppShell from '../../components/AppShell/AppShell';
import './ChatView.css';

const PAGE_SIZE = 50;


const QUICK_EMOJIS = ['😀', '😂', '🔥', '✅', '🙏', '👍', '🎉', '🤖'];

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ── Message thread with virtualisation ───────────────────────────────────────

interface MessageThreadProps {
  conversationId: string;
  onNewMessageCount: (n: number) => void;
  participantName?: string;
}

const MessageThread: Component<MessageThreadProps> = (props) => {
  const queryClient = useQueryClient();

  // Accumulated messages across pages (oldest first)
  const [messages, setMessages] = createSignal<Message[]>([]);
  const [oldestCursor, setOldestCursor] = createSignal<string | undefined>(undefined);
  const [hasMore, setHasMore] = createSignal(true);
  const [loadingMore, setLoadingMore] = createSignal(false);

  // Reactive scroll element — virtualizer reads this via the accessor
  const [scrollEl, setScrollEl] = createSignal<HTMLDivElement | null>(null);
  let userScrolledUp = false;

  async function fetchPage(before?: string): Promise<Message[]> {
    const data = await client.request<{ messages: Message[] }>(MESSAGES_QUERY, {
      conversationId: props.conversationId,
      before: before ?? null,
      limit: PAGE_SIZE,
    });
    return data.messages ?? [];
  }

  // Initial load when conversationId changes
  createEffect(() => {
    const cid = props.conversationId;
    if (!cid) return;

    batch(() => {
      setMessages([]);
      setOldestCursor(undefined);
      setHasMore(true);
      userScrolledUp = false;
    });

    fetchPage().then((page) => {
      setMessages(page);
      props.onNewMessageCount(page.length);
      if (page.length < PAGE_SIZE) setHasMore(false);
      if (page.length > 0) setOldestCursor(page[0].createdAt);
      requestAnimationFrame(() => {
        const el = scrollEl();
        if (el) el.scrollTop = el.scrollHeight;
      });
    });

    queryClient.setQueryData(['messages-append', cid], null);
  });

  async function loadOlder() {
    if (loadingMore() || !hasMore() || !oldestCursor()) return;
    setLoadingMore(true);
    const el = scrollEl();
    const prevScrollHeight = el?.scrollHeight ?? 0;
    const page = await fetchPage(oldestCursor());
    if (page.length === 0 || page.length < PAGE_SIZE) setHasMore(false);
    if (page.length > 0) {
      setOldestCursor(page[0].createdAt);
      setMessages((prev) => {
        const ids = new Set(prev.map((m) => m.id));
        const fresh = page.filter((m) => !ids.has(m.id));
        return [...fresh, ...prev];
      });
      requestAnimationFrame(() => {
        const el = scrollEl();
        if (el) el.scrollTop = el.scrollHeight - prevScrollHeight;
      });
    }
    setLoadingMore(false);
  }

  function onScroll() {
    const el = scrollEl();
    if (!el) return;
    const { scrollTop, scrollHeight, clientHeight } = el;
    userScrolledUp = scrollHeight - scrollTop - clientHeight > 60;
    if (scrollTop < 120) void loadOlder();
    // update our window start based on scroll position
    const approxIndex = Math.floor(scrollTop / estimatedHeight) - 3; // overscan above
    const newStart = Math.max(0, approxIndex);
    setStartIndex(newStart);
  }

  // Manual windowed rendering (replaces virtualizer): estimate a fixed height
  // per message and render only the visible slice to keep DOM count low.
  const estimatedHeight = 64; // px per message (reduced to tighten gaps)
  const [startIndex, setStartIndex] = createSignal(0);
  const visibleCount = () => Math.ceil(((scrollEl()?.clientHeight) ?? 600) / estimatedHeight) + 6; // overscan
  const endIndex = () => Math.min(messages().length, startIndex() + visibleCount());

  // Auto-scroll to bottom on new messages (only if not scrolled up)
  createEffect(() => {
    const len = messages().length;
    if (len === 0) return;
    if (!userScrolledUp) {
      requestAnimationFrame(() => {
        const el = scrollEl();
        if (el) el.scrollTop = el.scrollHeight;
      });
    }
  });

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
        // if user is at bottom, advance window to include the new message
        requestAnimationFrame(() => {
          if (!userScrolledUp) {
            const len = messages().length;
            const vc = visibleCount();
            setStartIndex(Math.max(0, len - vc));
            const el = scrollEl();
            if (el) el.scrollTop = el.scrollHeight;
          }
        });
      },
    });
  });

  // Heights map and measurement tick to drive recalculation of offsets
  const heights: Record<string, number> = {};
  const [measureTick, setMeasureTick] = createSignal(0);
  let resizeObserver: ResizeObserver | null = null;

  function measureElementRef(el: HTMLElement | null, index: number) {
    if (!el) return;
    const msg = messages()[index];
    if (!msg) return;
    const id = msg.id;
    const h = el.offsetHeight;
    // attach dataset for ResizeObserver to identify the message
    try {
      (el as HTMLElement).dataset.msgId = id;
    } catch (_) {
      // ignore non-writable dataset in some test environments
    }
    if (heights[id] !== h) {
      heights[id] = h;
      setMeasureTick((v) => v + 1);
    }
    // ensure the element is observed for future resizes
    if (resizeObserver) resizeObserver.observe(el);
  }

  function offsetForIndex(idx: number) {
    let off = 0;
    for (let i = 0; i < idx; i++) {
      const m = messages()[i];
      if (!m) continue;
      const h = heights[m.id];
      off += h && h > 0 ? h : estimatedHeight;
    }
    return off;
  }

  function totalContentHeight() {
    let total = 0;
    const all = messages();
    for (let i = 0; i < all.length; i++) {
      const h = heights[all[i].id];
      total += h && h > 0 ? h : estimatedHeight;
    }
    // In test environments offsetHeight may be 0; ensure a sensible fallback
    if (total === 0 && messages().length > 0) return messages().length * estimatedHeight;
    return total;
  }

  const items = () => {
    measureTick(); // depend on measurement updates
    const s = startIndex();
    const e = endIndex();
    const out: { index: number; start: number }[] = [];
    for (let i = s; i < e; i++) out.push({ index: i, start: offsetForIndex(i) });
    return out;
  };
  // config (agent display name)
  const config = useConfig(client);
  
  onMount(() => {
    // Observe size changes on measured message elements (images, async content)
    resizeObserver = new ResizeObserver((entries) => {
      let changed = false;
      for (const entry of entries) {
        const target = entry.target as HTMLElement;
        const id = target.dataset.msgId;
        if (!id) continue;
        const h = Math.round(entry.contentRect.height || target.offsetHeight || 0);
        if (heights[id] !== h) {
          heights[id] = h;
          changed = true;
        }
      }
      if (changed) setMeasureTick((v) => v + 1);
    });
  });

  onCleanup(() => {
    if (resizeObserver) {
      try {
        resizeObserver.disconnect();
      } catch (_) {
        // ignore
      }
      resizeObserver = null;
    }
  });

  // Ensure windowed rendering updates when messages change: anchor to bottom
  // and set the visible window to the last messages.
  createEffect(() => {
    const len = messages().length;
    if (len === 0) return;
    requestAnimationFrame(() => {
      const el = scrollEl();
      const vc = visibleCount();
      const newEnd = len;
      const newStart = Math.max(0, newEnd - vc);
      setStartIndex(newStart);
      if (el) el.scrollTop = el.scrollHeight;
    });
  });

  return (
    <div
      class="chat-thread__messages"
      ref={(el) => {
        setScrollEl(el);
        if (el && (import.meta.env.MODE === 'test' || process.env.NODE_ENV === 'test')) {
          // expose test hook to allow triggering loadOlder from unit tests (JSDOM)
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

      <div
        style={{
          height: `${totalContentHeight()}px`,
          position: 'relative',
        }}
      >
        {/* spacer to push the visible window into view; children use normal flow */}
        <div style={{ paddingTop: `${offsetForIndex(startIndex())}px` }}>
          <For each={items()}>
            {(it) => {
              const i = (it as { index: number; start: number }).index;
              const msg = () => messages()[i];
              const prevMsg = () => (i > 0 ? messages()[i - 1] : undefined);
              const showMeta = () => !prevMsg() || prevMsg()!.role !== msg()!.role;
              const senderLabel = () => {
                if (!msg()) return '';
                if (msg()!.role === 'tool') return t('chat.roleTool');
                if (msg()!.role === 'assistant' || msg()!.role === 'agent')
                  return config.data?.agent?.name ?? config.data?.agentName ?? 'OpenLobster';
                // Prefer per-message sender metadata when available (useful for group chats)
                // Support common fields that might appear from backend: senderName, sender.name, authorName, from
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                const m: any = msg();
                const perMsgName = m?.senderName ?? m?.sender?.name ?? m?.authorName ?? m?.from;
                if (perMsgName) return perMsgName;
                return props.participantName ? props.participantName : 'USER_' + msg()!.conversationId.slice(-4).toUpperCase();
              };

              return (
                <Show when={msg()} keyed>
                  <div
                    class="msg"
                    ref={(el) => measureElementRef(el as HTMLElement, i)}
                    classList={{
                      'msg--agent': msg().role === 'assistant' || msg().role === 'agent',
                      'msg--user': msg().role === 'user',
                      'msg--system': msg().role === 'system',
                      'msg--tool': msg().role === 'tool',
                    }}
                  >
                    <Show when={showMeta()}>
                      <div class="msg__meta">
                        <span class="msg__sender">{senderLabel()}</span>
                        <span class="msg__time">
                          {new Date(msg().createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                        </span>
                      </div>
                    </Show>
                    <div class="msg__body" innerHTML={renderMarkdown(msg().content)} />
                  </div>
                </Show>
              );
            }}
          </For>
        </div>
      </div>
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
  let fileInputRef: HTMLInputElement | undefined;

  const conversations = useConversations(client);
  const queryClient = useQueryClient();

  const graphqlUrl = import.meta.env.VITE_GRAPHQL_ENDPOINT ?? 'http://127.0.0.1:8080/graphql';
  const wsUrl = graphqlUrl.replace(/\/graphql\/?$/, '/ws').replace(/^http/, 'ws');

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
    onSuccess: () => setDraft(''),
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

  function handleAttachClick() {
    fileInputRef?.click();
  }

  function handleFileSelected(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    const markdownAttachment = `\n📎 **${t('chat.attachmentLabel')}:** \`${file.name}\`\n- ${t('chat.attachmentType')}: \`${file.type || t('chat.unknown')}\`\n- ${t('chat.attachmentSize')}: \`${formatFileSize(file.size)}\`\n`;
    setDraft((prev) => (prev ? `${prev}\n${markdownAttachment}` : markdownAttachment.trimStart()));
    input.value = '';
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
                <textarea
                  class="compose-input"
                  placeholder={t('chat.typeMessageHint')}
                  value={draft()}
                  onInput={(e) => setDraft(e.currentTarget.value)}
                  onKeyDown={handleKeyDown}
                />

                <input
                  ref={fileInputRef}
                  type="file"
                  style={{ display: 'none' }}
                  onChange={handleFileSelected}
                />

                <div class="compose-actions">
                  <button type="button" class="compose-icon-btn" onClick={handleAttachClick} title={t('chat.attachFile')}>
                    <span class="material-symbols-outlined compose-icon">attach_file</span>
                  </button>
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
                        if (!msg()) return '';
                        if (msg()!.role === 'tool') return t('chat.roleTool');
                        if (msg()!.role === 'assistant' || msg()!.role === 'agent')
                          return config.data?.agent?.name ?? config.data?.agentName ?? 'OpenLobster';
                        // user/system: prefer participantName prop when available
                        return props.participantName ? props.participantName : 'USER_' + msg()!.conversationId.slice(-4).toUpperCase();
                      };
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
