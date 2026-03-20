package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockMessagingService struct {
	mock.Mock
}

func (m *MockMessagingService) SendMessage(ctx context.Context, channelType, channelID, content string) error {
	args := m.Called(ctx, channelType, channelID, content)
	return args.Error(0)
}


func (m *MockMessagingService) SendMedia(ctx context.Context, media *ports.Media) error {
	args := m.Called(ctx, media)
	return args.Error(0)
}

type MockMessageLogService struct {
	mock.Mock
}

func (m *MockMessageLogService) SaveOutbound(ctx context.Context, channelType, channelID, content string) error {
	args := m.Called(ctx, channelType, channelID, content)
	return args.Error(0)
}

type MockMemoryService struct {
	mock.Mock
}

func (m *MockMemoryService) AddKnowledge(ctx context.Context, userID, content, label, relation, entityType string) error {
	args := m.Called(ctx, userID, content, label, relation, entityType)
	return args.Error(0)
}
func (m *MockMemoryService) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	args := m.Called(ctx, userID, displayName)
	return args.Error(0)
}

func (m *MockMemoryService) SearchMemory(ctx context.Context, userID, query string) (string, error) {
	args := m.Called(ctx, userID, query)
	return args.String(0), args.Error(1)
}

func (m *MockMemoryService) SetUserProperty(ctx context.Context, userID, key, value string) error {
	args := m.Called(ctx, userID, key, value)
	return args.Error(0)
}

func (m *MockMemoryService) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	args := m.Called(ctx, userID, nodeID, newValue)
	return args.Error(0)
}

func (m *MockMemoryService) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	args := m.Called(ctx, userID, nodeID)
	return args.Error(0)
}

func (m *MockMemoryService) AddRelation(ctx context.Context, from, to, relType string) error {
	args := m.Called(ctx, from, to, relType)
	return args.Error(0)
}

func (m *MockMemoryService) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	args := m.Called(ctx, cypher)
	return args.Get(0).(ports.GraphResult), args.Error(1)
}

type MockTerminalService struct {
	mock.Mock
}

func (m *MockTerminalService) Execute(ctx context.Context, cmd string, opts ...ports.TerminalOption) (ports.TerminalOutput, error) {
	args := m.Called(ctx, cmd, opts)
	return args.Get(0).(ports.TerminalOutput), args.Error(1)
}

func (m *MockTerminalService) Spawn(ctx context.Context, cmd string) (ports.PtySession, error) {
	args := m.Called(ctx, cmd)
	return args.Get(0).(ports.PtySession), args.Error(1)
}

func (m *MockTerminalService) ListProcesses(ctx context.Context) ([]ports.BackgroundProcess, error) {
	args := m.Called(ctx)
	return args.Get(0).([]ports.BackgroundProcess), args.Error(1)
}

func (m *MockTerminalService) GetProcess(ctx context.Context, id string) (ports.BackgroundProcess, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(ports.BackgroundProcess), args.Error(1)
}

func (m *MockTerminalService) KillProcess(ctx context.Context, pid int) error {
	args := m.Called(ctx, pid)
	return args.Error(0)
}

type MockBrowserService struct {
	mock.Mock
}

func (m *MockBrowserService) Fetch(ctx context.Context, sessionID, url string) (string, error) {
	args := m.Called(ctx, sessionID, url)
	return args.String(0), args.Error(1)
}

func (m *MockBrowserService) Screenshot(ctx context.Context, sessionID string) ([]byte, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockBrowserService) Click(ctx context.Context, sessionID, selector string) error {
	args := m.Called(ctx, sessionID, selector)
	return args.Error(0)
}

func (m *MockBrowserService) FillInput(ctx context.Context, sessionID, selector, text string) error {
	args := m.Called(ctx, sessionID, selector, text)
	return args.Error(0)
}

type MockCronService struct {
	mock.Mock
}

func (m *MockCronService) Schedule(ctx context.Context, name, schedule, prompt, channelID string) error {
	args := m.Called(ctx, name, schedule, prompt, channelID)
	return args.Error(0)
}

func (m *MockCronService) List(ctx context.Context) ([]CronJobInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]CronJobInfo), args.Error(1)
}

func (m *MockCronService) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockTaskService struct {
	mock.Mock
}

func (m *MockTaskService) Add(ctx context.Context, prompt, schedule string) (string, error) {
	args := m.Called(ctx, prompt, schedule)
	return args.String(0), args.Error(1)
}

func (m *MockTaskService) Done(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskService) List(ctx context.Context) ([]TaskInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]TaskInfo), args.Error(1)
}

type MockSubAgentService struct {
	mock.Mock
}

type MockSubAgent struct {
	mock.Mock
}

func (m *MockSubAgent) ID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSubAgent) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSubAgent) Status() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSubAgent) Result() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSubAgentService) Spawn(ctx context.Context, config SubAgentConfig, task string) (SubAgent, error) {
	args := m.Called(ctx, config, task)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SubAgent), args.Error(1)
}

func (m *MockSubAgentService) List(ctx context.Context) ([]SubAgentInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]SubAgentInfo), args.Error(1)
}

func (m *MockSubAgentService) Kill(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockFilesystemService struct {
	mock.Mock
}

func (m *MockFilesystemService) ReadFile(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	return args.String(0), args.Error(1)
}

func (m *MockFilesystemService) ReadFileBytes(ctx context.Context, path string) ([]byte, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFilesystemService) WriteFile(ctx context.Context, path, content string) error {
	args := m.Called(ctx, path, content)
	return args.Error(0)
}

func (m *MockFilesystemService) WriteBytes(ctx context.Context, path string, data []byte) error {
	args := m.Called(ctx, path, data)
	return args.Error(0)
}

func (m *MockFilesystemService) EditFile(ctx context.Context, path, oldContent, newContent string) error {
	args := m.Called(ctx, path, oldContent, newContent)
	return args.Error(0)
}

func (m *MockFilesystemService) ListContent(ctx context.Context, path string) ([]FileEntry, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]FileEntry), args.Error(1)
}

type MockConversationService struct {
	mock.Mock
}

func (m *MockConversationService) ListConversations(ctx context.Context) ([]ConversationSummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ConversationSummary), args.Error(1)
}

func (m *MockConversationService) GetConversationMessages(ctx context.Context, conversationID string, limit int) ([]ConversationMessage, error) {
	args := m.Called(ctx, conversationID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ConversationMessage), args.Error(1)
}

type MockSkillsService struct {
	mock.Mock
}

func (m *MockSkillsService) ListEnabledSkills() ([]SkillCatalogEntry, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]SkillCatalogEntry), args.Error(1)
}

func (m *MockSkillsService) LoadSkill(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockSkillsService) ReadSkillFile(name, filename string) (string, error) {
	args := m.Called(name, filename)
	return args.String(0), args.Error(1)
}

func TestListConversationsTool_Execute_Success(t *testing.T) {
	mockConv := new(MockConversationService)
	mockConv.On("ListConversations", mock.Anything).Return([]ConversationSummary{{ID: "c1"}}, nil)
	tool := &ListConversationsTool{Tools: InternalTools{Conversations: mockConv}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	assert.NoError(t, err)
	var convs []ConversationSummary
	json.Unmarshal(result, &convs)
	assert.Len(t, convs, 1)
	assert.Equal(t, "c1", convs[0].ID)
	mockConv.AssertExpectations(t)
}

func TestListConversationsTool_Execute_NilService(t *testing.T) {
	tool := &ListConversationsTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestGetConversationMessagesTool_Execute_Success(t *testing.T) {
	mockConv := new(MockConversationService)
	mockConv.On("GetConversationMessages", mock.Anything, "conv1", 100).Return([]ConversationMessage{{Role: "user", Content: "hi"}}, nil)
	tool := &GetConversationMessagesTool{Tools: InternalTools{Conversations: mockConv}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"conversation_id": "conv1"})
	assert.NoError(t, err)
	var msgs []ConversationMessage
	json.Unmarshal(result, &msgs)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0].Role)
	mockConv.AssertExpectations(t)
}

func TestGetConversationMessagesTool_Execute_WithLimit(t *testing.T) {
	mockConv := new(MockConversationService)
	mockConv.On("GetConversationMessages", mock.Anything, "c1", 50).Return([]ConversationMessage{}, nil)
	tool := &GetConversationMessagesTool{Tools: InternalTools{Conversations: mockConv}}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"conversation_id": "c1", "limit": float64(50)})
	assert.NoError(t, err)
	mockConv.AssertExpectations(t)
}

func TestGetConversationMessagesTool_Execute_MissingConvID(t *testing.T) {
	tool := &GetConversationMessagesTool{Tools: InternalTools{Conversations: new(MockConversationService)}}
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	assert.Error(t, err)
}

func TestLoadSkillTool_Execute_Success(t *testing.T) {
	mockSkills := new(MockSkillsService)
	mockSkills.On("LoadSkill", "codegen").Return("# Codegen skill", nil)
	tool := &LoadSkillTool{Tools: InternalTools{Skills: mockSkills}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"name": "codegen"})
	assert.NoError(t, err)
	var m map[string]interface{}
	json.Unmarshal(result, &m)
	assert.Equal(t, "codegen", m["skill_name"])
	assert.Equal(t, "# Codegen skill", m["instructions"])
	mockSkills.AssertExpectations(t)
}

func TestLoadSkillTool_Execute_NotFound(t *testing.T) {
	mockSkills := new(MockSkillsService)
	mockSkills.On("LoadSkill", "missing").Return("", assert.AnError)
	mockSkills.On("ListEnabledSkills").Return([]SkillCatalogEntry{{Name: "other"}}, nil)
	tool := &LoadSkillTool{Tools: InternalTools{Skills: mockSkills}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"name": "missing"})
	assert.NoError(t, err)
	var m map[string]interface{}
	json.Unmarshal(result, &m)
	assert.Contains(t, m["error"], "not found")
	assert.Contains(t, m["available_skills"], "other")
}

func TestLoadSkillTool_Execute_NilService(t *testing.T) {
	tool := &LoadSkillTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"name": "x"})
	assert.Error(t, err)
}

func TestReadSkillFileTool_Execute_Success(t *testing.T) {
	mockSkills := new(MockSkillsService)
	mockSkills.On("ReadSkillFile", "codegen", "refs/guide.md").Return("guide content", nil)
	tool := &ReadSkillFileTool{Tools: InternalTools{Skills: mockSkills}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"name": "codegen", "filename": "refs/guide.md"})
	assert.NoError(t, err)
	var m map[string]interface{}
	json.Unmarshal(result, &m)
	assert.Equal(t, "guide content", m["content"])
	mockSkills.AssertExpectations(t)
}

func TestReadSkillFileTool_Execute_MissingParams(t *testing.T) {
	tool := &ReadSkillFileTool{Tools: InternalTools{Skills: new(MockSkillsService)}}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"name": "x"})
	assert.Error(t, err)
}

func TestBuiltinToolNames(t *testing.T) {
	names := BuiltinToolNames()
	assert.Contains(t, names, "send_message")
	assert.Contains(t, names, "load_skill")
	assert.Greater(t, len(names), 20)
}

func TestReadFileTool_Definition(t *testing.T) {
	tool := &ReadFileTool{}
	assert.Equal(t, "read_file", tool.Definition().Name)
}

func TestReadFileTool_Execute_Success(t *testing.T) {
	mockFS := new(MockFilesystemService)
	mockFS.On("ReadFile", mock.Anything, "/tmp/x").Return("file content", nil)

	tool := &ReadFileTool{Tools: InternalTools{Filesystem: mockFS}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"path": "/tmp/x"})
	assert.NoError(t, err)
	var m map[string]string
	json.Unmarshal(result, &m)
	assert.Equal(t, "file content", m["content"])
	mockFS.AssertExpectations(t)
}

func TestReadFileTool_Execute_MissingPath(t *testing.T) {
	tool := &ReadFileTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	assert.Error(t, err)
}

func TestReadFileTool_Execute_Error(t *testing.T) {
	mockFS := new(MockFilesystemService)
	mockFS.On("ReadFile", mock.Anything, "/x").Return("", assert.AnError)
	tool := &ReadFileTool{Tools: InternalTools{Filesystem: mockFS}}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"path": "/x"})
	assert.Error(t, err)
}

func TestWriteFileTool_Definition(t *testing.T) {
	tool := &WriteFileTool{}
	assert.Equal(t, "write_file", tool.Definition().Name)
}

func TestWriteFileTool_Execute_Success(t *testing.T) {
	mockFS := new(MockFilesystemService)
	mockFS.On("WriteFile", mock.Anything, "/f", "hello").Return(nil)

	tool := &WriteFileTool{Tools: InternalTools{Filesystem: mockFS}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"path": "/f", "content": "hello"})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "written")
	mockFS.AssertExpectations(t)
}

func TestWriteFileTool_Execute_MissingParams(t *testing.T) {
	tool := &WriteFileTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"path": "/f"})
	assert.Error(t, err)
}

func TestEditFileTool_Execute_Success(t *testing.T) {
	mockFS := new(MockFilesystemService)
	mockFS.On("EditFile", mock.Anything, "/f", "old", "new").Return(nil)

	tool := &EditFileTool{Tools: InternalTools{Filesystem: mockFS}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": "/f", "old_content": "old", "new_content": "new",
	})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "edited")
	mockFS.AssertExpectations(t)
}

func TestEditFileTool_Execute_MissingParams(t *testing.T) {
	tool := &EditFileTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"path": "/f"})
	assert.Error(t, err)
}

func TestEditMemoryNodeTool_Execute_Success(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("EditMemoryNode", mock.Anything, "", "n1", "new val").Return(nil)
	tool := &EditMemoryNodeTool{Tools: InternalTools{Memory: mockMem}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"node_id": "n1", "new_value": "new val",
	})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "edited")
	mockMem.AssertExpectations(t)
}

func TestEditMemoryNodeTool_Execute_WithUserContext(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("EditMemoryNode", mock.Anything, "user1", "n1", "val").Return(nil)
	tool := &EditMemoryNodeTool{Tools: InternalTools{Memory: mockMem}}
	ctx := context.WithValue(context.Background(), ContextKeyUserID, "user1")
	result, err := tool.Execute(ctx, map[string]interface{}{"node_id": "n1", "new_value": "val"})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "edited")
	mockMem.AssertExpectations(t)
}

func TestDeleteMemoryNodeTool_Execute_Success(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("DeleteMemoryNode", mock.Anything, "", "n1").Return(nil)
	tool := &DeleteMemoryNodeTool{Tools: InternalTools{Memory: mockMem}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"node_id": "n1"})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "deleted")
	mockMem.AssertExpectations(t)
}

func TestDeleteMemoryNodeTool_Execute_MissingNodeID(t *testing.T) {
	tool := &DeleteMemoryNodeTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	assert.Error(t, err)
}

func TestListContentTool_Execute_Success(t *testing.T) {
	mockFS := new(MockFilesystemService)
	mockFS.On("ListContent", mock.Anything, "/dir").Return([]FileEntry{{Name: "a", Path: "/dir/a", IsDir: false}}, nil)
	tool := &ListContentTool{Tools: InternalTools{Filesystem: mockFS}}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"path": "/dir"})
	assert.NoError(t, err)
	var entries []FileEntry
	json.Unmarshal(result, &entries)
	assert.Len(t, entries, 1)
	assert.Equal(t, "a", entries[0].Name)
	mockFS.AssertExpectations(t)
}

func TestListContentTool_Execute_MissingPath(t *testing.T) {
	tool := &ListContentTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	assert.Error(t, err)
}

func TestSendMessageTool_Definition(t *testing.T) {
	tool := &SendMessageTool{}
	def := tool.Definition()

	assert.Equal(t, "send_message", def.Name)
	assert.Contains(t, def.Description, "Send a message")
	assert.NotNil(t, def.InputSchema)
}

func TestSendMessageTool_Execute_Success(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMessage", mock.Anything, "telegram", "channel123", "Hello").Return(nil)

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging: mockMsg,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"channel":      "channel123",
		"channel_type": "telegram",
		"content":      "Hello",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
}

func TestSendMessageTool_Execute_PersistsOutboundMessage(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockLog := new(MockMessageLogService)
	mockMsg.On("SendMessage", mock.Anything, "telegram", "channel123", "Hello").Return(nil)
	mockLog.On("SaveOutbound", mock.Anything, "telegram", "channel123", "Hello").Return(nil)

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging:  mockMsg,
			MessageLog: mockLog,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"channel":      "channel123",
		"channel_type": "telegram",
		"content":      "Hello",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
	mockLog.AssertExpectations(t)
}

func TestSendMessageTool_Execute_PersistenceFailureReturnsWarning(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockLog := new(MockMessageLogService)
	mockMsg.On("SendMessage", mock.Anything, "telegram", "channel123", "Hello").Return(nil)
	mockLog.On("SaveOutbound", mock.Anything, "telegram", "channel123", "Hello").Return(assert.AnError)

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging:  mockMsg,
			MessageLog: mockLog,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"channel":      "channel123",
		"channel_type": "telegram",
		"content":      "Hello",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "\"persisted\":false")
	assert.Contains(t, string(result), "warning")
	mockMsg.AssertExpectations(t)
	mockLog.AssertExpectations(t)
}

func TestSendMessageTool_Execute_MissingParams(t *testing.T) {
	tool := &SendMessageTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content")
}

func TestSendMessageTool_Execute_NoRecipient(t *testing.T) {
	tool := &SendMessageTool{Tools: InternalTools{Messaging: new(MockMessagingService)}}
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"content": "hello",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one recipient")
}

func TestSendMessageTool_Execute_ChannelWithoutChannelType_ReturnsError(t *testing.T) {
	tool := &SendMessageTool{
		Tools: InternalTools{Messaging: new(MockMessagingService)},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"channel": "channel123",
		"content": "Hello",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "together")
}

func TestSendMessageTool_Execute_Error(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMessage", mock.Anything, "telegram", "channel123", "Hello").Return(assert.AnError)

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging: mockMsg,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"channel":      "channel123",
		"channel_type": "telegram",
		"content":      "Hello",
	})

	assert.Error(t, err)
	mockMsg.AssertExpectations(t)
}

func TestSendMessageTool_Execute_WithUserName(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMessage", mock.Anything, "telegram", "tg-12345", "Hi Alice").Return(nil)

	resolver := &mockUserNameResolver{
		getUserIDByName: func(ctx context.Context, name string) (string, error) {
			if name == "Alice" {
				return "uuid-alice", nil
			}
			return "", nil
		},
		mockLastChannelResolver: mockLastChannelResolver{
			getLastChannel: func(ctx context.Context, userID string) (string, string, error) {
				if userID == "uuid-alice" {
					return "telegram", "tg-12345", nil
				}
				return "", "", nil
			},
		},
	}

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging:           mockMsg,
			LastChannelResolver: resolver,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"user_name": "Alice",
		"content":   "Hi Alice",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
}

func TestSendMessageTool_Execute_WithStoredUsername(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMessage", mock.Anything, "telegram", "555666777", "hey").Return(nil)

	repo := &sendMessageFullRepoMock{
		resolveByUsername: func(ctx context.Context, username, platform string) (string, string, error) {
			if (username == "bob" || username == "@bob") && platform == "" {
				return "telegram", "555666777", nil
			}
			return "", "", nil
		},
	}

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging:           mockMsg,
			LastChannelResolver: repo,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"username": "@bob",
		"content":  "hey",
	})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
}

func TestSendMessageTool_Execute_WithStoredUsernameDiscord(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMessage", mock.Anything, "discord", "snowflake-99", "hi").Return(nil)

	repo := &sendMessageFullRepoMock{
		resolveByUsername: func(ctx context.Context, username, platform string) (string, string, error) {
			if username == "carol" && platform == "discord" {
				return "discord", "snowflake-99", nil
			}
			return "", "", nil
		},
	}

	tool := &SendMessageTool{
		Tools: InternalTools{
			Messaging:           mockMsg,
			LastChannelResolver: repo,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"username":          "carol",
		"username_platform": "discord",
		"content":           "hi",
	})
	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
}

// sendMessageFullRepoMock implements ports.UserChannelRepositoryPort for send_message tests.
type sendMessageFullRepoMock struct {
	getUserIDByName   func(ctx context.Context, name string) (string, error)
	getLastChannel    func(ctx context.Context, userID string) (string, string, error)
	resolveByUsername func(ctx context.Context, username, platform string) (string, string, error)
}

func (m *sendMessageFullRepoMock) ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error) {
	return false, nil
}
func (m *sendMessageFullRepoMock) GetUserIDByPlatformUserID(ctx context.Context, platformUserID string) (string, error) {
	return "", nil
}
func (m *sendMessageFullRepoMock) GetDisplayNameByPlatformUserID(ctx context.Context, platformUserID string) (string, error) {
	return "", nil
}
func (m *sendMessageFullRepoMock) GetDisplayNameByUserID(ctx context.Context, userID string) (string, error) {
	return "", nil
}
func (m *sendMessageFullRepoMock) Create(ctx context.Context, userID, channelType, platformUserID, username string) error {
	return nil
}
func (m *sendMessageFullRepoMock) GetLastChannelForUser(ctx context.Context, userID string) (string, string, error) {
	if m.getLastChannel != nil {
		return m.getLastChannel(ctx, userID)
	}
	return "", "", nil
}
func (m *sendMessageFullRepoMock) GetUserIDByName(ctx context.Context, name string) (string, error) {
	if m.getUserIDByName != nil {
		return m.getUserIDByName(ctx, name)
	}
	return "", nil
}
func (m *sendMessageFullRepoMock) ResolveChannelByStoredUsername(ctx context.Context, username, platform string) (string, string, error) {
	if m.resolveByUsername != nil {
		return m.resolveByUsername(ctx, username, platform)
	}
	return "", "", nil
}
func (m *sendMessageFullRepoMock) UpdateLastSeen(ctx context.Context, channelType, platformUserID string) error {
	return nil
}

type mockLastChannelResolver struct {
	getLastChannel func(ctx context.Context, userID string) (channelType, platformChannelID string, err error)
}

func (m *mockLastChannelResolver) GetLastChannelForUser(ctx context.Context, userID string) (string, string, error) {
	if m.getLastChannel != nil {
		return m.getLastChannel(ctx, userID)
	}
	return "", "", nil
}

// mockUserNameResolver extends mockLastChannelResolver with GetUserIDByName.
type mockUserNameResolver struct {
	mockLastChannelResolver
	getUserIDByName func(ctx context.Context, name string) (string, error)
}

func (m *mockUserNameResolver) GetUserIDByName(ctx context.Context, name string) (string, error) {
	if m.getUserIDByName != nil {
		return m.getUserIDByName(ctx, name)
	}
	return "", nil
}

func TestSendFileTool_Definition(t *testing.T) {
	tool := &SendFileTool{}
	def := tool.Definition()

	assert.Equal(t, "send_file", def.Name)
	assert.Contains(t, def.Description, "file")
}

func TestSendFileTool_Execute_Success_FromContext(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMedia", mock.Anything, mock.MatchedBy(func(m *ports.Media) bool {
		return m.ChatID == "channel123" && m.URL == "/tmp/test.txt" && m.ChannelType == "telegram"
	})).Return(nil)

	tool := &SendFileTool{
		Tools: InternalTools{
			Messaging: mockMsg,
		},
	}

	ctx := context.WithValue(context.Background(), ContextKeyChannelID, "channel123")
	ctx = context.WithValue(ctx, ContextKeyChannelType, "telegram")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "/tmp/test.txt",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
}

func TestSendFileTool_Execute_Success_FromUserName(t *testing.T) {
	mockMsg := new(MockMessagingService)
	mockMsg.On("SendMedia", mock.Anything, mock.MatchedBy(func(m *ports.Media) bool {
		return m.ChatID == "tg-99" && m.ChannelType == "telegram"
	})).Return(nil)

	resolver := &mockUserNameResolver{
		getUserIDByName: func(ctx context.Context, name string) (string, error) {
			if name == "alice" {
				return "uuid-alice", nil
			}
			return "", nil
		},
		mockLastChannelResolver: mockLastChannelResolver{
			getLastChannel: func(ctx context.Context, userID string) (string, string, error) {
				if userID == "uuid-alice" {
					return "telegram", "tg-99", nil
				}
				return "", "", nil
			},
		},
	}

	tool := &SendFileTool{
		Tools: InternalTools{
			Messaging:           mockMsg,
			LastChannelResolver: resolver,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"user_name": "alice",
		"file_path": "/tmp/test.txt",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "sent")
	mockMsg.AssertExpectations(t)
}

func TestSendFileTool_Execute_MissingParams(t *testing.T) {
	tool := &SendFileTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestTerminalExecTool_Definition(t *testing.T) {
	tool := &TerminalExecTool{}
	def := tool.Definition()

	assert.Equal(t, "terminal_exec", def.Name)
	assert.Contains(t, def.Description, "synchronously")
}

func TestTerminalExecTool_Execute_Success(t *testing.T) {
	mockTerm := new(MockTerminalService)
	mockTerm.On("Execute", mock.Anything, "ls -la", mock.Anything).Return(ports.TerminalOutput{
		Stdout:   "total 0",
		Stderr:   "",
		ExitCode: 0,
	}, nil)

	tool := &TerminalExecTool{
		Tools: InternalTools{
			Terminal: mockTerm,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "ls -la",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "total 0")
	mockTerm.AssertExpectations(t)
}

func TestTerminalExecTool_Execute_MissingCommand(t *testing.T) {
	tool := &TerminalExecTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestTerminalExecTool_Execute_WithOptions(t *testing.T) {
	mockTerm := new(MockTerminalService)
	mockTerm.On("Execute", mock.Anything, "echo $VAR", mock.Anything).Return(ports.TerminalOutput{
		Stdout:   "value",
		Stderr:   "",
		ExitCode: 0,
	}, nil)

	tool := &TerminalExecTool{
		Tools: InternalTools{
			Terminal: mockTerm,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo $VAR",
		"env":     []interface{}{"VAR=value"},
		"cwd":     "/tmp",
		"timeout": float64(30),
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "value")
	mockTerm.AssertExpectations(t)
}

func TestTerminalExecTool_Execute_Error(t *testing.T) {
	mockTerm := new(MockTerminalService)
	mockTerm.On("Execute", mock.Anything, "false", mock.Anything).Return(ports.TerminalOutput{
		Stdout:   "",
		Stderr:   "error",
		ExitCode: 1,
	}, assert.AnError)

	tool := &TerminalExecTool{
		Tools: InternalTools{
			Terminal: mockTerm,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "false",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "error")
	assert.Contains(t, string(result), "exitCode")
	mockTerm.AssertExpectations(t)
}

func TestTerminalSpawnTool_Definition(t *testing.T) {
	tool := &TerminalSpawnTool{}
	def := tool.Definition()

	assert.Equal(t, "terminal_spawn", def.Name)
	assert.Contains(t, def.Description, "background")
}

func TestTerminalSpawnTool_Execute_Success(t *testing.T) {
	mockTerm := new(MockTerminalService)
	mockPty := new(MockPtySession)
	mockTerm.On("Spawn", mock.Anything, "node server.js").Return(mockPty, nil)

	tool := &TerminalSpawnTool{
		Tools: InternalTools{
			Terminal: mockTerm,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "node server.js",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "spawned")
	mockTerm.AssertExpectations(t)
}

func TestAddMemoryTool_Definition(t *testing.T) {
	tool := &AddMemoryTool{}
	def := tool.Definition()

	assert.Equal(t, "add_memory", def.Name)
	assert.NotEmpty(t, def.Description)
}

func TestAddMemoryTool_Execute_Success(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("AddKnowledge", mock.Anything, "", "Important info", mock.Anything, mock.Anything, "fact").Return(nil)

	tool := &AddMemoryTool{
		Tools: InternalTools{
			Memory: mockMem,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"content": "Important info",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "added")
	mockMem.AssertExpectations(t)
}

func TestAddMemoryTool_Execute_MissingContent(t *testing.T) {
	tool := &AddMemoryTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestAddMemoryTool_Execute_Error(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("AddKnowledge", mock.Anything, "", "Info", mock.Anything, mock.Anything, "fact").Return(assert.AnError)

	tool := &AddMemoryTool{
		Tools: InternalTools{
			Memory: mockMem,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"content": "Info",
	})

	assert.Error(t, err)
	mockMem.AssertExpectations(t)
}

func TestSearchMemoryTool_Definition(t *testing.T) {
	tool := &SearchMemoryTool{}
	def := tool.Definition()

	assert.Equal(t, "search_memory", def.Name)
	assert.Contains(t, def.Description, "Search")
}

func TestSearchMemoryTool_Execute_Success(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("SearchMemory", mock.Anything, "", "query").Return(`{"result": "found"}`, nil)

	tool := &SearchMemoryTool{
		Tools: InternalTools{
			Memory: mockMem,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "query",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "found")
	mockMem.AssertExpectations(t)
}

func TestSearchMemoryTool_Execute_MissingQuery(t *testing.T) {
	tool := &SearchMemoryTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestSearchMemoryTool_Execute_Error(t *testing.T) {
	mockMem := new(MockMemoryService)
	mockMem.On("SearchMemory", mock.Anything, "", "query").Return("", assert.AnError)

	tool := &SearchMemoryTool{
		Tools: InternalTools{
			Memory: mockMem,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "query",
	})

	assert.Error(t, err)
	mockMem.AssertExpectations(t)
}

func TestScheduleCronTool_Definition(t *testing.T) {
	tool := &ScheduleCronTool{}
	def := tool.Definition()

	assert.Equal(t, "schedule_cron", def.Name)
	assert.Contains(t, def.Description, "cron")
}

func TestScheduleCronTool_Execute_Success(t *testing.T) {
	mockCron := new(MockCronService)
	mockCron.On("Schedule", mock.Anything, "myjob", "0 8 * * *", "do something", "channel123").Return(nil)

	tool := &ScheduleCronTool{
		Tools: InternalTools{
			Cron: mockCron,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"name":     "myjob",
		"schedule": "0 8 * * *",
		"prompt":   "do something",
		"channel":  "channel123",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "scheduled")
	mockCron.AssertExpectations(t)
}

func TestScheduleCronTool_Execute_MissingParams(t *testing.T) {
	tool := &ScheduleCronTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"name": "job",
	})

	assert.Error(t, err)
}

func TestBrowserFetchTool_Definition(t *testing.T) {
	tool := &BrowserFetchTool{}
	def := tool.Definition()

	assert.Equal(t, "browser_fetch", def.Name)
}

func TestBrowserFetchTool_Execute_Success(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("Fetch", mock.Anything, "session1", "https://example.com").Return("page content", nil)

	tool := &BrowserFetchTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
		"url":        "https://example.com",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "page content")
	assert.Contains(t, string(result), "https://example.com")
	mockBrowser.AssertExpectations(t)
}

func TestBrowserFetchTool_Execute_MissingParams(t *testing.T) {
	tool := &BrowserFetchTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestBrowserFetchTool_Execute_Error(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("Fetch", mock.Anything, "session1", "https://example.com").Return("", assert.AnError)

	tool := &BrowserFetchTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
		"url":        "https://example.com",
	})

	assert.Error(t, err)
	mockBrowser.AssertExpectations(t)
}

func TestBrowserScreenshotTool_Definition(t *testing.T) {
	tool := &BrowserScreenshotTool{}
	def := tool.Definition()

	assert.Equal(t, "browser_screenshot", def.Name)
}

func TestBrowserScreenshotTool_Execute_Success(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("Screenshot", mock.Anything, "session1").Return([]byte("image data"), nil)

	tool := &BrowserScreenshotTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "session_id")
	assert.Contains(t, string(result), "bytes")
	assert.Contains(t, string(result), "message")
	mockBrowser.AssertExpectations(t)
}

func TestBrowserScreenshotTool_Execute_MissingSession(t *testing.T) {
	tool := &BrowserScreenshotTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestBrowserScreenshotTool_Execute_Error(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("Screenshot", mock.Anything, "session1").Return(nil, assert.AnError)

	tool := &BrowserScreenshotTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
	})

	assert.Error(t, err)
	mockBrowser.AssertExpectations(t)
}

func TestBrowserClickTool_Definition(t *testing.T) {
	tool := &BrowserClickTool{}
	def := tool.Definition()

	assert.Equal(t, "browser_click", def.Name)
}

func TestBrowserClickTool_Execute_Success(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("Click", mock.Anything, "session1", "#button").Return(nil)

	tool := &BrowserClickTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
		"selector":   "#button",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "clicked")
	mockBrowser.AssertExpectations(t)
}

func TestBrowserClickTool_Execute_MissingParams(t *testing.T) {
	tool := &BrowserClickTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestBrowserClickTool_Execute_Error(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("Click", mock.Anything, "session1", "#button").Return(assert.AnError)

	tool := &BrowserClickTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
		"selector":   "#button",
	})

	assert.Error(t, err)
	mockBrowser.AssertExpectations(t)
}

func TestBrowserFillInputTool_Definition(t *testing.T) {
	tool := &BrowserFillInputTool{}
	def := tool.Definition()

	assert.Equal(t, "browser_fill_input", def.Name)
}

func TestBrowserFillInputTool_Execute_Success(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("FillInput", mock.Anything, "session1", "#input", "text").Return(nil)

	tool := &BrowserFillInputTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
		"selector":   "#input",
		"text":       "text",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "filled")
	mockBrowser.AssertExpectations(t)
}

func TestBrowserFillInputTool_Execute_MissingParams(t *testing.T) {
	tool := &BrowserFillInputTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestBrowserFillInputTool_Execute_Error(t *testing.T) {
	mockBrowser := new(MockBrowserService)
	mockBrowser.On("FillInput", mock.Anything, "session1", "#input", "text").Return(assert.AnError)

	tool := &BrowserFillInputTool{
		Tools: InternalTools{
			Browser: mockBrowser,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"session_id": "session1",
		"selector":   "#input",
		"text":       "text",
	})

	assert.Error(t, err)
	mockBrowser.AssertExpectations(t)
}

func TestSubAgentSpawnTool_Definition(t *testing.T) {
	tool := &SubAgentSpawnTool{}
	def := tool.Definition()

	assert.Equal(t, "subagent_spawn", def.Name)
}

func TestSubAgentSpawnTool_Execute_Success(t *testing.T) {
	mockAgent := new(MockSubAgent)
	mockAgent.On("ID").Return("agent-123")
	mockAgent.On("Name").Return("worker")
	mockAgent.On("Status").Return("running")

	mockSubAgent := new(MockSubAgentService)
	mockSubAgent.On("Spawn", mock.Anything, mock.Anything, "do work").Return(mockAgent, nil)

	tool := &SubAgentSpawnTool{
		Tools: InternalTools{
			SubAgents: mockSubAgent,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"name":          "worker",
		"model":         "gpt-4",
		"system_prompt": "You are helpful",
		"task":          "do work",
		"timeout":       float64(300),
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "agent-123")
	assert.Contains(t, string(result), "worker")
	mockSubAgent.AssertExpectations(t)
}

func TestSubAgentSpawnTool_Execute_MissingParams(t *testing.T) {
	tool := &SubAgentSpawnTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestSubAgentSpawnTool_Execute_Error(t *testing.T) {
	mockSubAgent := new(MockSubAgentService)
	mockSubAgent.On("Spawn", mock.Anything, mock.Anything, "do work").Return(nil, assert.AnError)

	tool := &SubAgentSpawnTool{
		Tools: InternalTools{
			SubAgents: mockSubAgent,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"name": "worker",
		"task": "do work",
	})

	assert.Error(t, err)
	mockSubAgent.AssertExpectations(t)
}

func TestTaskAddTool_Definition(t *testing.T) {
	tool := &TaskAddTool{}
	def := tool.Definition()

	assert.Equal(t, "task_add", def.Name)
}

func TestTaskAddTool_Execute_Success(t *testing.T) {
	mockTasks := new(MockTaskService)
	mockTasks.On("Add", mock.Anything, "do something", "").Return("task-123", nil)

	tool := &TaskAddTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": "do something",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "task-123")
	assert.Contains(t, string(result), "added")
	mockTasks.AssertExpectations(t)
}

func TestTaskAddTool_Execute_Cyclic(t *testing.T) {
	mockTasks := new(MockTaskService)
	mockTasks.On("Add", mock.Anything, "do something", "0 8 * * *").Return("task-123", nil)

	tool := &TaskAddTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt":   "do something",
		"schedule": "0 8 * * *",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "0 8 * * *")
	mockTasks.AssertExpectations(t)
}

func TestTaskAddTool_Execute_NotifyUsernameAppendsInstruction(t *testing.T) {
	mockTasks := new(MockTaskService)

	notifyUser := "alice"
	notifyPlatform := "telegram"

	mockTasks.On("Add", mock.Anything, mock.MatchedBy(func(p string) bool {
		return strings.Contains(p, "When you send the final result") &&
			strings.Contains(p, "username=\""+notifyUser+"\"") &&
			strings.Contains(p, "username_platform=\""+notifyPlatform+"\"") &&
			strings.Contains(p, "Never infer the recipient")
	}), "").Return("task-123", nil)

	tool := &TaskAddTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt":               "do something",
		"schedule":             "",
		"notify_username":      notifyUser,
		"notify_channel_type": notifyPlatform,
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "task-123")
	mockTasks.AssertExpectations(t)
}

func TestTaskAddTool_Execute_MissingPrompt(t *testing.T) {
	tool := &TaskAddTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestTaskAddTool_Execute_Error(t *testing.T) {
	mockTasks := new(MockTaskService)
	mockTasks.On("Add", mock.Anything, "do something", "").Return("", assert.AnError)

	tool := &TaskAddTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": "do something",
	})

	assert.Error(t, err)
	mockTasks.AssertExpectations(t)
}

func TestTaskDoneTool_Definition(t *testing.T) {
	tool := &TaskDoneTool{}
	def := tool.Definition()

	assert.Equal(t, "task_done", def.Name)
}

func TestTaskDoneTool_Execute_Success(t *testing.T) {
	mockTasks := new(MockTaskService)
	mockTasks.On("Done", mock.Anything, "task-123").Return(nil)

	tool := &TaskDoneTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"id": "task-123",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "done")
	mockTasks.AssertExpectations(t)
}

func TestTaskDoneTool_Execute_MissingID(t *testing.T) {
	tool := &TaskDoneTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
}

func TestTaskDoneTool_Execute_Error(t *testing.T) {
	mockTasks := new(MockTaskService)
	mockTasks.On("Done", mock.Anything, "task-123").Return(assert.AnError)

	tool := &TaskDoneTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"id": "task-123",
	})

	assert.Error(t, err)
	mockTasks.AssertExpectations(t)
}

func TestTaskListTool_Definition(t *testing.T) {
	tool := &TaskListTool{}
	def := tool.Definition()

	assert.Equal(t, "task_list", def.Name)
}

func TestTaskListTool_Execute_Success(t *testing.T) {
	mockTasks := new(MockTaskService)
	mockTasks.On("List", mock.Anything).Return([]TaskInfo{
		{ID: "task-1", Prompt: "do this", Status: "pending"},
		{ID: "task-2", Prompt: "do that", Status: "running"},
	}, nil)

	tool := &TaskListTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.NoError(t, err)
	var tasks []TaskInfo
	json.Unmarshal(result, &tasks)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, "task-1", tasks[0].ID)
	mockTasks.AssertExpectations(t)
}

func TestTaskListTool_Execute_Error(t *testing.T) {
	mockTasks := new(MockTaskService)
	var emptyTasks []TaskInfo
	mockTasks.On("List", mock.Anything).Return(emptyTasks, assert.AnError)

	tool := &TaskListTool{
		Tools: InternalTools{
			Tasks: mockTasks,
		},
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	assert.Error(t, err)
	mockTasks.AssertExpectations(t)
}

func TestCronJobInfo(t *testing.T) {
	info := CronJobInfo{
		ID:        "job-1",
		Name:      "Daily Job",
		Schedule:  "0 8 * * *",
		Enabled:   true,
		ChannelID: "ch-123",
	}

	assert.Equal(t, "job-1", info.ID)
	assert.Equal(t, "Daily Job", info.Name)
	assert.True(t, info.Enabled)
}

func TestTaskInfo(t *testing.T) {
	info := TaskInfo{
		ID:       "task-1",
		Prompt:   "Do something",
		Schedule: "0 8 * * *",
		Status:   "running",
	}

	assert.Equal(t, "task-1", info.ID)
	assert.Equal(t, "0 8 * * *", info.Schedule)
	assert.Equal(t, "running", info.Status)
}

func TestSubAgentConfig(t *testing.T) {
	config := SubAgentConfig{
		Name:         "worker",
		Model:        "gpt-4",
		SystemPrompt: "You are a helper",
		Timeout:      300,
	}

	assert.Equal(t, "worker", config.Name)
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, 300, config.Timeout)
}

func TestSubAgentInfo(t *testing.T) {
	info := SubAgentInfo{
		ID:     "openlobster",
		Name:   "worker",
		Status: "running",
	}

	assert.Equal(t, "openlobster", info.ID)
	assert.Equal(t, "worker", info.Name)
}

func TestInternalTools_Interfaces(t *testing.T) {
	tools := InternalTools{
		Messaging: &MockMessagingService{},
		Memory:    &MockMemoryService{},
		Terminal:  &MockTerminalService{},
		Browser:   &MockBrowserService{},
		Cron:      &MockCronService{},
		Tasks:     &MockTaskService{},
		SubAgents: &MockSubAgentService{},
	}

	assert.NotNil(t, tools.Messaging)
	assert.NotNil(t, tools.Memory)
	assert.NotNil(t, tools.Terminal)
	assert.NotNil(t, tools.Browser)
	assert.NotNil(t, tools.Cron)
	assert.NotNil(t, tools.Tasks)
	assert.NotNil(t, tools.SubAgents)
}

func TestSplitToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		server   string
		toolName string
	}{
		{"with prefix", "server:tool", "server", "tool"},
		{"without prefix", "tool", "internal", "tool"},
		{"empty", "", "internal", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitToolName(tt.input)
			assert.Equal(t, tt.server, result[0])
			if len(result) > 1 {
				assert.Equal(t, tt.toolName, result[1])
			}
		})
	}
}

func TestCapabilityForTool(t *testing.T) {
	tests := []struct {
		name string
		cap  string
	}{
		{"send_message", ""},
		{"send_file", ""},
		{"terminal_exec", "terminal"},
		{"terminal_spawn", "terminal"},
		{"browser_fetch", "browser"},
		{"browser_screenshot", "browser"},
		{"add_memory", "memory"},
		{"search_memory", "memory"},
		{"subagent_spawn", "subagents"},
		{"read_file", "filesystem"},
		{"list_conversations", "sessions"},
		{"schedule_cron", ""},
		{"server:tool", "mcp"},
		{"fs:read", "mcp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CapabilityForTool(tt.name)
			assert.Equal(t, tt.cap, got, "CapabilityForTool(%q)", tt.name)
		})
	}
}

func TestRegisterAllInternalTools(t *testing.T) {
	registry := NewToolRegistry(true, nil)
	tools := InternalTools{
		Messaging: &MockMessagingService{},
		Memory:    &MockMemoryService{},
		Terminal:  &MockTerminalService{},
		Browser:   &MockBrowserService{},
		Cron:      &MockCronService{},
		Tasks:     &MockTaskService{},
		SubAgents: &MockSubAgentService{},
	}

	RegisterAllInternalTools(registry, tools)

	assert.True(t, registry.HasTool("send_message"))
	assert.True(t, registry.HasTool("send_file"))
	assert.True(t, registry.HasTool("terminal_exec"))
	assert.True(t, registry.HasTool("terminal_spawn"))
	assert.True(t, registry.HasTool("add_memory"))
	assert.True(t, registry.HasTool("search_memory"))
	assert.True(t, registry.HasTool("schedule_cron"))
	assert.True(t, registry.HasTool("browser_fetch"))
	assert.True(t, registry.HasTool("browser_screenshot"))
	assert.True(t, registry.HasTool("browser_click"))
	assert.True(t, registry.HasTool("browser_fill_input"))
	assert.True(t, registry.HasTool("subagent_spawn"))
	assert.True(t, registry.HasTool("task_add"))
	assert.True(t, registry.HasTool("task_done"))
	assert.True(t, registry.HasTool("task_list"))
}

type MockPtySession struct {
	mock.Mock
}

func (m *MockPtySession) Write(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockPtySession) Read() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockPtySession) Resize(cols, rows int) error {
	args := m.Called(cols, rows)
	return args.Error(0)
}

func (m *MockPtySession) Close() error {
	args := m.Called()
	return args.Error(0)
}
