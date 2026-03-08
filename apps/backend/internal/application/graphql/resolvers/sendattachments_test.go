package resolvers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/99designs/gqlgen/graphql"
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
	require.Len(t, md.lastInput.Attachments, 1)
	a := md.lastInput.Attachments[0]
	require.Equal(t, "image/jpeg", a.Type)
	require.Equal(t, "pic.jpg", a.Filename)
	require.Equal(t, "image/jpeg", a.MIMEType)
	require.Equal(t, int64(123), a.Size)
}
