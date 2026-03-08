// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from 'solid-js';
import { For, Show, createResource, createSignal, createMemo } from 'solid-js';
import { t } from '../../App';
import './MarketplaceModal.css';

export interface MarketplaceServer {
  id: string;
  name: string;
  company: string;
  description: string;
  url: string;
  homepage?: string;
  transport?: string;
  category?: string;
  /** When true, initiates OAuth flow directly instead of connectMcp. */
  oauth?: boolean;
}

export interface MarketplaceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (server: MarketplaceServer) => void;
}

const fetchMarketplace = async (): Promise<MarketplaceServer[]> => {
  const res = await fetch('/marketplace.json');
  if (!res.ok) throw new Error('Failed to load marketplace');
  return res.json() as Promise<MarketplaceServer[]>;
};

const faviconUrl = (url: string, homepage?: string): string => {
  try {
    const { hostname } = new URL(homepage ?? url);
    const parts = hostname.split('.');
    const rootDomain = parts.length > 2 ? parts.slice(-2).join('.') : hostname;
    return `https://www.google.com/s2/favicons?domain=${rootDomain}&sz=32`;
  } catch {
    return '';
  }
};

const MarketplaceModal: Component<MarketplaceModalProps> = (props) => {
  const [search, setSearch] = createSignal('');
  const [selected, setSelected] = createSignal<MarketplaceServer | null>(null);

  const [servers] = createResource(() => props.isOpen, fetchMarketplace);

  const filtered = createMemo(() => {
    const q = search().toLowerCase();
    const data = servers() ?? [];
    if (!q) return data;
    return data.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.company.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        (s.category ?? '').toLowerCase().includes(q),
    );
  });

  const handleOverlayClick = (e: MouseEvent) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const handleClose = () => {
    setSearch('');
    setSelected(null);
    props.onClose();
  };

  const handleAdd = (server: MarketplaceServer) => {
    props.onAdd(server);
    handleClose();
  };

  return (
    <Show when={props.isOpen}>
      <div class="marketplace-overlay" onClick={handleOverlayClick}>
        <div class="marketplace-box">
          {/* Header */}
          <div class="marketplace-header">
            <Show
              when={selected()}
              fallback={
                <div class="marketplace-header__left">
                  <span class="material-symbols-outlined marketplace-header-icon">storefront</span>
                  <div>
                    <h3 class="marketplace-title">{t('marketplace.title')}</h3>
                    <p class="marketplace-subtitle">{t('marketplace.subtitle')}</p>
                  </div>
                </div>
              }
            >
              <button class="marketplace-back-btn" onClick={() => setSelected(null)}>
                <span class="material-symbols-outlined">arrow_back</span>
                {t('marketplace.back')}
              </button>
            </Show>

            <button class="marketplace-close-btn" onClick={handleClose} aria-label="Close">
              <span class="material-symbols-outlined">close</span>
            </button>
          </div>

          {/* Search — only visible in list view */}
          <Show when={!selected()}>
            <div class="marketplace-search-wrap">
              <span class="material-symbols-outlined marketplace-search-icon">search</span>
              <input
                class="marketplace-search"
                type="search"
                placeholder={t('marketplace.searchPlaceholder')}
                value={search()}
                onInput={(e) => setSearch(e.currentTarget.value)}
                autocomplete="off"
              />
            </div>
          </Show>

          {/* Body */}
          <div class="marketplace-body">
            <Show when={!selected()}>
              {/* LIST VIEW */}
              <Show when={servers.loading}>
                <div class="marketplace-loading">
                  <span class="material-symbols-outlined marketplace-loading-icon">rotate_right</span>
                  <p>{t('marketplace.loading')}</p>
                </div>
              </Show>

              <Show when={servers.error}>
                <div class="marketplace-loading">
                  <span class="material-symbols-outlined marketplace-loading-icon">error</span>
                  <p>{t('marketplace.error')}</p>
                </div>
              </Show>

              <Show when={!servers.loading && !servers.error && filtered().length === 0}>
                <div class="marketplace-loading">
                  <span class="material-symbols-outlined marketplace-loading-icon">search_off</span>
                  <p>{t('marketplace.noResults')}</p>
                </div>
              </Show>

              <div class="marketplace-grid">
                <For each={filtered()}>
                  {(server) => (
                    <button
                      class="marketplace-card"
                      onClick={() => setSelected(server)}
                    >
                      <div class="marketplace-card__icon-wrap">
                        <img
                          class="marketplace-card__favicon"
                          src={faviconUrl(server.url, server.homepage)}
                          alt=""
                          onError={(e) => {
                            (e.currentTarget as HTMLImageElement).style.display = 'none';
                            const fallback = e.currentTarget.nextElementSibling as HTMLElement | null;
                            if (fallback) fallback.style.display = '';
                          }}
                        />
                        <span
                          class="material-symbols-outlined marketplace-card__fallback-icon"
                          style="display:none"
                        >
                          extension
                        </span>
                      </div>
                      <div class="marketplace-card__body">
                        <span class="marketplace-card__name">{server.name}</span>
                        <div class="marketplace-card__meta">
                          <span class="marketplace-card__company">{server.company}</span>
                          <Show when={server.category}>
                            <span class="marketplace-card__category-dot">·</span>
                            <span class="marketplace-card__category">{server.category}</span>
                          </Show>
                        </div>
                        <p class="marketplace-card__description">{server.description}</p>
                      </div>
                      <span class="material-symbols-outlined marketplace-card__chevron">chevron_right</span>
                    </button>
                  )}
                </For>
              </div>
            </Show>

            <Show when={selected()}>
              {/* DETAIL VIEW */}
              {(() => {
                const server = selected()!;
                return (
                  <div class="marketplace-detail">
                    <div class="marketplace-detail__hero">
                      <div class="marketplace-detail__icon-wrap">
                        <img
                          class="marketplace-detail__favicon"
                          src={faviconUrl(server.url, server.homepage)}
                          alt=""
                          onError={(e) => {
                            (e.currentTarget as HTMLImageElement).style.display = 'none';
                            const fallback = e.currentTarget.nextElementSibling as HTMLElement | null;
                            if (fallback) fallback.style.display = '';
                          }}
                        />
                        <span
                          class="material-symbols-outlined marketplace-detail__fallback-icon"
                          style="display:none"
                        >
                          extension
                        </span>
                      </div>
                      <div class="marketplace-detail__hero-text">
                        <h2 class="marketplace-detail__name">{server.name}</h2>
                        <div class="marketplace-detail__meta">
                          <p class="marketplace-detail__company">{server.company}</p>
                          <Show when={server.category}>
                            <span class="marketplace-detail__category-dot">·</span>
                            <span class="marketplace-detail__category">{server.category}</span>
                          </Show>
                        </div>
                      </div>
                    </div>

                    <p class="marketplace-detail__description">{server.description}</p>

                    <div class="marketplace-detail__field">
                      <span class="marketplace-detail__field-label">{t('marketplace.endpoint')}</span>
                      <code class="marketplace-detail__field-value">{server.url}</code>
                    </div>

                    <div class="marketplace-detail__field">
                      <span class="marketplace-detail__field-label">{t('marketplace.transport')}</span>
                      <code class="marketplace-detail__field-value">{server.transport ?? 'http'}</code>
                    </div>

                    <div class="marketplace-detail__actions">
                      <button
                        class="marketplace-connect-btn"
                        onClick={() => handleAdd(server)}
                      >
                        <span class="material-symbols-outlined">add_circle</span>
                        {t('marketplace.connect')}
                      </button>
                    </div>
                  </div>
                );
              })()}
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default MarketplaceModal;
