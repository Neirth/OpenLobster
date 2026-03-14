// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from 'solid-js';
import { For } from 'solid-js';
import './SkeletonMessages.css';

const SkeletonMessages: Component = () => (
  <div class="chat-thread__skeleton-list" aria-hidden="true">
    <For each={[1, 2, 3, 4]}>
      {() => (
        <div class="msg-skeleton">
          <div class="msg-skeleton__meta">
            <div class="msg-skeleton__avatar msg-skeleton__block" />
            <div class="msg-skeleton__name msg-skeleton__block" />
          </div>
          <div class="msg-skeleton__line msg-skeleton__line--wide msg-skeleton__block" />
          <div class="msg-skeleton__line msg-skeleton__block" />
        </div>
      )}
    </For>
  </div>
);

export default SkeletonMessages;

