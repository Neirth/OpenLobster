// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * MessageThread — virtualised, paginated message list for a single conversation.
 *
 * Renders only a sliding window of RENDER_WINDOW messages at a time.
 * Older messages are fetched on scroll-to-top; new messages are appended
 * via a query-cache slot written by the WebSocket handler in ChatView.
 */

import type { Component } from 'solid-js';
import { createSignal, For, Show, createEffect, batch, createMemo } from 'solid-js';
import { useQueryClient } from '@tanstack/solid-query';
import { useConfig } from '@openlobster/ui/hooks';
import { MESSAGES_QUERY } from '@openlobster/ui/graphql/queries';
import type { Message } from '@openlobster/ui/types';
import { renderMarkdown } from '../../lib/markdown';
import { t } from '../../App';
import { client } from '../../graphql/client';
import SkeletonMessages from '../SkeletonMessages';
import { formatChatTime } from '../../utils/formatChatTime';
import './MessageThread.css';

const PAGE_SIZE = 50;
const RENDER_WINDOW = 120;
const TOOL_OUTPUT_MAX_CHARS = 2000;

export interface MessageThreadProps {
  conversationId: string;
  onNewMessageCount: (n: number) => void;
  participantName?: string;
}

const MessageThread: Component<MessageThreadProps> = (props) => {
  const queryClient = useQueryClient();

  // Full accumulated list (oldest first). Only a slice is rendered.
  const [messages, setMessages] = createSignal<Message[]>([]);
  const [oldestCursor, setOldestCursor] = createSignal<string | undefined>(undefined);
  const [hasMore, setHasMore] = createSignal(true);
  const [loadingMore, setLoadingMore] = createSignal(false);
  const [initialLoading, setInitialLoading] = createSignal(false);

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

    setInitialLoading(true);

    batch(() => {
      setMessages([]);
      setWindowStart(0);
      setOldestCursor(undefined);
      setHasMore(true);
      userScrolledUp = false;
    });

    (async () => {
      try {
        const page = await fetchPage();
        // Si mientras cargábamos se cambió de conversación, ignoramos este resultado.
        if (props.conversationId !== cid) return;
        setMessages(page);
        props.onNewMessageCount(page.length);
        if (page.length < PAGE_SIZE) setHasMore(false);
        if (page.length > 0) setOldestCursor(page[0].createdAt);
        anchorToBottom();
      } finally {
        if (props.conversationId === cid) setInitialLoading(false);
      }
    })();

    queryClient.setQueryData(['messages-append', cid], null);
  });

  async function loadOlder() {
    const cidAtStart = props.conversationId;
    if (loadingMore() || !hasMore() || !oldestCursor()) return;
    setLoadingMore(true);
    const prevScrollHeight = scrollEl?.scrollHeight ?? 0;
    const page = await fetchPage(oldestCursor());
    // Si ha cambiado la conversación mientras se cargaba la página, no toques el estado.
    if (props.conversationId !== cidAtStart) {
      setLoadingMore(false);
      return;
    }
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
    const isToolMessage = msg.role === 'tool';
    const rawContent = msg.content ?? '';
    const displayContent =
      isToolMessage && rawContent.length > TOOL_OUTPUT_MAX_CHARS
        ? `${rawContent.slice(0, TOOL_OUTPUT_MAX_CHARS)}\n\n${t('chat.contentTruncated')}`
        : rawContent;
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
              {formatChatTime(msg.createdAt, true)}
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
              {/* eslint-disable-next-line solid/no-innerhtml */}
              <div class="msg__attachment-caption" innerHTML={renderMarkdown(displayContent)} />
            </Show>
          </div>
        </Show>

        {/* If no attachments, render body normally */}
        <Show when={(msg.attachments ?? []).length === 0}>
          {/* eslint-disable-next-line solid/no-innerhtml */}
          <div class="msg__body" innerHTML={renderMarkdown(displayContent)} />
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

      <Show when={initialLoading() && messages().length === 0}>
        <SkeletonMessages />
      </Show>

      <For each={visibleMessages()}>
        {(msg, localIndex) => {
          const node = createMemo(() => renderMessage(msg, windowStart() + localIndex()));
          return <>{node()}</>;
        }}
      </For>
    </div>
  );
};

export default MessageThread;
