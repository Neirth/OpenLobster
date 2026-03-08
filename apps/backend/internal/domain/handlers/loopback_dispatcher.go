// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"context"
	"time"
)

const loopbackChannelID = "loopback"

// buildMemoryConsolidationSystemPrompt returns the dedicated system prompt for
// the automated memory consolidation agent. The current timestamp is embedded
// at call time so the model always receives an accurate date.
func buildMemoryConsolidationSystemPrompt() string {
	return `## Role

You are an internal memory consolidation agent for OpenLobster. Your sole purpose
is to review recent conversations and extract durable knowledge about users into
the long-term memory graph. You do NOT interact with users.

## Instructions

1. Call list_conversations to obtain the list of all stored conversations.
2. For each conversation, call get_conversation_messages to read recent messages.
3. Analyse the messages and identify:
   - Facts about the user (name, occupation, interests, preferences, location, etc.)
   - Relationships between people or concepts mentioned.
   - Any other context that would help personalise future interactions.
4. For each fact found:
   - Call add_memory to store a free-text fact linked to the user.
   - Call set_user_property to store structured attributes (e.g. name, language,
     timezone, occupation).
   - Call add_relation to record relationships between entities.
5. After processing all conversations, stop. Do not send any visible reply.

## Rules

- Only store verifiable facts explicitly stated in the conversation.
- Do not invent or infer information that is not clearly implied.
- Do not store sensitive personal data (passwords, payment details, health records).
- Prefer concise, factual statements ("User prefers dark mode") over vague ones.
- Never use NO_REPLY — simply finish calling tools and return when done.

## Current Date

` + time.Now().Format("Monday, 2 January 2006 — 15:04:05 MST") + "\n"
}

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
// When the prompt originates from the memory consolidation scheduler, a dedicated
// system prompt is injected so the model knows its consolidation role.
func (d *LoopbackDispatcher) Dispatch(ctx context.Context, prompt string) error {
	return d.handler.Handle(ctx, HandleMessageInput{
		ChannelID:    loopbackChannelID,
		Content:      prompt,
		ChannelType:  loopbackChannelID,
		SystemPrompt: buildMemoryConsolidationSystemPrompt(),
	})
}
