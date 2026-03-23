// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Tool registry helpers ────────────────────────────────────────────────────

type mockInternalTool struct {
	def    mcp.ToolDefinition
	result json.RawMessage
	err    error
}

func (m *mockInternalTool) Definition() mcp.ToolDefinition { return m.def }
func (m *mockInternalTool) Execute(_ context.Context, _ map[string]interface{}) (json.RawMessage, error) {
	return m.result, m.err
}

// ─── AI provider with tool_use ────────────────────────────────────────────────

type toolAIProvider struct {
	calls    int
	firstResp ports.ChatResponse
	synthResp ports.ChatResponse
	err       error
}

func (m *toolAIProvider) Chat(_ context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	if m.err != nil {
		return ports.ChatResponse{}, m.err
	}
	m.calls++
	if m.calls == 1 {
		return m.firstResp, nil
	}
	return m.synthResp, nil
}
func (m *toolAIProvider) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}
func (m *toolAIProvider) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}
func (m *toolAIProvider) SupportsAudioInput() bool  { return false }
func (m *toolAIProvider) SupportsAudioOutput() bool { return false }
func (m *toolAIProvider) GetMaxTokens() int         { return 4096 }
func (m *toolAIProvider) GetContextWindow() int     { return 8192 }

// ─── summarizeForAgent ────────────────────────────────────────────────────────

func TestSummarizeForAgent_Empty(t *testing.T) {
	assert.Equal(t, "", summarizeForAgent(""))
}

func TestSummarizeForAgent_FirstSentence(t *testing.T) {
	got := summarizeForAgent("Hello world. More text here.")
	assert.Equal(t, "Hello world.", got)
}

func TestSummarizeForAgent_ExclamationMark(t *testing.T) {
	// Only ! in the string (no '.' before it), so it returns the first ! sentence.
	got := summarizeForAgent("Alert! No more periods here")
	assert.Equal(t, "Alert!", got)
}

func TestSummarizeForAgent_QuestionMark(t *testing.T) {
	// Only ? in the string (no '.' before it), so it returns the first ? sentence.
	got := summarizeForAgent("What happened? No more periods")
	assert.Equal(t, "What happened?", got)
}

func TestSummarizeForAgent_ShortNoSentenceEnd(t *testing.T) {
	got := summarizeForAgent("short")
	assert.Equal(t, "short", got)
}

func TestSummarizeForAgent_TruncatesLong(t *testing.T) {
	long := "this is a very long description without any sentence terminator and it goes on and on and on and on and even more"
	got := summarizeForAgent(long)
	assert.True(t, len(got) <= maxToolDescriptionLen+10, "should truncate")
	assert.True(t, len(got) > 0)
}

func TestSummarizeForAgent_WithSpaceBeforeTruncation(t *testing.T) {
	// Build a string longer than maxToolDescriptionLen with no sentence terminator
	// and a space before the cutpoint so word-boundary truncation is triggered.
	word := "longword "
	var s string
	for len(s) < maxToolDescriptionLen+20 {
		s += word
	}
	got := summarizeForAgent(s)
	assert.NotEmpty(t, got)
	assert.True(t, len(got) <= maxToolDescriptionLen+5)
}

// ─── SetToolRegistry / SetPermissionManager / SetCapabilitiesChecker ─────────

func TestService_SetToolRegistry(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	tr := mcp.NewToolRegistry(false, nil)
	svc.SetToolRegistry(tr)
	assert.Equal(t, tr, svc.toolRegistry)
}

func TestService_SetPermissionManager(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	pm := permissions.NewManager()
	svc.SetPermissionManager(pm)
	assert.Equal(t, pm, svc.permManager)
}

func TestService_SetCapabilitiesChecker(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	var called bool
	checker := func(cap string) bool { called = true; return true }
	svc.SetCapabilitiesChecker(checker)
	require.NotNil(t, svc.capabilitiesCheck)
	_ = svc.capabilitiesCheck("any")
	assert.True(t, called)
}

// ─── adapter methods ─────────────────────────────────────────────────────────

func TestAdapter_IDAndName(t *testing.T) {
	ai := &mockAIProvider{response: "x"}
	svc := NewService(ai, 5, 2*time.Second)

	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "alpha", Model: "gpt-4"}, "task")
	require.NoError(t, err)
	require.NotNil(t, agent)

	assert.NotEmpty(t, agent.ID())
	assert.Equal(t, "alpha", agent.Name())
}

// ─── buildToolsForSubAgent ────────────────────────────────────────────────────

func TestBuildToolsForSubAgent_NilRegistry(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	tools := svc.buildToolsForSubAgent("user1")
	assert.Nil(t, tools)
}

func TestBuildToolsForSubAgent_FiltersRestrictedTools(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "terminal_spawn", permissions.PermissionAlways)
	pm.SetPermission("default", "subagent_spawn", permissions.PermissionAlways)
	pm.SetPermission("default", "terminal_list_processes", permissions.PermissionAlways)
	pm.SetPermission("default", "terminal_get_output", permissions.PermissionAlways)

	tr := mcp.NewToolRegistry(false, pm)
	for _, name := range []string{"terminal_spawn", "subagent_spawn", "terminal_list_processes", "terminal_get_output"} {
		tr.RegisterInternal(name, &mockInternalTool{def: mcp.ToolDefinition{Name: name, Description: "Restricted."}})
	}

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tools := svc.buildToolsForSubAgent("user1")
	for _, t2 := range tools {
		fn := t2.Function
		if fn != nil {
			assert.NotEqual(t, "terminal_spawn", fn.Name)
			assert.NotEqual(t, "subagent_spawn", fn.Name)
		}
	}
}

func TestBuildToolsForSubAgent_CapabilityFilterHidesTool(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("user1", "read_file", permissions.PermissionAlways)

	schema := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)
	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("read_file", &mockInternalTool{
		def: mcp.ToolDefinition{Name: "read_file", Description: "Reads files.", InputSchema: schema},
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)
	// CapabilityForTool("read_file") returns "filesystem"; disable it.
	svc.SetCapabilitiesChecker(func(cap string) bool {
		return cap != "filesystem"
	})

	tools := svc.buildToolsForSubAgent("user1")
	for _, tool := range tools {
		if tool.Function != nil {
			assert.NotEqual(t, "read_file", tool.Function.Name)
		}
	}
}

func TestBuildToolsForSubAgent_PermissionDenyHidesTool(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("user1", "read_file", permissions.PermissionDeny)

	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("read_file", &mockInternalTool{
		def: mcp.ToolDefinition{Name: "read_file", Description: "Reads."},
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)
	svc.SetPermissionManager(pm)

	tools := svc.buildToolsForSubAgent("user1")
	for _, t2 := range tools {
		if t2.Function != nil {
			assert.NotEqual(t, "read_file", t2.Function.Name)
		}
	}
}

func TestBuildToolsForSubAgent_IncludesNormalTool(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "list_files", permissions.PermissionAlways)

	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("list_files", &mockInternalTool{
		def: mcp.ToolDefinition{
			Name:        "list_files",
			Description: "Lists files in a directory.",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tools := svc.buildToolsForSubAgent("user1")
	require.Len(t, tools, 1)
	assert.Equal(t, "list_files", tools[0].Function.Name)
}

// ─── jsonArgumentsToMap ───────────────────────────────────────────────────────

func TestJsonArgumentsToMap_Empty(t *testing.T) {
	result := jsonArgumentsToMap("")
	assert.Nil(t, result)
}

func TestJsonArgumentsToMap_Valid(t *testing.T) {
	result := jsonArgumentsToMap(`{"key":"value","count":1}`)
	require.NotNil(t, result)
	assert.Equal(t, "value", result["key"])
}

func TestJsonArgumentsToMap_Invalid(t *testing.T) {
	result := jsonArgumentsToMap("not json")
	assert.Nil(t, result)
}

// ─── dispatchToolCall ─────────────────────────────────────────────────────────

func TestDispatchToolCall_Success(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "echo_tool", permissions.PermissionAlways)
	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("echo_tool", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "echo_tool"},
		result: json.RawMessage(`{"output":"hello"}`),
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tc := ports.ToolCall{
		ID: "call-1",
		Function: ports.FunctionCall{
			Name:      "echo_tool",
			Arguments: `{}`,
		},
	}

	msg := svc.dispatchToolCall(context.Background(), tc)
	assert.Equal(t, "tool", msg.Role)
	assert.Equal(t, "call-1", msg.ToolCallID)
	assert.Contains(t, msg.Content, "hello")
}

func TestDispatchToolCall_ToolError(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "bad_tool", permissions.PermissionAlways)
	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("bad_tool", &mockInternalTool{
		def: mcp.ToolDefinition{Name: "bad_tool"},
		err: fmt.Errorf("tool failed"),
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tc := ports.ToolCall{
		ID: "call-err",
		Function: ports.FunctionCall{Name: "bad_tool", Arguments: `{}`},
	}

	msg := svc.dispatchToolCall(context.Background(), tc)
	assert.Equal(t, "tool", msg.Role)
	assert.Contains(t, msg.Content, "error")
}

func TestDispatchToolCall_MultimodalImageBlock(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "img_tool", permissions.PermissionAlways)

	// Build a valid multimodal response.
	imgData := []byte{0xFF, 0xD8, 0xFF}
	encoded := base64.StdEncoding.EncodeToString(imgData)
	raw := fmt.Sprintf(`{"_openlobster_blocks":[{"type":"image","mime_type":"image/jpeg","data":"%s","text":"photo"}]}`, encoded)

	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("img_tool", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "img_tool"},
		result: json.RawMessage(raw),
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tc := ports.ToolCall{
		ID:       "call-img",
		Function: ports.FunctionCall{Name: "img_tool", Arguments: `{}`},
	}
	msg := svc.dispatchToolCall(context.Background(), tc)
	assert.Equal(t, "tool", msg.Role)
	assert.Len(t, msg.Blocks, 1)
	assert.Equal(t, ports.ContentBlockImage, msg.Blocks[0].Type)
}

func TestDispatchToolCall_MultimodalAudioBlock(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "audio_tool", permissions.PermissionAlways)

	audioData := []byte{0x49, 0x44, 0x33}
	encoded := base64.StdEncoding.EncodeToString(audioData)
	raw := fmt.Sprintf(`{"_openlobster_blocks":[{"type":"audio","mime_type":"audio/mpeg","data":"%s","text":"clip"}]}`, encoded)

	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("audio_tool", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "audio_tool"},
		result: json.RawMessage(raw),
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tc := ports.ToolCall{
		ID:       "call-audio",
		Function: ports.FunctionCall{Name: "audio_tool", Arguments: `{}`},
	}
	msg := svc.dispatchToolCall(context.Background(), tc)
	assert.Equal(t, "tool", msg.Role)
	assert.Len(t, msg.Blocks, 1)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[0].Type)
}

func TestDispatchToolCall_MultimodalBadBase64(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "bad_b64", permissions.PermissionAlways)

	raw := `{"_openlobster_blocks":[{"type":"image","mime_type":"image/png","data":"!!!invalid_base64!!!","text":""}]}`

	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("bad_b64", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "bad_b64"},
		result: json.RawMessage(raw),
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tc := ports.ToolCall{
		ID:       "call-b64",
		Function: ports.FunctionCall{Name: "bad_b64", Arguments: `{}`},
	}
	// Should not panic; bad base64 blocks are skipped, falls through to Content path.
	msg := svc.dispatchToolCall(context.Background(), tc)
	assert.Equal(t, "tool", msg.Role)
}

// ─── runAgenticLoop ───────────────────────────────────────────────────────────

func TestRunAgenticLoop_ToolUseLoop(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "echo_tool", permissions.PermissionAlways)
	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("echo_tool", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "echo_tool"},
		result: json.RawMessage(`{"output":"done"}`),
	})

	ai := &toolAIProvider{
		firstResp: ports.ChatResponse{
			StopReason: "tool_use",
			ToolCalls: []ports.ToolCall{{
				ID: "tc1",
				Function: ports.FunctionCall{
					Name:      "echo_tool",
					Arguments: `{}`,
				},
			}},
			Content: "",
		},
		synthResp: ports.ChatResponse{Content: "Final answer.", StopReason: "stop"},
	}

	svc := NewService(ai, 5, time.Minute)
	svc.SetToolRegistry(tr)

	messages := []ports.ChatMessage{{Role: "user", Content: "do something"}}
	result, err := svc.runAgenticLoop(context.Background(), "model", messages, []ports.Tool{})
	require.NoError(t, err)
	assert.Equal(t, "Final answer.", result)
}

func TestRunAgenticLoop_NoToolUseImmediateReply(t *testing.T) {
	ai := &mockAIProvider{response: "Direct answer."}
	svc := NewService(ai, 5, time.Minute)

	pm := permissions.NewManager()
	tr := mcp.NewToolRegistry(false, pm)
	svc.SetToolRegistry(tr)

	messages := []ports.ChatMessage{{Role: "user", Content: "hello"}}
	result, err := svc.runAgenticLoop(context.Background(), "model", messages, []ports.Tool{})
	require.NoError(t, err)
	assert.Equal(t, "Direct answer.", result)
}

func TestRunAgenticLoop_AIError(t *testing.T) {
	ai := &toolAIProvider{err: fmt.Errorf("AI failure")}
	svc := NewService(ai, 5, time.Minute)

	pm := permissions.NewManager()
	tr := mcp.NewToolRegistry(false, pm)
	svc.SetToolRegistry(tr)

	messages := []ports.ChatMessage{{Role: "user", Content: "hello"}}
	_, err := svc.runAgenticLoop(context.Background(), "model", messages, []ports.Tool{})
	assert.Error(t, err)
}

func TestRunAgenticLoop_NoReply_ReturnsNO_REPLY(t *testing.T) {
	// AI returns tool_use but with empty content; exhausts rounds and synthResp also empty.
	// Without tools executed yet means AI never calls tool_use → returns NO_REPLY.
	ai := &mockAIProvider{response: ""} // StopReason != "tool_use", empty content
	svc := NewService(ai, 5, time.Minute)

	pm := permissions.NewManager()
	tr := mcp.NewToolRegistry(false, pm)
	svc.SetToolRegistry(tr)

	messages := []ports.ChatMessage{{Role: "user", Content: "hello"}}
	result, err := svc.runAgenticLoop(context.Background(), "model", messages, []ports.Tool{})
	require.NoError(t, err)
	assert.Equal(t, "NO_REPLY", result)
}

// ─── Spawn with tool registry ────────────────────────────────────────────────

func TestService_Spawn_WithToolRegistry_RunsAgenticLoop(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "noop_tool", permissions.PermissionAlways)
	tr := mcp.NewToolRegistry(false, pm)
	tr.RegisterInternal("noop_tool", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "noop_tool", Description: "No-op."},
		result: json.RawMessage(`{}`),
	})

	ai := &mockAIProvider{response: "tool loop done"}
	svc := NewService(ai, 5, 2*time.Second)
	svc.SetToolRegistry(tr)

	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "loop-agent", Model: "gpt"}, "run")
	require.NoError(t, err)
	require.NotNil(t, agent)

	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, "tool loop done", agent.Result())
}

func TestService_Spawn_WithToolRegistry_AIError_StatusFailed(t *testing.T) {
	pm := permissions.NewManager()
	tr := mcp.NewToolRegistry(false, pm)

	ai := &toolAIProvider{err: fmt.Errorf("network error")}
	svc := NewService(ai, 5, 2*time.Second)
	svc.SetToolRegistry(tr)

	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "fail-agent", Model: "gpt"}, "task")
	require.NoError(t, err)

	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, StatusFailed, agent.Status())
}

func TestDispatchToolCall_ColonInToolName(t *testing.T) {
	pm := permissions.NewManager()
	pm.SetPermission("default", "server:tool", permissions.PermissionAlways)
	tr := mcp.NewToolRegistry(false, pm)

	// Register via MCP (qualified name uses ':')
	type fakeMCPClient struct{ mcp.MCPClient }
	_ = tr.RegisterMCP("server", nil, []mcp.ToolDefinition{})

	// Use internal tool to simulate the colon replacement behaviour.
	tr.RegisterInternal("server:tool", &mockInternalTool{
		def:    mcp.ToolDefinition{Name: "server:tool"},
		result: json.RawMessage(`{"ok":true}`),
	})

	svc := NewService(nil, 5, time.Minute)
	svc.SetToolRegistry(tr)

	tc := ports.ToolCall{
		ID:       "call-colon",
		Function: ports.FunctionCall{Name: "server:tool", Arguments: `{}`},
	}
	msg := svc.dispatchToolCall(context.Background(), tc)
	assert.Equal(t, "tool", msg.Role)
	// ToolName should have the colon replaced by __
	assert.Equal(t, "server__tool", msg.ToolName)
}

func TestService_Spawn_WithTimeout(t *testing.T) {
	// Spawn with a non-zero timeout in the config; agent completes quickly.
	ai := &mockAIProvider{response: "done"}
	svc := NewService(ai, 5, 5*time.Second)

	cfg := mcp.SubAgentConfig{Name: "timed", Model: "gpt", Timeout: 10} // 10 seconds
	agent, err := svc.Spawn(context.Background(), cfg, "task")
	require.NoError(t, err)
	require.NotNil(t, agent)

	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, "done", agent.Result())
}
