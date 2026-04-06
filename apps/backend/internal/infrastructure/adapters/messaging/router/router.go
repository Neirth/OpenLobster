// Package router provides the thread-safe channel adapter registry and the
// fan-out messaging router that dispatches outbound messages to whichever
// platform adapter is currently active for a given channel type.
package router

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// Registry is a thread-safe registry of active messaging adapters keyed by
// channel type (e.g. "telegram", "discord"). It is populated at startup and
// updated on hot-reload without restarting the server.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]ports.MessagingPort
}

// New returns an empty Registry ready for use.
func New() *Registry {
	return &Registry{adapters: make(map[string]ports.MessagingPort)}
}

// Set registers (or replaces) the adapter for the given channel type.
func (r *Registry) Set(channelType string, a ports.MessagingPort) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[channelType] = a
}

// Get returns the adapter for the given channel type, or nil if not registered.
func (r *Registry) Get(channelType string) ports.MessagingPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType]
}

// Remove deregisters the adapter for the given channel type.
func (r *Registry) Remove(channelType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, channelType)
}

// Clear removes all registered adapters while keeping the same registry instance.
// This avoids stale pointer issues for components that keep a reference to Registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = make(map[string]ports.MessagingPort)
}

// ListTypes returns all currently registered channel type names.
func (r *Registry) ListTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.adapters))
	for k := range r.adapters {
		out = append(out, k)
	}
	return out
}

// SendTextToChannel delivers a plain-text message to a user identified by
// channelType and channelID. Satisfies dto.MessageSender; silently no-ops
// when the adapter is not active.
func (r *Registry) SendTextToChannel(ctx context.Context, channelType, channelID, text string) error {
	adapter := r.Get(channelType)
	if adapter == nil {
		return nil
	}
	msg := models.NewMessage(channelID, text)
	return adapter.SendMessage(ctx, msg)
}

// Router implements ports.MessagingPort by routing each outbound call to the
// correct channel adapter based on msg.Metadata["channel_type"]. The message
// handler sets that metadata field so replies reach the right platform.
type Router struct {
	reg *Registry
}

// NewRouter wraps reg in a Router.
func NewRouter(reg *Registry) *Router {
	return &Router{reg: reg}
}

func (m *Router) SendTyping(ctx context.Context, channelID string) error {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil
	}
	adapter := m.reg.Get(ct)
	if adapter == nil {
		return nil
	}
	return adapter.SendTyping(ctx, channelID)
}

func (m *Router) SendMessage(ctx context.Context, msg *models.Message) error {
	if msg == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(msg.Content), "no_reply") {
		return nil
	}
	ct := ""
	if msg.Metadata != nil {
		if v, ok := msg.Metadata["channel_type"].(string); ok {
			ct = strings.TrimSpace(strings.ToLower(v))
		}
	}
	if ct == "" {
		err := fmt.Errorf("messaging: cannot route — msg has no channel_type in Metadata (channel_id=%q)", msg.ChannelID)
		log.Print(err)
		return err
	}
	adapter := m.reg.Get(ct)
	if adapter == nil {
		activeTypes := m.reg.ListTypes()
		sort.Strings(activeTypes)
		err := fmt.Errorf("messaging: cannot route — no adapter for channel_type=%q (channel_id=%q, active_adapters=%v)", ct, msg.ChannelID, activeTypes)
		log.Print(err)
		return err
	}
	return adapter.SendMessage(ctx, msg)
}

func (m *Router) SendMedia(ctx context.Context, media *ports.Media) error {
	if media == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(media.Caption), "no_reply") {
		return nil
	}
	ct := media.ChannelType
	if ct == "" {
		return nil
	}
	adapter := m.reg.Get(ct)
	if adapter == nil {
		return nil
	}
	return adapter.SendMedia(ctx, media)
}

// HandleWebhook is not used by the router — platform webhooks dispatch directly
// to each adapter via the webhooks.Handler.
func (m *Router) HandleWebhook(_ context.Context, _ []byte) (*models.Message, error) {
	return nil, nil
}

func (m *Router) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil, nil
	}
	adapter := m.reg.Get(ct)
	if adapter == nil {
		return nil, nil
	}
	return adapter.GetUserInfo(ctx, userID)
}

func (m *Router) React(ctx context.Context, messageID string, emoji string) error {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil
	}
	adapter := m.reg.Get(ct)
	if adapter == nil {
		return nil
	}
	return adapter.React(ctx, messageID, emoji)
}

// GetCapabilities merges capabilities from all active adapters.
func (m *Router) GetCapabilities() ports.ChannelCapabilities {
	merged := ports.ChannelCapabilities{}
	for _, ct := range m.reg.ListTypes() {
		adapter := m.reg.Get(ct)
		if adapter == nil {
			continue
		}
		caps := adapter.GetCapabilities()
		if caps.HasVoiceMessage {
			merged.HasVoiceMessage = true
		}
		if caps.HasCallStream {
			merged.HasCallStream = true
		}
		if caps.HasTextStream {
			merged.HasTextStream = true
		}
		if caps.HasMediaSupport {
			merged.HasMediaSupport = true
		}
	}
	return merged
}

func (m *Router) ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error) {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil, "", nil
	}
	adapter := m.reg.Get(ct)
	if adapter == nil {
		return nil, "", nil
	}
	return adapter.ConvertAudioForPlatform(ctx, audioData, format)
}

// Start is a no-op for the router — each adapter is started individually.
func (m *Router) Start(_ context.Context, _ func(context.Context, *models.Message)) error {
	return nil
}
