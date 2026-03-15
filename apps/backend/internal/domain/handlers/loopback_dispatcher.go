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

## Critical Rules for Quality Memory Consolidation

BEFORE making ANY memory modification:
1. Check if the fact/relation already exists by searching with search_memory
2. ONLY create nodes if they do NOT already exist in the knowledge graph
3. Do NOT create duplicate nodes or intermediate nodes
4. ONLY modify if something truly new is discovered
5. If no modifications are needed, SKIP and move to next conversation

## Instructions

1. Call list_conversations to obtain the list of all stored conversations.
2. For each conversation, call get_conversation_messages to read recent messages.
3. For each conversation, identify potential new knowledge:
   - Facts about the user (name, occupation, interests, preferences, location, etc.)
   - Relationships between people or concepts mentioned
   - Structured attributes (e.g. language, timezone, occupation)
   - Context that would help personalize future interactions

4. **VERIFICATION STEP**: Before storing EACH fact:
   - Use search_memory(query=<keyword>, for_user=<participant_name>) to check if this fact already exists for that user
   - If found: SKIP this fact, do NOT add it again
   - If NOT found: proceed to add it

5. When storing new facts:
   - Always pass for_user=<participant_name> (from the conversation's participantName field) to every add_memory, search_memory, and set_user_property call so that facts are stored under the correct user and not under a shared loopback user.
   - Call set_user_property(..., for_user=<participant_name>) for the user's own attributes (real name, phone, birthday, language, timezone, occupation)
   - Call add_memory(..., for_user=<participant_name>) for facts that link the user to things or places (e.g. lives in Valencia → label='Valencia', relation='LIVES_IN'; likes X → relation='LIKES')
   - Call add_user_relation when two users are related (e.g. friends → from_user, to_user, relation='FRIEND_OF')
   - Keep each fact independent (avoid intermediate nodes or grouping nodes)

6. After processing all conversations, stop. Do not send any visible reply.

## Rules for Node Creation

- Only store verifiable facts explicitly stated in the conversation
- Do not invent or infer information that is not clearly implied
- Do not store sensitive personal data (passwords, payment details, health records)
- Each fact should be concise and self-contained ("User prefers dark mode", not "UI preferences")
- Never create intermediate nodes just to organize relationships
- Never leave nodes without clear, descriptive titles
- If you find an existing node with missing or incomplete data, use edit_memory_node to improve it
- NEVER use NO_REPLY — simply finish calling tools and return when done

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
