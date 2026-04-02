package resolvers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/require"
)

func newTestDepsMinimal() *Deps {
	reg := registry.NewAgentRegistry()
	return &Deps{AgentRegistry: reg}
}

type mockDispatcher struct {
	lastInput handlers.HandleMessageInput
	called    bool
}

type mockConversationPort struct {
	convs []dto.ConversationSnapshot
}

type mockRouteUserChannelRepo struct {
	lastByUser map[string]struct {
		channelType string
		channelID   string
	}
}

type mockScopedRouteUserChannelRepo struct {
	*mockRouteUserChannelRepo
	lastByUserAndChannel map[string]struct {
		channelType string
		channelID   string
	}
}

func (m *mockConversationPort) ListConversations() ([]dto.ConversationSnapshot, error) {
	return m.convs, nil
}

func (m *mockConversationPort) DeleteUser(ctx context.Context, conversationID string) error {
	return nil
}

func (m *mockConversationPort) DeleteGroup(ctx context.Context, conversationID string) error {
	return nil
}

func (m *mockRouteUserChannelRepo) ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error) {
	return false, nil
}

func (m *mockRouteUserChannelRepo) Create(ctx context.Context, userID, channelType, platformUserID, username string) error {
	return nil
}

func (m *mockRouteUserChannelRepo) GetDisplayNameByUserID(ctx context.Context, userID string) (string, error) {
	return "", nil
}

func (m *mockRouteUserChannelRepo) GetLastChannelForUser(ctx context.Context, userID string) (string, string, error) {
	if m == nil || m.lastByUser == nil {
		return "", "", nil
	}
	entry, ok := m.lastByUser[userID]
	if !ok {
		return "", "", nil
	}
	return entry.channelType, entry.channelID, nil
}

func (m *mockScopedRouteUserChannelRepo) GetLastChannelForUserByChannel(ctx context.Context, userID, channelType string) (string, string, error) {
	if m == nil || m.lastByUserAndChannel == nil {
		return "", "", nil
	}
	entry, ok := m.lastByUserAndChannel[userID+"|"+channelType]
	if !ok {
		return "", "", nil
	}
	return entry.channelType, entry.channelID, nil
}

func (m *mockDispatcher) Handle(ctx context.Context, input handlers.HandleMessageInput) error {
	m.called = true
	m.lastInput = input
	return nil
}

func TestSendMessage_WithAttachmentsVariable(t *testing.T) {
	deps := newTestDepsMinimal()
	r := NewResolver(deps)
	mr := r.Mutation()

	// attach mock dispatcher
	md := &mockDispatcher{}
	deps.MessageDispatcher = md

	// prepare attachments JSON and operation context
	atts := []models.Attachment{{Type: "image/jpeg", Filename: "pic.jpg", MIMEType: "image/jpeg", Size: 123}}
	raw, err := json.Marshal(atts)
	require.NoError(t, err)

	op := &graphql.OperationContext{Variables: map[string]interface{}{"attachments": string(raw)}}
	ctx := graphql.WithOperationContext(context.Background(), op)

	chID := "conv-test"
	res, err := mr.SendMessage(ctx, &chID, nil, "Here is an image")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Success)
	if !*res.Success {
		t.Fatalf("expected success")
	}

	// Ensure dispatcher called and attachments mapped
	require.True(t, md.called, "MessageDispatcher.Handle should be called")
	require.True(t, md.lastInput.FromDashboard)
	require.Equal(t, md.lastInput.ChannelID, md.lastInput.SenderID)
	require.Len(t, md.lastInput.Attachments, 1)
	a := md.lastInput.Attachments[0]
	require.Equal(t, "image/jpeg", a.Type)
	require.Equal(t, "pic.jpg", a.Filename)
	require.Equal(t, "image/jpeg", a.MIMEType)
	require.Equal(t, int64(123), a.Size)
}

func TestSendMessage_UsesConversationChannelRoute(t *testing.T) {
	deps := newTestDepsMinimal()
	r := NewResolver(deps)
	mr := r.Mutation()

	md := &mockDispatcher{}
	deps.MessageDispatcher = md
	deps.ConvPort = &mockConversationPort{convs: []dto.ConversationSnapshot{
		{
			ID:          "conv-telegram-1",
			ChannelID:   "-100777000111",
			ChannelType: "telegram",
			IsGroup:     true,
		},
	}}

	op := &graphql.OperationContext{Variables: map[string]interface{}{}}
	ctx := graphql.WithOperationContext(context.Background(), op)

	cid := "conv-telegram-1"
	res, err := mr.SendMessage(ctx, &cid, nil, "Hola desde dashboard")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, md.called)
	require.Equal(t, "-100777000111", md.lastInput.ChannelID)
	require.Equal(t, "telegram", md.lastInput.ChannelType)
	require.Equal(t, md.lastInput.ChannelID, md.lastInput.SenderID)
	require.True(t, md.lastInput.FromDashboard)
	require.NotNil(t, md.lastInput.ConversationID)
	require.Equal(t, "conv-telegram-1", *md.lastInput.ConversationID)
	require.True(t, md.lastInput.IsGroup)
}

func TestSendMessage_ResolvesDMPlatformRouteFromUserChannel(t *testing.T) {
	deps := newTestDepsMinimal()
	r := NewResolver(deps)
	mr := r.Mutation()

	md := &mockDispatcher{}
	deps.MessageDispatcher = md
	deps.ConvPort = &mockConversationPort{convs: []dto.ConversationSnapshot{
		{
			ID:            "conv-telegram-dm-1",
			ChannelID:     "telegram",
			ChannelType:   "telegram",
			ParticipantID: "user-42",
			IsGroup:       false,
		},
	}}
	deps.UserChannelRepo = &mockRouteUserChannelRepo{lastByUser: map[string]struct {
		channelType string
		channelID   string
	}{
		"user-42": {channelType: "telegram", channelID: "813579246"},
	}}

	op := &graphql.OperationContext{Variables: map[string]interface{}{}}
	ctx := graphql.WithOperationContext(context.Background(), op)

	cid := "conv-telegram-dm-1"
	res, err := mr.SendMessage(ctx, &cid, nil, "Hola desde dashboard")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, md.called)
	require.Equal(t, "813579246", md.lastInput.ChannelID)
	require.Equal(t, "telegram", md.lastInput.ChannelType)
	require.Equal(t, md.lastInput.ChannelID, md.lastInput.SenderID)
	require.True(t, md.lastInput.FromDashboard)
	require.NotNil(t, md.lastInput.ConversationID)
	require.Equal(t, "conv-telegram-dm-1", *md.lastInput.ConversationID)
	require.False(t, md.lastInput.IsGroup)
}

func TestSendMessage_IgnoresInvalidLastChannelFromUserChannel(t *testing.T) {
	deps := newTestDepsMinimal()
	r := NewResolver(deps)
	mr := r.Mutation()

	md := &mockDispatcher{}
	deps.MessageDispatcher = md
	deps.ConvPort = &mockConversationPort{convs: []dto.ConversationSnapshot{
		{
			ID:            "conv-telegram-dm-2",
			ChannelID:     "telegram",
			ChannelType:   "telegram",
			ParticipantID: "user-43",
			IsGroup:       false,
		},
	}}
	deps.UserChannelRepo = &mockRouteUserChannelRepo{lastByUser: map[string]struct {
		channelType string
		channelID   string
	}{
		"user-43": {channelType: "telegram", channelID: "dashboard"},
	}}

	op := &graphql.OperationContext{Variables: map[string]interface{}{}}
	ctx := graphql.WithOperationContext(context.Background(), op)

	cid := "conv-telegram-dm-2"
	res, err := mr.SendMessage(ctx, &cid, nil, "Hola desde dashboard")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, md.called)
	require.Equal(t, "telegram", md.lastInput.ChannelID)
	require.Equal(t, "telegram", md.lastInput.ChannelType)
	require.Equal(t, md.lastInput.ChannelID, md.lastInput.SenderID)
	require.True(t, md.lastInput.FromDashboard)
}

func TestSendMessage_ResolvesDMRouteWithinConversationChannel(t *testing.T) {
	deps := newTestDepsMinimal()
	r := NewResolver(deps)
	mr := r.Mutation()

	md := &mockDispatcher{}
	deps.MessageDispatcher = md
	deps.ConvPort = &mockConversationPort{convs: []dto.ConversationSnapshot{
		{
			ID:            "conv-twilio-dm-1",
			ChannelID:     "twilio",
			ChannelType:   "twilio",
			ParticipantID: "user-99",
			IsGroup:       false,
		},
	}}
	deps.UserChannelRepo = &mockScopedRouteUserChannelRepo{
		mockRouteUserChannelRepo: &mockRouteUserChannelRepo{lastByUser: map[string]struct {
			channelType string
			channelID   string
		}{
			"user-99": {channelType: "telegram", channelID: "813579246"},
		}},
		lastByUserAndChannel: map[string]struct {
			channelType string
			channelID   string
		}{
			"user-99|twilio": {channelType: "twilio", channelID: "+34600111222"},
		},
	}

	op := &graphql.OperationContext{Variables: map[string]interface{}{}}
	ctx := graphql.WithOperationContext(context.Background(), op)

	cid := "conv-twilio-dm-1"
	res, err := mr.SendMessage(ctx, &cid, nil, "Hola desde dashboard")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, md.called)
	require.Equal(t, "+34600111222", md.lastInput.ChannelID)
	require.Equal(t, "twilio", md.lastInput.ChannelType)
	require.Equal(t, md.lastInput.ChannelID, md.lastInput.SenderID)
	require.True(t, md.lastInput.FromDashboard)
}
