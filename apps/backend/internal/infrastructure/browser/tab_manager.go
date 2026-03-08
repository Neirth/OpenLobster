package browser

import (
	"context"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

const TabIdleTimeout = 5 * time.Minute

type TabEntry struct {
	Page       ports.BrowserPage
	LastUsedAt time.Time
	SessionID  string
}

type BrowserTabManager struct {
	tabs    map[string]*TabEntry
	mu      sync.Mutex
	browser ports.BrowserPort
}

func NewBrowserTabManager(browser ports.BrowserPort) *BrowserTabManager {
	return &BrowserTabManager{
		tabs:    make(map[string]*TabEntry),
		browser: browser,
	}
}

func (m *BrowserTabManager) GetOrCreate(ctx context.Context, sessionID string) (ports.BrowserPage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, ok := m.tabs[sessionID]; ok {
		entry.LastUsedAt = time.Now()
		return entry.Page, nil
	}

	page, err := m.browser.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	m.tabs[sessionID] = &TabEntry{
		Page:       page,
		LastUsedAt: time.Now(),
		SessionID:  sessionID,
	}
	return page, nil
}

func (m *BrowserTabManager) Touch(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.tabs[sessionID]; ok {
		entry.LastUsedAt = time.Now()
	}
}

func (m *BrowserTabManager) Close(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.tabs[sessionID]
	if !ok {
		return nil
	}
	delete(m.tabs, sessionID)
	return entry.Page.Close()
}

func (m *BrowserTabManager) RunReaper(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reap()
		}
	}
}

func (m *BrowserTabManager) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for sessionID, entry := range m.tabs {
		if now.Sub(entry.LastUsedAt) >= TabIdleTimeout {
			entry.Page.Close()
			delete(m.tabs, sessionID)
		}
	}
}

func (m *BrowserTabManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var lastErr error
	for _, entry := range m.tabs {
		if err := entry.Page.Close(); err != nil {
			lastErr = err
		}
	}
	m.tabs = make(map[string]*TabEntry)
	return lastErr
}
