package ports

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatMessage(t *testing.T) {
	msg := ChatMessage{
		Role:    "user",
		Content: "Hello",
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello", msg.Content)
}

func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
	}

	assert.Equal(t, "gpt-4", req.Model)
	assert.Len(t, req.Messages, 2)
}

func TestToolCall(t *testing.T) {
	call := ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: FunctionCall{
			Name:      "send_message",
			Arguments: `{"channel": "telegram", "content": "Hello"}`,
		},
	}

	assert.Equal(t, "call_123", call.ID)
	assert.Equal(t, "send_message", call.Function.Name)
}

func TestChatRequestWithAudio(t *testing.T) {
	req := ChatRequestWithAudio{
		Model:     "gpt-4-audio",
		AudioData: []byte{0x00, 0x01, 0x02},
		Messages:  []ChatMessage{{Role: "user", Content: "Listen to this"}},
	}

	assert.NotEmpty(t, req.AudioData)
	assert.Len(t, req.Messages, 1)
}

func TestChatResponse(t *testing.T) {
	resp := ChatResponse{
		Content:    "Hello!",
		StopReason: "stop",
	}

	assert.Equal(t, "Hello!", resp.Content)
	assert.Equal(t, "stop", resp.StopReason)
}

func TestSubAgentStatus(t *testing.T) {
	type SubAgentStatus string
	const (
		SubAgentStatusRunning SubAgentStatus = "running"
		SubAgentStatusDone    SubAgentStatus = "done"
		SubAgentStatusFailed  SubAgentStatus = "failed"
	)
	assert.Equal(t, SubAgentStatus("running"), SubAgentStatusRunning)
	assert.Equal(t, SubAgentStatus("done"), SubAgentStatusDone)
	assert.Equal(t, SubAgentStatus("failed"), SubAgentStatusFailed)
}

func TestCallStatus(t *testing.T) {
	assert.Equal(t, CallStatus("ringing"), CallStatusRinging)
	assert.Equal(t, CallStatus("active"), CallStatusActive)
	assert.Equal(t, CallStatus("on_hold"), CallStatusOnHold)
	assert.Equal(t, CallStatus("ended"), CallStatusEnded)
}

func TestToneType(t *testing.T) {
	assert.Equal(t, ToneType("thinking"), ToneThinking)
	assert.Equal(t, ToneType("tools"), ToneTools)
	assert.Equal(t, ToneType("encoding"), ToneEncoding)
}

func TestKnowledge(t *testing.T) {
	knowledge := Knowledge{
		ID:        "1",
		UserID:    "user1",
		Content:   "Likes pizza",
		Embedding: []float64{0.1, 0.2, 0.3},
	}

	assert.Equal(t, "1", knowledge.ID)
	assert.Equal(t, "user1", knowledge.UserID)
	assert.Equal(t, "Likes pizza", knowledge.Content)
	assert.Len(t, knowledge.Embedding, 3)
}

func TestGraph(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "1", Label: "user:1", Type: "user", Value: "John"},
			{ID: "2", Label: "fact:1", Type: "fact", Value: "Likes pizza"},
		},
		Edges: []GraphEdge{
			{Source: "1", Target: "2", Label: "HAS_FACT"},
		},
	}

	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.Edges, 1)
	assert.Equal(t, "HAS_FACT", graph.Edges[0].Label)
}

func TestPairing(t *testing.T) {
	pairing := Pairing{
		Code:           "abc123",
		ChannelID:      "channel1",
		PlatformUserID: "user123",
		ExpiresAt:      1700000000,
		Status:         "pending",
		CreatedAt:      1699999900,
	}

	assert.Equal(t, "abc123", pairing.Code)
	assert.Equal(t, "pending", pairing.Status)
}

func TestChannel(t *testing.T) {
	channel := Channel{
		ID:        "ch1",
		Type:      "telegram",
		Name:      "Test Channel",
		CreatedAt: 1700000000,
	}

	assert.Equal(t, "ch1", channel.ID)
	assert.Equal(t, "telegram", channel.Type)
	assert.Equal(t, "Test Channel", channel.Name)
}

func TestChannelCapabilities(t *testing.T) {
	t.Run("telegram", func(t *testing.T) {
		caps := GetCapabilitiesForType("telegram")
		assert.True(t, caps.HasVoiceMessage)
		assert.False(t, caps.HasCallStream)
		assert.True(t, caps.HasTextStream)
		assert.True(t, caps.HasMediaSupport)
	})

	t.Run("discord", func(t *testing.T) {
		caps := GetCapabilitiesForType("discord")
		assert.True(t, caps.HasVoiceMessage)
		assert.True(t, caps.HasCallStream)
		assert.True(t, caps.HasTextStream)
		assert.True(t, caps.HasMediaSupport)
	})

	t.Run("whatsapp", func(t *testing.T) {
		caps := GetCapabilitiesForType("whatsapp")
		assert.True(t, caps.HasVoiceMessage)
		assert.True(t, caps.HasCallStream)
		assert.True(t, caps.HasTextStream)
		assert.True(t, caps.HasMediaSupport)
	})

	t.Run("twilio", func(t *testing.T) {
		caps := GetCapabilitiesForType("twilio")
		assert.True(t, caps.HasVoiceMessage)
		assert.True(t, caps.HasCallStream)
		assert.True(t, caps.HasTextStream)
		assert.True(t, caps.HasMediaSupport)
	})

	t.Run("unknown", func(t *testing.T) {
		caps := GetCapabilitiesForType("unknown")
		assert.True(t, caps.HasTextStream)
		assert.False(t, caps.HasMediaSupport)
	})

	t.Run("empty", func(t *testing.T) {
		caps := GetCapabilitiesForType("")
		assert.True(t, caps.HasTextStream)
		assert.False(t, caps.HasMediaSupport)
	})
}

func TestMedia(t *testing.T) {
	media := Media{
		ChatID:      "chat123",
		URL:         "https://example.com/image.jpg",
		Caption:     "A beautiful image",
		FileName:    "image.jpg",
		ContentType: "image/jpeg",
	}

	assert.Equal(t, "chat123", media.ChatID)
	assert.Equal(t, "image.jpg", media.FileName)
}

func TestUserInfo(t *testing.T) {
	user := UserInfo{
		ID:          "user1",
		Username:    "johndoe",
		DisplayName: "John Doe",
	}

	assert.Equal(t, "user1", user.ID)
	assert.Equal(t, "johndoe", user.Username)
}

func TestVoiceCall(t *testing.T) {
	call := VoiceCall{
		ID:        "call123",
		UserID:    "user1",
		Status:    CallStatusRinging,
		StartTime: 1700000000,
	}

	assert.Equal(t, "call123", call.ID)
	assert.Equal(t, CallStatusRinging, call.Status)
}

func TestVoiceStream(t *testing.T) {
	stream := VoiceStream{
		Input:     make(chan AudioChunk),
		Output:    make(chan AudioChunk),
		Interrupt: make(chan struct{}),
		Mute:      make(chan bool),
	}

	assert.NotNil(t, stream.Input)
	assert.NotNil(t, stream.Output)
	assert.NotNil(t, stream.Interrupt)
	assert.NotNil(t, stream.Mute)
}

func TestAudioChunk(t *testing.T) {
	chunk := AudioChunk{
		Data: []byte{0x00, 0x01, 0x02},
	}

	assert.Len(t, chunk.Data, 3)
}

func TestTerminalOptions(t *testing.T) {
	opts := TerminalOptions{
		Env:        []string{"VAR=value"},
		WorkingDir: "/home/user",
		Timeout:    30,
	}

	assert.Equal(t, "VAR=value", opts.Env[0])
	assert.Equal(t, "/home/user", opts.WorkingDir)
	assert.Equal(t, 30, opts.Timeout)
}

func TestTerminalOutput(t *testing.T) {
	output := TerminalOutput{
		Stdout:   "Hello World",
		Stderr:   "",
		ExitCode: 0,
	}

	assert.Equal(t, "Hello World", output.Stdout)
	assert.Equal(t, 0, output.ExitCode)
}

func TestProcessStatus(t *testing.T) {
	assert.Equal(t, ProcessStatus("running"), ProcessStatusRunning)
	assert.Equal(t, ProcessStatus("done"), ProcessStatusDone)
	assert.Equal(t, ProcessStatus("failed"), ProcessStatusFailed)
	assert.Equal(t, ProcessStatus("killed"), ProcessStatusKilled)
}

func TestGraphNode(t *testing.T) {
	node := GraphNode{
		ID:         "node1",
		Label:      "person:1",
		Type:       "person",
		Value:      "John",
		Properties: map[string]string{"age": "30"},
	}

	assert.Equal(t, "node1", node.ID)
	assert.Equal(t, "person", node.Type)
	assert.Equal(t, "30", node.Properties["age"])
}

func TestGraphEdge(t *testing.T) {
	edge := GraphEdge{
		Source: "node1",
		Target: "node2",
		Label:  "KNOWS",
	}

	assert.Equal(t, "node1", edge.Source)
	assert.Equal(t, "KNOWS", edge.Label)
}

func TestGraphResult(t *testing.T) {
	result := GraphResult{
		Data: []map[string]interface{}{
			{"name": "John"},
		},
		Errors: []error{nil},
	}

	assert.Len(t, result.Data, 1)
	assert.Len(t, result.Errors, 1)
}

func TestTool(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: &FunctionTool{
			Name:        "send_message",
			Description: "Send a message",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]string{"type": "string"},
				},
			},
		},
	}

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "send_message", tool.Function.Name)
}

func TestFunctionTool(t *testing.T) {
	ft := FunctionTool{
		Name:        "test",
		Description: "Test function",
		Parameters:  map[string]interface{}{},
	}

	assert.Equal(t, "test", ft.Name)
	assert.NotNil(t, ft.Parameters)
}

func TestFunctionCall(t *testing.T) {
	fc := FunctionCall{
		Name:      "test_func",
		Arguments: `{"arg1": "value1"}`,
	}

	assert.Equal(t, "test_func", fc.Name)
	assert.Contains(t, fc.Arguments, "arg1")
}

func TestChatResponseWithAudio(t *testing.T) {
	resp := ChatResponseWithAudio{
		Content:    "Hello",
		AudioData:  []byte{0x00, 0x01},
		StopReason: "stop",
	}

	assert.Equal(t, "Hello", resp.Content)
	assert.Len(t, resp.AudioData, 2)
}
