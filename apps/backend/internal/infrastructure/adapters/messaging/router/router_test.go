// Copyright (c) OpenLobster contributors. See LICENSE for details.

package router

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMessagingPort struct{}

func (s *stubMessagingPort) SendMessage(ctx context.Context, msg *models.Message) error {
	return nil
}

func (s *stubMessagingPort) SendMedia(ctx context.Context, media *ports.Media) error {
	return nil
}

func (s *stubMessagingPort) SendTyping(ctx context.Context, channelID string) error {
	return nil
}

func (s *stubMessagingPort) HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error) {
	return nil, nil
}

func (s *stubMessagingPort) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	return nil, nil
}

func (s *stubMessagingPort) React(ctx context.Context, messageID string, emoji string) error {
	return nil
}

func (s *stubMessagingPort) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{}
}

func (s *stubMessagingPort) ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, format, nil
}

func (s *stubMessagingPort) Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error {
	return nil
}

func TestRouterSendMessage_NoAdapterIncludesActiveTypes(t *testing.T) {
	reg := New()
	reg.Set("discord", &stubMessagingPort{})
	reg.Set("slack", &stubMessagingPort{})
	r := NewRouter(reg)

	err := r.SendMessage(context.Background(), &models.Message{
		ChannelID: "3654107",
		Content:   "hola",
		Metadata:  map[string]interface{}{"channel_type": "telegram"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no adapter for channel_type=\"telegram\"")
	assert.Contains(t, err.Error(), "active_adapters=[discord slack]")
}
