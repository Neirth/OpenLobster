// Copyright (c) OpenLobster contributors. See LICENSE for details.

package message_compaction

import (
	"context"
	"errors"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil)
	assert.NotNil(t, svc)
	assert.Equal(t, 0.85, svc.ThresholdRatio)
}

func TestShouldCompact_UnderThreshold(t *testing.T) {
	svc := NewService(nil, nil)
	messages := []ports.ChatMessage{{Role: "user", Content: "short"}}
	assert.False(t, svc.ShouldCompact(messages, 10000))
}

func TestShouldCompact_OverThreshold(t *testing.T) {
	svc := NewService(nil, nil)
	content := ""
	for i := 0; i < 10000; i++ {
		content += "x"
	}
	// Need at least 2 user messages to compact (min history requirement)
	messages := []ports.ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "ok"},
		{Role: "user", Content: content},
	}
	assert.True(t, svc.ShouldCompact(messages, 1000))
}

func TestShouldCompact_SingleUserMessage_NeverCompacts(t *testing.T) {
	svc := NewService(nil, nil)
	content := ""
	for i := 0; i < 10000; i++ {
		content += "x"
	}
	// Even over token threshold, 1 user message = no compaction
	messages := []ports.ChatMessage{{Role: "user", Content: content}}
	assert.False(t, svc.ShouldCompact(messages, 1000))
}

func TestCompact_NoMessages(t *testing.T) {
	repo := &mockMsgRepo{messages: nil}
	svc := NewService(repo, &mockAI{})
	_, err := svc.Compact(context.Background(), "conv1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages")
}

func TestCompact_RepoError(t *testing.T) {
	repo := &mockMsgRepo{err: errors.New("db error")}
	svc := NewService(repo, &mockAI{})
	_, err := svc.Compact(context.Background(), "conv1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestCompact_AIError(t *testing.T) {
	repo := &mockMsgRepo{messages: []models.Message{{Role: "user", Content: "hi"}}}
	ai := &mockAI{err: errors.New("ai error")}
	svc := NewService(repo, ai)
	_, err := svc.Compact(context.Background(), "conv1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "summarisation failed")
}

func TestCompact_EmptyAIResponse(t *testing.T) {
	repo := &mockMsgRepo{messages: []models.Message{{Role: "user", Content: "hi"}}}
	ai := &mockAI{content: ""}
	svc := NewService(repo, ai)
	_, err := svc.Compact(context.Background(), "conv1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty content")
}

func TestCompact_Success(t *testing.T) {
	repo := &mockMsgRepo{messages: []models.Message{{Role: "user", Content: "hi"}}}
	ai := &mockAI{content: "Summary of conversation."}
	svc := NewService(repo, ai)
	msg, err := svc.Compact(context.Background(), "conv1")
	assert.NoError(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, "compaction", msg.Role)
	assert.Equal(t, "Summary of conversation.", msg.Content)
	assert.True(t, repo.saved)
}

func TestBuildMessages_NoSystemPrompt(t *testing.T) {
	repo := &mockMsgRepo{messages: []models.Message{{Role: "user", Content: "hello"}}}
	svc := NewService(repo, nil)
	out, err := svc.BuildMessages(context.Background(), "c1", "")
	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "user", out[0].Role)
	assert.Equal(t, "hello", out[0].Content)
}

func TestBuildMessages_WithSystemPrompt(t *testing.T) {
	repo := &mockMsgRepo{messages: nil}
	svc := NewService(repo, nil)
	out, err := svc.BuildMessages(context.Background(), "c1", "You are helpful.")
	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "system", out[0].Role)
	assert.Equal(t, "You are helpful.", out[0].Content)
}

func TestBuildMessages_WithLastCompaction(t *testing.T) {
	repo := &mockMsgRepo{
		lastCompaction: &models.Message{Content: "Prior summary.", Role: "compaction"},
		messages:       []models.Message{{Role: "user", Content: "new msg"}},
	}
	svc := NewService(repo, nil)
	out, err := svc.BuildMessages(context.Background(), "c1", "System")
	assert.NoError(t, err)
	assert.Len(t, out, 3)
	assert.Equal(t, "system", out[0].Role)
	assert.Equal(t, "System", out[0].Content)
	assert.Contains(t, out[1].Content, "Prior summary")
	assert.Equal(t, "user", out[2].Role)
}

func TestBuildMessages_SkipsCompactionMessages(t *testing.T) {
	repo := &mockMsgRepo{
		messages: []models.Message{
			{Role: "compaction", Content: "summary"},
			{Role: "user", Content: "user msg"},
		},
	}
	svc := NewService(repo, nil)
	out, err := svc.BuildMessages(context.Background(), "c1", "")
	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "user", out[0].Role)
}

func TestBuildMessages_GetLastCompactionErr(t *testing.T) {
	repo := &mockMsgRepo{getLastErr: errors.New("get last err")}
	svc := NewService(repo, nil)
	_, err := svc.BuildMessages(context.Background(), "c1", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GetLastCompaction")
}

func TestBuildMessages_GetSinceErr(t *testing.T) {
	repo := &mockMsgRepo{getSinceErr: errors.New("get since err")}
	svc := NewService(repo, nil)
	_, err := svc.BuildMessages(context.Background(), "c1", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GetSinceLastCompaction")
}

type mockMsgRepo struct {
	messages       []models.Message
	lastCompaction *models.Message
	err            error
	getLastErr     error
	getSinceErr    error
	saved          bool
}

func (m *mockMsgRepo) GetSinceLastCompaction(_ context.Context, _ string) ([]models.Message, error) {
	if m.getSinceErr != nil {
		return nil, m.getSinceErr
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.messages, nil
}

func (m *mockMsgRepo) GetLastCompaction(_ context.Context, _ string) (*models.Message, error) {
	if m.getLastErr != nil {
		return nil, m.getLastErr
	}
	if m.lastCompaction != nil {
		return m.lastCompaction, nil
	}
	return nil, ports.ErrNotFound
}

func (m *mockMsgRepo) Save(_ context.Context, msg *models.Message) error {
	m.saved = true
	return nil
}

func (m *mockMsgRepo) GetByConversation(_ context.Context, _ string, _ int) ([]models.Message, error) {
	return nil, nil
}

func (m *mockMsgRepo) GetUnvalidated(ctx context.Context, limit int) ([]models.Message, error) {
	return nil, nil
}

func (m *mockMsgRepo) MarkAsValidated(ctx context.Context, ids []string) error {
	return nil
}

type mockAI struct {
	content string
	err     error
}

func (m *mockAI) Chat(_ context.Context, _ ports.ChatRequest) (ports.ChatResponse, error) {
	if m.err != nil {
		return ports.ChatResponse{}, m.err
	}
	return ports.ChatResponse{Content: m.content}, nil
}

func (m *mockAI) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, errors.New("not implemented")
}

func (m *mockAI) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, errors.New("not implemented")
}

func (m *mockAI) SupportsAudioInput() bool  { return false }
func (m *mockAI) SupportsAudioOutput() bool { return false }
func (m *mockAI) GetMaxTokens() int         { return 4096 }
func (m *mockAI) GetContextWindow() int     { return 8192 }
