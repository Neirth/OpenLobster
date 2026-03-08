// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"context"
)

const loopbackChannelID = "loopback"

// LoopbackDispatcher implements ports.TaskDispatcherPort and bridges the domain
// Scheduler with the MessageHandler.
type LoopbackDispatcher struct {
	handler *MessageHandler
}

// NewLoopbackDispatcher constructs a LoopbackDispatcher that routes task
// execution through handler.
func NewLoopbackDispatcher(handler *MessageHandler) *LoopbackDispatcher {
	return &LoopbackDispatcher{handler: handler}
}

// Dispatch sends prompt through the full agentic pipeline via the loopback channel.
func (d *LoopbackDispatcher) Dispatch(ctx context.Context, prompt string) error {
	return d.handler.Handle(ctx, HandleMessageInput{
		ChannelID:   loopbackChannelID,
		Content:     prompt,
		ChannelType: loopbackChannelID,
	})
}
