// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedReplyAI struct {
	content string
	calls   int32
}

func (f *fixedReplyAI) Chat(_ context.Context, _ ports.ChatRequest) (ports.ChatResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	return ports.ChatResponse{Content: f.content}, nil
}

func (f *fixedReplyAI) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}

func (f *fixedReplyAI) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}

func (f *fixedReplyAI) SupportsAudioInput() bool  { return false }
func (f *fixedReplyAI) SupportsAudioOutput() bool { return false }
func (f *fixedReplyAI) GetMaxTokens() int         { return 512 }
func (f *fixedReplyAI) GetContextWindow() int     { return 8192 }

type countingMessageRepo struct {
	saves int32
}

func (m *countingMessageRepo) Save(_ context.Context, _ *models.Message) error {
	atomic.AddInt32(&m.saves, 1)
	return nil
}

func (m *countingMessageRepo) GetByConversation(_ context.Context, _ string, _ int) ([]models.Message, error) {
	return nil, nil
}

func (m *countingMessageRepo) GetSinceLastCompaction(_ context.Context, _ string) ([]models.Message, error) {
	return nil, nil
}

func (m *countingMessageRepo) GetLastCompaction(_ context.Context, _ string) (*models.Message, error) {
	return nil, nil
}

func (m *countingMessageRepo) GetUnvalidated(_ context.Context, _ int) ([]models.Message, error) {
	return nil, nil
}

func (m *countingMessageRepo) MarkAsValidated(_ context.Context, _ []string) error {
	return nil
}

type countingSessionRepo struct {
	creates int32
	updates int32
}

func (s *countingSessionRepo) Create(_ context.Context, _ *models.Session) error {
	atomic.AddInt32(&s.creates, 1)
	return nil
}

func (s *countingSessionRepo) GetByID(_ context.Context, _ string) (*models.Session, error) {
	return nil, nil
}

func (s *countingSessionRepo) Update(_ context.Context, _ *models.Session) error {
	atomic.AddInt32(&s.updates, 1)
	return nil
}

func (s *countingSessionRepo) GetActiveByUser(_ context.Context, _ string) ([]models.Session, error) {
	return nil, nil
}

func (s *countingSessionRepo) GetActiveByChannel(_ context.Context, _ string) ([]models.Session, error) {
	return nil, nil
}

func (s *countingSessionRepo) GetActiveByGroup(_ context.Context, _ string) ([]models.Session, error) {
	return nil, nil
}

func newAgentChannelHandler(ai ports.AIProviderPort, msgRepo ports.MessageRepositoryPort, sessRepo ports.SessionRepositoryPort) *MessageHandler {
	h := &MessageHandler{
		runner:      agenticRunner{aiProvider: ai},
		messageRepo: msgRepo,
		sessionRepo: sessRepo,
		queue:       newJobQueue(),
	}
	go h.runWorker()
	return h
}

func TestHandle_AgentChannel_IsEphemeralAndReturnsCallbackResponse(t *testing.T) {
	ai := &fixedReplyAI{content: "respuesta interna"}
	msgRepo := &countingMessageRepo{}
	sessRepo := &countingSessionRepo{}
	h := newAgentChannelHandler(ai, msgRepo, sessRepo)

	var got string
	err := h.Handle(context.Background(), HandleMessageInput{
		ChannelID:   "agent",
		ChannelType: "agent",
		SenderID:    "agent",
		Content:     "hola",
		OnAssistantResponse: func(content string) {
			got = content
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "respuesta interna", got)
	assert.Equal(t, int32(1), atomic.LoadInt32(&ai.calls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&msgRepo.saves), "agent channel must not persist messages")
	assert.Equal(t, int32(0), atomic.LoadInt32(&sessRepo.creates), "agent channel must not create sessions")
	assert.Equal(t, int32(0), atomic.LoadInt32(&sessRepo.updates), "agent channel must not update sessions")
}

func TestHandle_AgentChannel_DashboardTypeDoesNotRequireUUIDConversation(t *testing.T) {
	ai := &fixedReplyAI{content: "ok dashboard agent"}
	msgRepo := &countingMessageRepo{}
	sessRepo := &countingSessionRepo{}
	h := newAgentChannelHandler(ai, msgRepo, sessRepo)

	convID := "agent"
	var got string
	err := h.Handle(context.Background(), HandleMessageInput{
		ChannelID:      "agent",
		ChannelType:    "dashboard",
		ConversationID: &convID,
		SenderID:       "dashboard",
		SenderName:     "Dashboard",
		Content:        "ping",
		OnAssistantResponse: func(content string) {
			got = content
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "ok dashboard agent", got)
	assert.Equal(t, int32(1), atomic.LoadInt32(&ai.calls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&msgRepo.saves), "agent channel must not persist messages")
}
