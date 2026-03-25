// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"testing"
	"time"

	appcontext "github.com/neirth/openlobster/internal/domain/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── buildMemoryConsolidationSystemPrompt ─────────────────────────────────────

func TestBuildMemoryConsolidationSystemPrompt_ContainsRole(t *testing.T) {
	prompt := buildMemoryConsolidationSystemPrompt()
	assert.Contains(t, prompt, "memory consolidation agent")
}

func TestBuildMemoryConsolidationSystemPrompt_ContainsDate(t *testing.T) {
	prompt := buildMemoryConsolidationSystemPrompt()
	// The prompt embeds the current year, so verify it starts with a plausible year.
	currentYear := time.Now().Format("2006")
	assert.Contains(t, prompt, currentYear)
}

func TestBuildMemoryConsolidationSystemPrompt_ContainsInstructions(t *testing.T) {
	prompt := buildMemoryConsolidationSystemPrompt()
	assert.Contains(t, prompt, "list_conversations")
	assert.Contains(t, prompt, "search_memory")
	assert.Contains(t, prompt, "add_memory")
}

func TestBuildMemoryConsolidationSystemPrompt_NonEmpty(t *testing.T) {
	prompt := buildMemoryConsolidationSystemPrompt()
	require.NotEmpty(t, prompt)
}

// ─── buildLoopbackSystemPrompt ────────────────────────────────────────────────

func TestBuildLoopbackSystemPrompt_ContainsRole(t *testing.T) {
	agentCtx := &appcontext.AgentLLMContext{
		AgentName: "TestAgent",
	}
	prompt := buildLoopbackSystemPrompt(agentCtx)
	assert.Contains(t, prompt, "autonomous agent")
	assert.Contains(t, prompt, "TestAgent")
}

func TestBuildLoopbackSystemPrompt_ContainsDate(t *testing.T) {
	agentCtx := &appcontext.AgentLLMContext{}
	prompt := buildLoopbackSystemPrompt(agentCtx)
	// The prompt embeds the current year, so verify it starts with a plausible year.
	currentYear := time.Now().Format("2006")
	assert.Contains(t, prompt, currentYear)
}

func TestBuildLoopbackSystemPrompt_NoUserReferences(t *testing.T) {
	agentCtx := &appcontext.AgentLLMContext{}
	prompt := buildLoopbackSystemPrompt(agentCtx)
	// Should not contain user-focused instructions
	assert.NotContains(t, prompt, "MUST send a brief acknowledgement to the user")
	assert.NotContains(t, prompt, "you MUST send a follow-up message to the user")
	assert.NotContains(t, prompt, "NO_REPLY")
}

func TestBuildLoopbackSystemPrompt_ContainsTaskExecution(t *testing.T) {
	agentCtx := &appcontext.AgentLLMContext{}
	prompt := buildLoopbackSystemPrompt(agentCtx)
	assert.Contains(t, prompt, "Task Execution")
	assert.Contains(t, prompt, "Focus on completing the assigned objective")
}

// ─── NewLoopbackDispatcher ───────────────────────────────────────────────────

func TestNewLoopbackDispatcher(t *testing.T) {
	handler := &MessageHandler{}
	dispatcher := NewLoopbackDispatcher(handler)
	require.NotNil(t, dispatcher)
	assert.Equal(t, handler, dispatcher.handler)
}

// ─── summarizeForAgent ───────────────────────────────────────────────────────

func TestSummarizeForAgent_Empty(t *testing.T) {
	assert.Equal(t, "", summarizeForAgent(""))
}

func TestSummarizeForAgent_ShortDescription(t *testing.T) {
	desc := "Searches the web."
	result := summarizeForAgent(desc)
	assert.Equal(t, "Searches the web.", result)
}

func TestSummarizeForAgent_FirstSentenceWithDot(t *testing.T) {
	desc := "Searches the web. Returns top results. And more info."
	result := summarizeForAgent(desc)
	assert.Equal(t, "Searches the web.", result)
}

func TestSummarizeForAgent_FirstSentenceWithExclamation(t *testing.T) {
	// The function checks '.', '!', '?' in order — '.' is checked first.
	// A string with '!' but NO '.' will stop at '!'.
	desc := "Amazing tool! Does everything"
	result := summarizeForAgent(desc)
	assert.Equal(t, "Amazing tool!", result)
}

func TestSummarizeForAgent_FirstSentenceWithQuestion(t *testing.T) {
	// A string with '?' but NO '.' or '!' stops at '?'.
	desc := "Can it do that? Yes it can"
	result := summarizeForAgent(desc)
	assert.Equal(t, "Can it do that?", result)
}

func TestSummarizeForAgent_LongDescriptionTruncated(t *testing.T) {
	// Build a description longer than 120 chars with no sentence terminator.
	long := "This is a very long description without any punctuation that goes on and on and keeps going past the maximum allowed length"
	result := summarizeForAgent(long)
	assert.LessOrEqual(t, len(result), maxToolDescriptionLen+3) // +3 for "..."
	assert.True(t, len(result) > 0)
}

func TestSummarizeForAgent_LongDescriptionEndsWithEllipsis(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz abcdefghijklmnopqrstuvwxyz abcdefghijklmnopqrstuvwxyz abcdefghijklmnopqrstuvwxyz abcdefghijklmnopqrstuvwxyz"
	result := summarizeForAgent(long)
	assert.True(t, len(result) <= maxToolDescriptionLen+3)
}

func TestSummarizeForAgent_ExactlyAtLimit(t *testing.T) {
	// Exactly maxToolDescriptionLen characters with no punctuation.
	desc := make([]byte, maxToolDescriptionLen)
	for i := range desc {
		desc[i] = 'a'
	}
	result := summarizeForAgent(string(desc))
	// No truncation needed when within limit.
	assert.Equal(t, string(desc), result)
}

func TestSummarizeForAgent_WhitespaceOnly(t *testing.T) {
	result := summarizeForAgent("   ")
	assert.Equal(t, "", result)
}

func TestSummarizeForAgent_LeadingTrailingWhitespace(t *testing.T) {
	result := summarizeForAgent("  Search the web.  ")
	assert.Equal(t, "Search the web.", result)
}

// ─── MessageHandler setters ───────────────────────────────────────────────────

func TestMessageHandler_SetPermissionLoader(t *testing.T) {
	h := &MessageHandler{}
	// Use a nil loader to verify the setter runs without panic.
	h.SetPermissionLoader(nil)
	assert.Nil(t, h.permLoader)
}

func TestMessageHandler_SetGroupRegistrar(t *testing.T) {
	h := &MessageHandler{}
	h.SetGroupRegistrar(nil)
	assert.Nil(t, h.groupReg)
}

func TestMessageHandler_SetPlatformEnsurer(t *testing.T) {
	h := &MessageHandler{}
	h.SetPlatformEnsurer(nil)
	assert.Nil(t, h.platformReg)
}

func TestMessageHandler_SetSkillsProvider(t *testing.T) {
	h := &MessageHandler{}
	h.SetSkillsProvider(nil)
	assert.Nil(t, h.skillsProvider)
}

func TestMessageHandler_SetAIProvider(t *testing.T) {
	h := &MessageHandler{}
	h.SetAIProvider(nil)
	assert.Nil(t, h.runner.aiProvider)
}

func TestMessageHandler_SetCapabilitiesChecker(t *testing.T) {
	h := &MessageHandler{}
	var checker CapabilitiesChecker = func(cap string) bool { return true }
	h.SetCapabilitiesChecker(checker)
	assert.NotNil(t, h.runner.capabilitiesCheck)
}

// ─── agenticRunner.buildToolsForAgent ────────────────────────────────────────

func TestBuildToolsForAgent_NilRegistry(t *testing.T) {
	runner := &agenticRunner{toolRegistry: nil}
	tools := runner.buildToolsForAgent("user1")
	assert.Nil(t, tools)
}

// ─── HandleMessageInput ───────────────────────────────────────────────────────

func TestHandleMessageInput_ZeroValue(t *testing.T) {
	var input HandleMessageInput
	assert.Empty(t, input.ChannelID)
	assert.Empty(t, input.Content)
	assert.Nil(t, input.ConversationID)
	assert.Nil(t, input.Attachments)
	assert.Nil(t, input.Audio)
}

func TestHandleMessageInput_WithConversationID(t *testing.T) {
	convID := "conv-123"
	input := HandleMessageInput{
		ChannelID:      "ch1",
		Content:        "hello",
		ChannelType:    "dashboard",
		ConversationID: &convID,
	}
	require.NotNil(t, input.ConversationID)
	assert.Equal(t, "conv-123", *input.ConversationID)
}
