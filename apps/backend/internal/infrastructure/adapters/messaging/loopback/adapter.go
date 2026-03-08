// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package loopback provides a no-op MessagingPort implementation for the
// virtual loopback channel used by the Scheduler.
//
// The Scheduler dispatches tasks through the standard agentic pipeline, which
// at the end attempts to deliver the LLM response via the configured
// MessagingPort. For loopback executions no external delivery is desired; this
// adapter satisfies the interface contract while performing no I/O.
package loopback

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// Adapter is a no-op implementation of ports.MessagingPort for the loopback
// virtual channel. All methods are safe for concurrent use.
type Adapter struct{}

// New returns a ready-to-use loopback Adapter.
func New() *Adapter { return &Adapter{} }

// SendMessage discards the message — loopback sessions have no external recipient.
func (a *Adapter) SendMessage(_ context.Context, _ *models.Message) error { return nil }

// SendTyping is a no-op for the loopback channel.
func (a *Adapter) SendTyping(_ context.Context, _ string) error { return nil }

// SendMedia is a no-op for the loopback channel.
func (a *Adapter) SendMedia(_ context.Context, _ *ports.Media) error { return nil }

// HandleWebhook always returns nil because the loopback channel never receives
// incoming webhooks from external platforms.
func (a *Adapter) HandleWebhook(_ context.Context, _ []byte) (*models.Message, error) {
	return nil, nil
}

// GetUserInfo returns a minimal placeholder — the loopback channel has no real
// user on the external platform side.
func (a *Adapter) GetUserInfo(_ context.Context, userID string) (*ports.UserInfo, error) {
	return &ports.UserInfo{ID: userID, Username: "loopback", DisplayName: "Loopback"}, nil
}

// React is a no-op for the loopback channel.
func (a *Adapter) React(_ context.Context, _ string, _ string) error { return nil }

// GetCapabilities returns the base text-only capability set.
func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{HasTextStream: true}
}

// ConvertAudioForPlatform returns the input unchanged — the loopback channel
// performs no audio transcoding.
// Start is a no-op for the loopback adapter; it never receives external messages.
func (a *Adapter) Start(_ context.Context, _ func(context.Context, *models.Message)) error {
	return nil
}

func (a *Adapter) ConvertAudioForPlatform(_ context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, format, nil
}
