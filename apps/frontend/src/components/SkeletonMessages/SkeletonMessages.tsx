// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from 'solid-js';
import './SkeletonMessages.css';

const SkeletonMessages: Component = () => (
  <div class="chat-thread__skeleton-list" aria-hidden="true">

    {/* Agent message — left aligned */}
    <div class="msg-skeleton msg-skeleton--agent">
      <div class="msg-skeleton__meta">
        <div class="msg-skeleton__name msg-skeleton__block" />
        <div class="msg-skeleton__time msg-skeleton__block" />
      </div>
      <div class="msg-skeleton__body msg-skeleton__block">
        <div class="msg-skeleton__line msg-skeleton__line--wide msg-skeleton__block" />
        <div class="msg-skeleton__line msg-skeleton__block" />
        <div class="msg-skeleton__line msg-skeleton__line--mid msg-skeleton__block" />
        <div class="msg-skeleton__line msg-skeleton__line--wide msg-skeleton__block" />
        <div class="msg-skeleton__line msg-skeleton__block" />
      </div>
    </div>

    {/* User message — right aligned */}
    <div class="msg-skeleton msg-skeleton--user">
      <div class="msg-skeleton__meta">
        <div class="msg-skeleton__name msg-skeleton__block" />
        <div class="msg-skeleton__time msg-skeleton__block" />
      </div>
      <div class="msg-skeleton__body msg-skeleton__block">
        <div class="msg-skeleton__line msg-skeleton__line--mid msg-skeleton__block" />
        <div class="msg-skeleton__line msg-skeleton__block" />
      </div>
    </div>

    {/* Agent message — continuation (no meta) */}
    <div class="msg-skeleton msg-skeleton--agent msg-skeleton--continuation">
      <div class="msg-skeleton__body msg-skeleton__block">
        <div class="msg-skeleton__line msg-skeleton__line--wide msg-skeleton__block" />
        <div class="msg-skeleton__line msg-skeleton__line--mid msg-skeleton__block" />
      </div>
    </div>

    {/* User message — right aligned */}
    <div class="msg-skeleton msg-skeleton--user">
      <div class="msg-skeleton__meta">
        <div class="msg-skeleton__name msg-skeleton__block" />
        <div class="msg-skeleton__time msg-skeleton__block" />
      </div>
      <div class="msg-skeleton__body msg-skeleton__block">
        <div class="msg-skeleton__line msg-skeleton__line--wide msg-skeleton__block" />
      </div>
    </div>

  </div>
);

export default SkeletonMessages;
