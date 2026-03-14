package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// contextKey is used to avoid collisions in context keys.
type contextKey string

const contextKeyUserID = contextKey("user_id")

// ContextKeyUserID is the exported context key used to pass the current user's
// channel ID through the request context into tool Execute calls.
// Use this key (not a raw string) in context.WithValue / ctx.Value to avoid
// type-mismatch issues between packages.
const ContextKeyUserID = contextKeyUserID

// ContextKeyUserDisplayName is the exported context key used to pass the current
// user's human-readable display name through the request context into tool Execute
// calls. Used by AddMemoryTool to label the user node in the memory graph.
const ContextKeyUserDisplayName = contextKey("user_display_name")

// ContextKeyChannelID is the exported context key used to pass the current
// conversation's platform channel ID through the request context into tool Execute
// calls. Used by SendFileTool to fall back to the conversation channel when no
// explicit channel is provided.
const ContextKeyChannelID = contextKey("channel_id")

// ContextKeyChannelType is the exported context key used to pass the current
// conversation's platform channel type (e.g. "telegram", "discord") through the
// request context into tool Execute calls. Used by SendFileTool to fall back to
// the conversation channel type when no explicit channel_type is provided.
const ContextKeyChannelType = contextKey("channel_type")

type InternalTools struct {
	Messaging           MessagingService
	LastChannelResolver LastChannelResolver // optional: used when send_message gets user_id
	Memory              MemoryService
	Terminal            TerminalService
	Browser             BrowserService
	Cron                CronService
	Tasks               TaskService
	SubAgents           SubAgentService
	Filesystem          FilesystemService
	Conversations       ConversationService
	Skills              SkillsService
	// ConfigPath is the absolute path to the application configuration file.
	// Terminal tools use this to deny any command that references that file.
	ConfigPath string
}

// SkillsService provides access to the workspace skill library at runtime.
// The LLM receives a lightweight catalog in its system prompt and can call
// load_skill / read_skill_file to fetch full instructions on demand.
type SkillsService interface {
	// ListEnabledSkills returns a compact catalog of all enabled skills
	// (name + description). Used to build the catalog injected into the prompt.
	ListEnabledSkills() ([]SkillCatalogEntry, error)
	// LoadSkill returns the full SKILL.md content for the named skill.
	LoadSkill(name string) (string, error)
	// ReadSkillFile returns the content of a supporting file inside a skill
	// directory (e.g. references/guide.md).
	ReadSkillFile(name, filename string) (string, error)
}

// SkillCatalogEntry is a lightweight descriptor used to build the skill catalog
// injected into the LLM system prompt.
type SkillCatalogEntry struct {
	Name        string
	Description string
}

// ConversationService provides read access to conversation history.
// Intended for use by memory consolidation sub-agents that need to
// iterate over past conversations and extract durable facts.
type ConversationService interface {
	ListConversations(ctx context.Context) ([]ConversationSummary, error)
	GetConversationMessages(ctx context.Context, conversationID string, limit int) ([]ConversationMessage, error)
}

// ConversationSummary is a brief description of a stored conversation.
type ConversationSummary struct {
	ID              string
	ChannelID       string
	ChannelName     string
	ParticipantID   string
	ParticipantName string
	LastMessageAt   string
	MessageCount    int
}

// ConversationMessage is a single message within a conversation.
type ConversationMessage struct {
	Role      string
	Content   string
	Timestamp string
}

type FilesystemService interface {
	ReadFile(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, path, content string) error
	EditFile(ctx context.Context, path, oldContent, newContent string) error
	ListContent(ctx context.Context, path string) ([]FileEntry, error)
}

type FileEntry struct {
	Name  string
	Path  string
	IsDir bool
	Size  int64
	Mode  string
}

type MessagingService interface {
	// SendMessage sends content to channelID. channelType routes to the correct
	// adapter (telegram, discord, etc.); if empty, uses the first available adapter.
	SendMessage(ctx context.Context, channelType, channelID, content string) error
	SendFile(ctx context.Context, channelID, filePath string) error
	// SendMedia sends a media object (URL/file) to the given chat. Optional
	// helper to allow media sending when channel type is known.
	SendMedia(ctx context.Context, media *ports.Media) error
}

// LastChannelResolver returns the last channel used with a user (for send_message routing).
type LastChannelResolver interface {
	GetLastChannelForUser(ctx context.Context, userID string) (channelType, platformChannelID string, err error)
}

type MemoryService interface {
	// AddKnowledge stores a new fact for a user.
	// label is a short descriptive keyword (e.g. "Electronica").
	// relation is the semantic edge label (e.g. "LIKES"). Both have sensible defaults.
	AddKnowledge(ctx context.Context, userID, content, label, relation string) error
	SearchMemory(ctx context.Context, userID, query string) (string, error)
	SetUserProperty(ctx context.Context, userID, key, value string) error
	EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error
	DeleteMemoryNode(ctx context.Context, userID, nodeID string) error
	// UpdateUserLabel sets the human-readable display label on the user node.
	UpdateUserLabel(ctx context.Context, userID, displayName string) error
	// AddRelation creates an edge between two existing entities in the memory graph.
	AddRelation(ctx context.Context, from, to, relType string) error
	// QueryGraph executes an arbitrary Cypher query against the memory graph and
	// returns the raw results. Useful for advanced operations such as deleting
	// or transforming relationships when the backend supports Cypher.
	QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error)
}

type TerminalService interface {
	Execute(ctx context.Context, cmd string, opts ...ports.TerminalOption) (ports.TerminalOutput, error)
	Spawn(ctx context.Context, cmd string) (ports.PtySession, error)
	ListProcesses(ctx context.Context) ([]ports.BackgroundProcess, error)
	GetProcess(ctx context.Context, id string) (ports.BackgroundProcess, error)
	KillProcess(ctx context.Context, pid int) error
}

type BrowserService interface {
	Fetch(ctx context.Context, sessionID, url string) (string, error)
	Screenshot(ctx context.Context, sessionID string) ([]byte, error)
	Click(ctx context.Context, sessionID, selector string) error
	FillInput(ctx context.Context, sessionID, selector, text string) error
}

type CronService interface {
	Schedule(ctx context.Context, name, schedule, prompt, channelID string) error
	List(ctx context.Context) ([]CronJobInfo, error)
	Delete(ctx context.Context, id string) error
}

type CronJobInfo struct {
	ID        string
	Name      string
	Schedule  string
	Enabled   bool
	ChannelID string
}

type TaskService interface {
	Add(ctx context.Context, prompt, schedule string) (string, error)
	Done(ctx context.Context, id string) error
	List(ctx context.Context) ([]TaskInfo, error)
}

type TaskInfo struct {
	ID       string
	Prompt   string
	Schedule string // cron expression; empty means one-shot
	Status   string
}

type SubAgentService interface {
	Spawn(ctx context.Context, config SubAgentConfig, task string) (SubAgent, error)
	List(ctx context.Context) ([]SubAgentInfo, error)
	Kill(ctx context.Context, id string) error
}

type SubAgentConfig struct {
	Name         string
	Model        string
	SystemPrompt string
	Timeout      int
}

type SubAgent interface {
	ID() string
	Name() string
	Status() string
	Result() string
}

type SubAgentInfo struct {
	ID     string
	Name   string
	Status string
}

type SendMessageTool struct {
	Tools InternalTools
}

func (t *SendMessageTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "send_message",
		Description: "Send a message to a user. Use user_id to send to the last channel they used with the bot, or channel+channel_type for a specific destination.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"user_id": {"type": "string", "description": "Internal user UUID — sends to the last channel they used"},
				"channel": {"type": "string", "description": "Platform channel ID (use with channel_type when not using user_id)"},
				"channel_type": {"type": "string", "description": "Platform: telegram, discord, whatsapp, twilio (required when using channel)"},
				"content": {"type": "string", "description": "Message content"}
			},
			"required": ["content"]
		}`),
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	userID, _ := params["user_id"].(string)
	channel, _ := params["channel"].(string)
	channelType, _ := params["channel_type"].(string)
	content, _ := params["content"].(string)

	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	if userID != "" {
		if t.Tools.LastChannelResolver == nil {
			return nil, fmt.Errorf("user_id not supported: no resolver configured")
		}
		ct, cid, err := t.Tools.LastChannelResolver.GetLastChannelForUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("resolve last channel for user: %w", err)
		}
		if ct == "" || cid == "" {
			// Try to resolve from context (tools invoked with current user id)
			if u, ok := ctx.Value(ContextKeyUserID).(string); ok && u != "" {
				// Treat ctx user value as platform channel id when resolver failed
				channel = u
			} else {
				// Attempt to cast resolver to full UserChannelRepositoryPort to fetch display name
				if repo, ok := t.Tools.LastChannelResolver.(ports.UserChannelRepositoryPort); ok {
					if dn, derr := repo.GetDisplayNameByUserID(ctx, userID); derr == nil && dn != "" {
						return nil, fmt.Errorf("no channel found for user %s (display_name=%s)", userID, dn)
					}
				}
				return nil, fmt.Errorf("no channel found for user %s", userID)
			}
		} else {
			channelType, channel = ct, cid
		}
	}

	if channel == "" {
		return nil, fmt.Errorf("channel is required (or use user_id to send to their last channel)")
	}
	if channelType == "" && userID == "" {
		return nil, fmt.Errorf("channel_type is required when specifying channel directly (e.g. telegram, discord)")
	}

	// If a display name for the recipient is available in context, update memory label
	if dn, ok := ctx.Value(ContextKeyUserDisplayName).(string); ok && dn != "" {
		if t.Tools.Memory != nil {
			// best-effort: update user label for routing/records
			_ = t.Tools.Memory.UpdateUserLabel(ctx, userID, dn)
		}
	}

	err := t.Tools.Messaging.SendMessage(ctx, channelType, channel, content)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "sent"}`), nil
}

type SendFileTool struct {
	Tools InternalTools
}

// UserNameResolver is an optional interface that the LastChannelResolver may
// implement to support looking up a user UUID by their display name.
type UserNameResolver interface {
	GetUserIDByName(ctx context.Context, name string) (string, error)
}

func (t *SendFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "send_file",
		Description: "Send a file to a user. user_name and channel_type are optional and default to the current conversation context. The MIME type is auto-detected so the adapter can handle it correctly (voice note, image, video, document, etc.).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"user_name": {"type": "string", "description": "Display name of the recipient user (from the users table). Optional — defaults to the current conversation user."},
				"channel_type": {"type": "string", "description": "Platform: telegram, discord, whatsapp, twilio. Optional — defaults to the current conversation channel type."},
				"file_path": {"type": "string", "description": "Absolute path to the file to send"}
			},
			"required": ["file_path"]
		}`),
	}
}

// detectMIMEType returns the MIME type for filePath by examining the file header
// and falling back to the extension. Returns "application/octet-stream" on failure.
func detectMIMEType(filePath string) string {
	// Try extension first as it is cheap and reliable for common types.
	if ext := filepath.Ext(filePath); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	// Fall back to reading the first 512 bytes for sniffing.
	f, err := os.Open(filePath)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

func (t *SendFileTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	userName, _ := params["user_name"].(string)
	channelType, _ := params["channel_type"].(string)
	filePath, _ := params["file_path"].(string)

	if filePath == "" {
		return nil, fmt.Errorf("file_path is required")
	}

	var channel string

	// Resolve channel from user_name when provided.
	if userName != "" {
		if t.Tools.LastChannelResolver == nil {
			return nil, fmt.Errorf("user_name routing not supported: no resolver configured")
		}
		// Resolve display name → internal user UUID.
		userID := ""
		if nr, ok := t.Tools.LastChannelResolver.(UserNameResolver); ok {
			var err error
			userID, err = nr.GetUserIDByName(ctx, userName)
			if err != nil {
				return nil, fmt.Errorf("look up user %q: %w", userName, err)
			}
		}
		if userID == "" {
			return nil, fmt.Errorf("no user found with name %q", userName)
		}
		// Resolve UUID → last used channel.
		ct, cid, err := t.Tools.LastChannelResolver.GetLastChannelForUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("resolve last channel for user %q: %w", userName, err)
		}
		if cid == "" {
			return nil, fmt.Errorf("no channel found for user %q — they have not interacted with the bot yet", userName)
		}
		channel = cid
		if channelType == "" {
			channelType = ct
		}
	}

	// Fall back to the current conversation channel from context when no user_name was given.
	if channel == "" {
		if cid, ok := ctx.Value(ContextKeyChannelID).(string); ok && cid != "" {
			channel = cid
		}
	}
	if channelType == "" {
		if ct, ok := ctx.Value(ContextKeyChannelType).(string); ok && ct != "" {
			channelType = ct
		}
	}

	if channel == "" {
		return nil, fmt.Errorf("recipient channel could not be determined — provide user_name or ensure the conversation context is available")
	}
	if channelType == "" {
		return nil, fmt.Errorf("channel_type could not be determined — provide it explicitly or ensure the conversation context is available")
	}

	// Auto-detect MIME type so the adapter can handle the file correctly
	// (voice note, image, video, document, etc.).
	mimeType := detectMIMEType(filePath)

	media := &ports.Media{
		ChatID:      channel,
		URL:         filePath,
		ChannelType: channelType,
		ContentType: mimeType,
		FileName:    filepath.Base(filePath),
	}
	if t.Tools.Messaging == nil {
		return nil, fmt.Errorf("messaging adapter unavailable")
	}
	if err := t.Tools.Messaging.SendMedia(ctx, media); err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "sent"}`), nil
}

type TerminalExecTool struct {
	Tools InternalTools
}

func (t *TerminalExecTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "terminal_exec",
		Description: "Execute a command synchronously (blocks until complete)",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {"type": "string", "description": "Command to execute"},
				"env": {"type": "array", "items": {"type": "string"}, "description": "Environment variables"},
				"cwd": {"type": "string", "description": "Working directory"},
				"timeout": {"type": "integer", "description": "Timeout in seconds"}
			},
			"required": ["command"]
		}`),
	}
}

// commandReferencesConfig returns true when cmd contains the config file path
// (by absolute or relative form), preventing tools from touching it.
func commandReferencesConfig(cmd, configPath string) bool {
	if configPath == "" {
		return false
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		absConfig = configPath
	}
	// Normalise separators in the command for comparison.
	normCmd := filepath.ToSlash(cmd)
	return strings.Contains(normCmd, filepath.ToSlash(absConfig)) ||
		strings.Contains(normCmd, filepath.ToSlash(configPath))
}

func (t *TerminalExecTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	cmd, _ := params["command"].(string)
	if cmd == "" {
		return nil, fmt.Errorf("command is required")
	}
	if commandReferencesConfig(cmd, t.Tools.ConfigPath) {
		return nil, fmt.Errorf("access denied: commands that reference the application configuration file are not allowed")
	}

	opts := []ports.TerminalOption{}
	if env, ok := params["env"].([]interface{}); ok {
		envStrs := make([]string, len(env))
		for i, e := range env {
			envStrs[i] = fmt.Sprintf("%v", e)
		}
		opts = append(opts, func(o *ports.TerminalOptions) { o.Env = envStrs })
	}
	if cwd, ok := params["cwd"].(string); ok {
		opts = append(opts, func(o *ports.TerminalOptions) { o.WorkingDir = cwd })
	}
	if timeout, ok := params["timeout"].(float64); ok {
		opts = append(opts, func(o *ports.TerminalOptions) { o.Timeout = int(timeout) })
	}

	output, err := t.Tools.Terminal.Execute(ctx, cmd, opts...)
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"error":    err.Error(),
			"stdout":   output.Stdout,
			"stderr":   output.Stderr,
			"exitCode": output.ExitCode,
		})
	}

	return json.Marshal(output)
}

type TerminalSpawnTool struct {
	Tools InternalTools
}

func (t *TerminalSpawnTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "terminal_spawn",
		Description: "Launch a process in background (non-blocking). Returns a process ID you can use with terminal_list_processes to check status and output. Only available to master agent.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {"type": "string", "description": "Command to execute in the background"}
			},
			"required": ["command"]
		}`),
	}
}

func (t *TerminalSpawnTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	cmd, _ := params["command"].(string)
	if cmd == "" {
		return nil, fmt.Errorf("command is required")
	}
	if commandReferencesConfig(cmd, t.Tools.ConfigPath) {
		return nil, fmt.Errorf("access denied: commands that reference the application configuration file are not allowed")
	}

	if _, err := t.Tools.Terminal.Spawn(ctx, cmd); err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"status":  "spawned",
		"message": "Process launched in background. Use terminal_list_processes to check status and output.",
	})
}

type TerminalListProcessesTool struct {
	Tools InternalTools
}

func (t *TerminalListProcessesTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "terminal_list_processes",
		Description: "List all background processes launched with terminal_spawn: id, pid, command and status. Use terminal_get_output to read a process's stdout/stderr.",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
	}
}

func (t *TerminalListProcessesTool) Execute(ctx context.Context, _ map[string]interface{}) (json.RawMessage, error) {
	if t.Tools.Terminal == nil {
		return nil, fmt.Errorf("terminal service unavailable")
	}
	procs, err := t.Tools.Terminal.ListProcesses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	type procEntry struct {
		ID      string `json:"id"`
		PID     int    `json:"pid"`
		Command string `json:"command"`
		Status  string `json:"status"`
	}

	entries := make([]procEntry, 0, len(procs))
	for _, p := range procs {
		entries = append(entries, procEntry{
			ID:      p.ID(),
			PID:     p.PID(),
			Command: p.Command(),
			Status:  string(p.Status()),
		})
	}
	return json.Marshal(entries)
}

type TerminalGetOutputTool struct {
	Tools InternalTools
}

func (t *TerminalGetOutputTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "terminal_get_output",
		Description: "Get the captured stdout/stderr of a background process by its id. Use tail to limit the number of last lines returned.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id":   {"type": "string", "description": "Process ID returned by terminal_spawn"},
				"tail": {"type": "integer", "description": "Return only the last N lines. Omit to get all output."}
			},
			"required": ["id"]
		}`),
	}
}

func (t *TerminalGetOutputTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	if t.Tools.Terminal == nil {
		return nil, fmt.Errorf("terminal service unavailable")
	}
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	proc, err := t.Tools.Terminal.GetProcess(ctx, id)
	if err != nil {
		return nil, err
	}

	output := proc.CollectedOutput()

	if tail, ok := params["tail"].(float64); ok && tail > 0 {
		lines := strings.Split(output, "\n")
		n := int(tail)
		if n < len(lines) {
			lines = lines[len(lines)-n:]
		}
		output = strings.Join(lines, "\n")
	}

	return json.Marshal(map[string]interface{}{
		"id":     proc.ID(),
		"status": string(proc.Status()),
		"output": output,
	})
}

type AddMemoryTool struct {
	Tools InternalTools
}

func (t *AddMemoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "add_memory",
		Description: "Store a fact about the user that links them to a thing or place (not the user's own profile fields). " +
			"Use a short label for the fact and a semantic relation from user to that fact. " +
			"Examples: User lives in Valencia → content='User lives in Valencia', label='Valencia', relation='LIVES_IN'. " +
			"User likes electronic music → content='User loves electronic music', label='Electronica', relation='LIKES'. " +
			"User works as X → label='Software Engineer', relation='IS'. " +
			"For the user's own attributes (name, phone, birthday) use set_user_property instead. " +
			"For a relation between two users (e.g. friends) use add_user_relation instead.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content":  {"type": "string", "description": "Full sentence describing what to remember (e.g. 'User lives in Valencia', 'User loves electronic music')"},
				"label":    {"type": "string", "description": "Short keyword for the fact (e.g. 'Valencia', 'Electronica', 'Software Engineer'). Defaults to first words of content."},
				"relation": {"type": "string", "description": "Edge from user to this fact: LIVES_IN, LIKES, IS, WORKS_AT, WORKS_AS, PREFERS, HAS_FACT, etc. Defaults to HAS_FACT."}
			},
			"required": ["content"]
		}`),
	}
}

func (t *AddMemoryTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	content, _ := params["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	label, _ := params["label"].(string)
	relation, _ := params["relation"].(string)

	// Prefer the display name (users.name) as the memory key when available.
	var userKey string
	if dn, ok := ctx.Value(ContextKeyUserDisplayName).(string); ok && dn != "" {
		userKey = dn
	} else if u, ok := ctx.Value(contextKeyUserID).(string); ok {
		userKey = u
	}

	// Keep the user node label in sync with the display name whenever we add memory.
	if t.Tools.Memory != nil {
		if displayName, ok := ctx.Value(ContextKeyUserDisplayName).(string); ok && displayName != "" {
			_ = t.Tools.Memory.UpdateUserLabel(ctx, userKey, displayName)
		}
	}

	err := t.Tools.Memory.AddKnowledge(ctx, userKey, content, label, relation)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "added"}`), nil
}

// AddUserRelationTool creates a semantic relation between two users (by user name/id)
// and allows supplying a `purpose`/description for the relation. The tool will
// check for an existing identical relation before creating a new one.
type AddUserRelationTool struct {
	Tools InternalTools
}

func (t *AddUserRelationTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "add_user_relation",
		Description: "Create a relation between two users (e.g. this user is friends with that user). " +
			"Use when the user says they know someone, are friends with someone, or any link between two people. " +
			"Examples: relation='FRIEND_OF', relation='KNOWS', relation='COLLEAGUE_OF'. " +
			"Optional purpose records a short description of the relation.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"from_user": {"type": "string", "description": "Display name or id of the first user (source)"},
				"to_user": {"type": "string", "description": "Display name or id of the second user (target)"},
				"relation": {"type": "string", "description": "Relation type: FRIEND_OF, KNOWS, COLLEAGUE_OF, FAMILY_OF, etc."},
				"purpose": {"type": "string", "description": "Optional short description (e.g. 'met at work', 'childhood friends')"}
			},
			"required": ["from_user", "to_user", "relation"]
		}`),
	}
}

func (t *AddUserRelationTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	from, _ := params["from_user"].(string)
	to, _ := params["to_user"].(string)
	relation, _ := params["relation"].(string)
	purpose, _ := params["purpose"].(string)

	if from == "" || to == "" || relation == "" {
		return nil, fmt.Errorf("from_user, to_user and relation are required")
	}

	if t.Tools.Memory == nil {
		return nil, fmt.Errorf("memory backend not configured")
	}

	// Use a per-user marker fact to avoid creating duplicate logical relations
	// across backends. The marker label is deterministic and stored as a fact
	// on the `from` user so backends without Cypher support (GML) can still
	// deduplicate.
	markerLabel := fmt.Sprintf("relation:%s:%s", to, relation)
	// Search only in the `from` user's facts for an existing marker.
	if t.Tools.Memory != nil {
		if markerRes, err := t.Tools.Memory.SearchMemory(ctx, from, markerLabel); err == nil {
			if strings.Contains(markerRes, "[node_id:") {
				return json.RawMessage(`{"status": "exists"}`), nil
			}
		}

		// Create the relation (adapters will create nodes if needed).
		if err := t.Tools.Memory.AddRelation(ctx, from, to, relation); err != nil {
			return nil, err
		}

		// Persist the purpose as a user-scoped fact so it is searchable and
		// visible in the UI; use the markerLabel so future calls detect it.
		if purpose != "" {
			if err := t.Tools.Memory.AddKnowledge(ctx, from, purpose, markerLabel, "RELATION_PURPOSE"); err != nil {
				// best-effort: relation was created, but recording purpose failed
				return json.RawMessage(`{"status": "created_with_purpose_failed"}`), nil
			}
		} else {
			// create an empty marker fact to record existence
			_ = t.Tools.Memory.AddKnowledge(ctx, from, "", markerLabel, "RELATION_MARKER")
		}
	}

	return json.RawMessage(`{"status": "created"}`), nil
}

type SetUserPropertyTool struct {
	Tools InternalTools
}

func (t *SetUserPropertyTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "set_user_property",
		Description: "Persist a structured key/value attribute on the current user's profile. " +
			"Use this for the user's own attributes: real name, phone, birthday, language, timezone, etc. " +
			"Call it proactively whenever you learn something concrete about the user. " +
			"Examples: key='real_name' value='Alice'; key='phone' value='+34 600 000 000'; " +
			"key='birthday' value='15 March'; key='preferred_language' value='Spanish'; key='timezone' value='Europe/Madrid'; " +
			"key='occupation' value='software engineer'. Use snake_case for keys.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"key": {"type": "string", "description": "Attribute name, snake_case (e.g. 'real_name', 'phone', 'birthday', 'preferred_language', 'timezone', 'occupation')"},
				"value": {"type": "string", "description": "Attribute value as a string"}
			},
			"required": ["key", "value"]
		}`),
	}
}

func (t *SetUserPropertyTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	key, _ := params["key"].(string)
	value, _ := params["value"].(string)
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}

	// Prefer the display name (users.name) as the memory key when available.
	var userKey string
	if dn, ok := ctx.Value(ContextKeyUserDisplayName).(string); ok && dn != "" {
		userKey = dn
	} else if u, ok := ctx.Value(contextKeyUserID).(string); ok {
		userKey = u
	}

	if err := t.Tools.Memory.SetUserProperty(ctx, userKey, key, value); err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "property_set"}`), nil
}

type SearchMemoryTool struct {
	Tools InternalTools
}

func (t *SearchMemoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "search_memory",
		Description: "Search the current user's long-term memory for stored facts and preferences. " +
			"Pass a topic keyword or short phrase (e.g. 'music', 'work', 'location', 'preferences'). " +
			"Returns matching facts previously saved with add_memory. " +
			"Use this before making assumptions about the user — check memory first.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "Topic or keyword to look up (e.g. 'music', 'work', 'language', 'hobbies')"}
			},
			"required": ["query"]
		}`),
	}
}

func (t *SearchMemoryTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Prefer display name as memory key when available.
	var userKey string
	if dn, ok := ctx.Value(ContextKeyUserDisplayName).(string); ok && dn != "" {
		userKey = dn
	} else if u, ok := ctx.Value(contextKeyUserID).(string); ok {
		userKey = u
	}

	result, err := t.Tools.Memory.SearchMemory(ctx, userKey, query)
	if err != nil {
		return nil, err
	}
	if result == "" {
		result = "No memories found for this query."
	}

	return json.RawMessage(fmt.Sprintf("%q", result)), nil
}

type ScheduleCronTool struct {
	Tools InternalTools
}

func (t *ScheduleCronTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "schedule_cron",
		Description: "Create or modify a cron job",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Job name"},
				"schedule": {"type": "string", "description": "Cron expression (e.g., '0 8 * * *')"},
				"prompt": {"type": "string", "description": "Prompt to execute"},
				"channel": {"type": "string", "description": "Channel ID for announcements"}
			},
			"required": ["name", "schedule", "prompt", "channel"]
		}`),
	}
}

func (t *ScheduleCronTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	name, _ := params["name"].(string)
	schedule, _ := params["schedule"].(string)
	prompt, _ := params["prompt"].(string)
	channel, _ := params["channel"].(string)

	if name == "" || schedule == "" || prompt == "" || channel == "" {
		return nil, fmt.Errorf("name, schedule, prompt, and channel are required")
	}

	err := t.Tools.Cron.Schedule(ctx, name, schedule, prompt, channel)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "scheduled"}`), nil
}

type BrowserFetchTool struct {
	Tools InternalTools
}

func (t *BrowserFetchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "browser_fetch",
		Description: "Navigate to a URL and get the page content",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"session_id": {"type": "string", "description": "Session ID"},
				"url": {"type": "string", "description": "URL to navigate to"}
			},
			"required": ["session_id", "url"]
		}`),
	}
}

func (t *BrowserFetchTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	sessionID, _ := params["session_id"].(string)
	url, _ := params["url"].(string)

	if sessionID == "" || url == "" {
		return nil, fmt.Errorf("session_id and url are required")
	}

	content, err := t.Tools.Browser.Fetch(ctx, sessionID, url)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"content": content,
		"url":     url,
	})
}

type BrowserScreenshotTool struct {
	Tools InternalTools
}

func (t *BrowserScreenshotTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "browser_screenshot",
		Description: "Take a screenshot of the current page",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"session_id": {"type": "string", "description": "Session ID"}
			},
			"required": ["session_id"]
		}`),
	}
}

func (t *BrowserScreenshotTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	sessionID, _ := params["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	data, err := t.Tools.Browser.Screenshot(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Best-effort: persist the screenshot to disk so the agent can send it back
	// to the user via send_file. We write under a deterministic workspace path
	// that tools and dashboards can discover later.
	var filePath string
	if t.Tools.Filesystem != nil {
		ts := time.Now().UTC().Format("20060102-150405")
		safeSession := strings.ReplaceAll(sessionID, string(filepath.Separator), "_")
		if safeSession == "" {
			safeSession = "session"
		}
		path := filepath.Join("workspace", "screenshots", fmt.Sprintf("%s-%s.png", safeSession, ts))
		if err := t.Tools.Filesystem.WriteFile(ctx, path, string(data)); err == nil {
			filePath = path
		}
	}

	// Return only metadata and file_path; do NOT include the raw base64 screenshot.
	// Full-page PNGs can be several MB; including them in the tool result causes
	// oversized requests to Ollama and 500 Internal Server Error. The agent can
	// use send_file with file_path to share the image with the user.
	result := map[string]interface{}{
		"session_id": sessionID,
		"bytes":      len(data),
	}
	if filePath != "" {
		result["file_path"] = filePath
		result["message"] = "Screenshot saved. Use send_file with file_path to share it with the user."
	} else {
		result["message"] = "Screenshot taken but could not be saved to workspace (no filesystem). Image data omitted to avoid oversized response."
	}

	return json.Marshal(result)
}

type BrowserClickTool struct {
	Tools InternalTools
}

func (t *BrowserClickTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "browser_click",
		Description: "Click a DOM element by CSS selector",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"session_id": {"type": "string"},
				"selector": {"type": "string"}
			},
			"required": ["session_id", "selector"]
		}`),
	}
}

func (t *BrowserClickTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	sessionID, _ := params["session_id"].(string)
	selector, _ := params["selector"].(string)
	if sessionID == "" || selector == "" {
		return nil, fmt.Errorf("session_id and selector are required")
	}
	if err := t.Tools.Browser.Click(ctx, sessionID, selector); err != nil {
		return nil, err
	}
	return json.RawMessage(`{"status": "clicked"}`), nil
}

type BrowserFillInputTool struct {
	Tools InternalTools
}

func (t *BrowserFillInputTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "browser_fill_input",
		Description: "Fill a form input field",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"session_id": {"type": "string", "description": "Session ID"},
				"selector": {"type": "string", "description": "CSS selector for input"},
				"text": {"type": "string", "description": "Text to fill"}
			},
			"required": ["session_id", "selector", "text"]
		}`),
	}
}

func (t *BrowserFillInputTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	sessionID, _ := params["session_id"].(string)
	selector, _ := params["selector"].(string)
	text, _ := params["text"].(string)

	if sessionID == "" || selector == "" || text == "" {
		return nil, fmt.Errorf("session_id, selector, and text are required")
	}

	err := t.Tools.Browser.FillInput(ctx, sessionID, selector, text)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "filled"}`), nil
}

type SubAgentSpawnTool struct {
	Tools InternalTools
}

func (t *SubAgentSpawnTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "subagent_spawn",
		Description: "Spawn a sub-agent with a specific task. Only available to master agent.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Sub-agent name"},
				"model": {"type": "string", "description": "Model to use"},
				"system_prompt": {"type": "string", "description": "System prompt"},
				"task": {"type": "string", "description": "Task to execute"},
				"timeout": {"type": "integer", "description": "Timeout in seconds"}
			},
			"required": ["name", "task"]
		}`),
	}
}

func (t *SubAgentSpawnTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	name, _ := params["name"].(string)
	model, _ := params["model"].(string)
	systemPrompt, _ := params["system_prompt"].(string)
	task, _ := params["task"].(string)
	timeout, _ := params["timeout"].(float64)

	if name == "" || task == "" {
		return nil, fmt.Errorf("name and task are required")
	}

	config := SubAgentConfig{
		Name:         name,
		Model:        model,
		SystemPrompt: systemPrompt,
		Timeout:      int(timeout),
	}

	agent, err := t.Tools.SubAgents.Spawn(ctx, config, task)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"id":     agent.ID(),
		"name":   agent.Name(),
		"status": agent.Status(),
	})
}

type TaskAddTool struct {
	Tools InternalTools
}

func (t *TaskAddTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "task_add",
		Description: "Add a task to the heartbeat task queue",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"prompt": {"type": "string", "description": "Task prompt"},
				"schedule": {"type": "string", "description": "Cron expression (e.g. \"0 8 * * *\"); empty means one-shot"}
			},
			"required": ["prompt"]
		}`),
	}
}

func (t *TaskAddTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	prompt, _ := params["prompt"].(string)
	schedule, _ := params["schedule"].(string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	id, err := t.Tools.Tasks.Add(ctx, prompt, schedule)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"id":       id,
		"status":   "added",
		"schedule": schedule,
	})
}

type TaskDoneTool struct {
	Tools InternalTools
}

func (t *TaskDoneTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "task_done",
		Description: "Mark a task as completed",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "Task ID"}
			},
			"required": ["id"]
		}`),
	}
}

func (t *TaskDoneTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	err := t.Tools.Tasks.Done(ctx, id)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "done"}`), nil
}

type TaskListTool struct {
	Tools InternalTools
}

func (t *TaskListTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "task_list",
		Description: "List pending tasks in the heartbeat queue",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	}
}

func (t *TaskListTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	tasks, err := t.Tools.Tasks.List(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(tasks)
}

type ReadFileTool struct {
	Tools InternalTools
}

func (t *ReadFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "read_file",
		Description: "Read the contents of a file from the filesystem",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path to the file"}
			},
			"required": ["path"]
		}`),
	}
}

// openlobsterBlocks is the wire format used to pass multimodal content blocks
// through the json.RawMessage tool result boundary. The handler layer detects
// this key and converts it into ports.ContentBlock slices on the message.
type openlobsterBlocks struct {
	Blocks []openlobsterBlock `json:"_openlobster_blocks"`
}

type openlobsterBlock struct {
	Type     string `json:"type"`
	MIMEType string `json:"mime_type,omitempty"`
	Data     string `json:"data,omitempty"` // base64-encoded
	Text     string `json:"text,omitempty"`
}

func (t *ReadFileTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	mimeType := detectMIMEType(path)

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		payload := openlobsterBlocks{
			Blocks: []openlobsterBlock{
				{
					Type:     "image",
					MIMEType: mimeType,
					Data:     base64.StdEncoding.EncodeToString(raw),
					Text:     fmt.Sprintf("Image file: %s (%s, %d bytes)", path, mimeType, len(raw)),
				},
			},
		}
		data, _ := json.Marshal(payload)
		return data, nil

	case strings.HasPrefix(mimeType, "audio/"):
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		payload := openlobsterBlocks{
			Blocks: []openlobsterBlock{
				{
					Type:     "audio",
					MIMEType: mimeType,
					Data:     base64.StdEncoding.EncodeToString(raw),
					Text:     fmt.Sprintf("Audio file: %s (%s, %d bytes)", path, mimeType, len(raw)),
				},
			},
		}
		data, _ := json.Marshal(payload)
		return data, nil

	default:
		content, err := t.Tools.Filesystem.ReadFile(ctx, path)
		if err != nil {
			return nil, err
		}
		result := map[string]string{"content": content}
		data, _ := json.Marshal(result)
		return data, nil
	}
}

type WriteFileTool struct {
	Tools InternalTools
}

func (t *WriteFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write_file",
		Description: "Write content to a file (creates or overwrites)",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path to the file"},
				"content": {"type": "string", "description": "Content to write to the file"}
			},
			"required": ["path", "content"]
		}`),
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)

	if path == "" || content == "" {
		return nil, fmt.Errorf("path and content are required")
	}

	err := t.Tools.Filesystem.WriteFile(ctx, path, content)
	if err != nil {
		return nil, err
	}

	result := map[string]string{"status": "written", "path": path}
	data, _ := json.Marshal(result)
	return data, nil
}

type EditFileTool struct {
	Tools InternalTools
}

func (t *EditFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "edit_file",
		Description: "Edit a file by replacing specific content",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path to the file"},
				"old_content": {"type": "string", "description": "Content to find and replace"},
				"new_content": {"type": "string", "description": "Replacement content"}
			},
			"required": ["path", "old_content", "new_content"]
		}`),
	}
}

func (t *EditFileTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	path, _ := params["path"].(string)
	oldContent, _ := params["old_content"].(string)
	newContent, _ := params["new_content"].(string)

	if path == "" || oldContent == "" {
		return nil, fmt.Errorf("path and old_content are required")
	}

	err := t.Tools.Filesystem.EditFile(ctx, path, oldContent, newContent)
	if err != nil {
		return nil, err
	}

	result := map[string]string{"status": "edited", "path": path}
	data, _ := json.Marshal(result)
	return data, nil
}

type EditMemoryNodeTool struct {
	Tools InternalTools
}

func (t *EditMemoryNodeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "edit_memory_node",
		Description: "Edit the text value of an existing memory node (fact) by its node ID. Use search_memory first to discover the node ID. Only fact nodes belonging to the current user can be edited.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"node_id": {"type": "string", "description": "The node ID of the memory entry to edit (obtained from search_memory results)"},
				"new_value": {"type": "string", "description": "The new text value for the memory node"}
			},
			"required": ["node_id", "new_value"]
		}`),
	}
}

func (t *EditMemoryNodeTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	nodeID, _ := params["node_id"].(string)
	newValue, _ := params["new_value"].(string)
	if nodeID == "" || newValue == "" {
		return nil, fmt.Errorf("node_id and new_value are required")
	}

	var userID string
	if u, ok := ctx.Value(contextKeyUserID).(string); ok {
		userID = u
	}

	if err := t.Tools.Memory.EditMemoryNode(ctx, userID, nodeID, newValue); err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "edited"}`), nil
}

type DeleteMemoryNodeTool struct {
	Tools InternalTools
}

func (t *DeleteMemoryNodeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_memory_node",
		Description: "Delete a memory node (fact) by its node ID. Use search_memory first to discover the node ID. Only fact nodes belonging to the current user can be deleted.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"node_id": {"type": "string", "description": "The node ID of the memory entry to delete (obtained from search_memory results)"}
			},
			"required": ["node_id"]
		}`),
	}
}

func (t *DeleteMemoryNodeTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	nodeID, _ := params["node_id"].(string)
	if nodeID == "" {
		return nil, fmt.Errorf("node_id is required")
	}

	var userID string
	if u, ok := ctx.Value(contextKeyUserID).(string); ok {
		userID = u
	}

	if err := t.Tools.Memory.DeleteMemoryNode(ctx, userID, nodeID); err != nil {
		return nil, err
	}

	return json.RawMessage(`{"status": "deleted"}`), nil
}

type ListContentTool struct {
	Tools InternalTools
}

func (t *ListContentTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_content",
		Description: "List files and directories at a given path",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Directory path to list"}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ListContentTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	entries, err := t.Tools.Filesystem.ListContent(ctx, path)
	if err != nil {
		return nil, err
	}

	return json.Marshal(entries)
}

// ListConversationsTool returns a list of all stored conversations.
// Intended for memory consolidation sub-agents.
type ListConversationsTool struct{ Tools InternalTools }

func (t *ListConversationsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_conversations",
		Description: "List all stored conversations. Use this to discover which conversations exist before reading their messages. Only available to sub-agents performing memory consolidation.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
	}
}

func (t *ListConversationsTool) Execute(ctx context.Context, _ map[string]interface{}) (json.RawMessage, error) {
	if t.Tools.Conversations == nil {
		return nil, fmt.Errorf("conversation service not configured")
	}
	conversations, err := t.Tools.Conversations.ListConversations(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(conversations)
}

// GetConversationMessagesTool returns the messages from a specific conversation.
// Intended for memory consolidation sub-agents.
type GetConversationMessagesTool struct{ Tools InternalTools }

func (t *GetConversationMessagesTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_conversation_messages",
		Description: "Get messages from a specific conversation by ID. Returns up to `limit` most recent messages (default 100, max 500). Use list_conversations first to obtain conversation IDs.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"conversation_id": {"type": "string", "description": "The conversation ID to read"},
				"limit": {"type": "integer", "description": "Maximum number of messages to return (default 100, max 500)"}
			},
			"required": ["conversation_id"]
		}`),
	}
}

func (t *GetConversationMessagesTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	if t.Tools.Conversations == nil {
		return nil, fmt.Errorf("conversation service not configured")
	}
	conversationID, _ := params["conversation_id"].(string)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	limit := 100
	if v, ok := params["limit"].(float64); ok && v > 0 {
		limit = int(v)
		if limit > 500 {
			limit = 500
		}
	}
	messages, err := t.Tools.Conversations.GetConversationMessages(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}
	return json.Marshal(messages)
}

// LoadSkillTool lets the LLM fetch the full SKILL.md instructions for a skill
// on demand, following the progressive-disclosure pattern (catalog in prompt →
// load full content only when needed).
type LoadSkillTool struct {
	Tools InternalTools
}

func (t *LoadSkillTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "load_skill",
		Description: "Load the full instructions of an agent skill by name. " +
			"Call this when you need to apply a skill to the current task. " +
			"The skill catalog is listed in the system prompt under ## Skills.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Exact skill name as it appears in the catalog"}
			},
			"required": ["name"]
		}`),
	}
}

func (t *LoadSkillTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	if t.Tools.Skills == nil {
		return nil, fmt.Errorf("skills service not configured")
	}
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	content, err := t.Tools.Skills.LoadSkill(name)
	if err != nil {
		// Return a structured error so the LLM can self-correct with a valid name.
		catalog, _ := t.Tools.Skills.ListEnabledSkills()
		names := make([]string, 0, len(catalog))
		for _, e := range catalog {
			names = append(names, e.Name)
		}
		return json.Marshal(map[string]interface{}{
			"error":            fmt.Sprintf("skill %q not found", name),
			"available_skills": names,
		})
	}
	return json.Marshal(map[string]interface{}{
		"skill_name":   name,
		"instructions": content,
	})
}

// ReadSkillFileTool lets the LLM retrieve a specific supporting file from a
// skill directory (e.g. references/guide.md) when deeper detail is needed.
type ReadSkillFileTool struct {
	Tools InternalTools
}

func (t *ReadSkillFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "read_skill_file",
		Description: "Read a supporting file from a skill's directory. " +
			"Use this after load_skill when you need additional reference material.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name":     {"type": "string", "description": "Skill name"},
				"filename": {"type": "string", "description": "Relative path inside the skill directory (e.g. 'references/guide.md')"}
			},
			"required": ["name", "filename"]
		}`),
	}
}

func (t *ReadSkillFileTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	if t.Tools.Skills == nil {
		return nil, fmt.Errorf("skills service not configured")
	}
	name, _ := params["name"].(string)
	filename, _ := params["filename"].(string)
	if name == "" || filename == "" {
		return nil, fmt.Errorf("name and filename are required")
	}
	content, err := t.Tools.Skills.ReadSkillFile(name, filename)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{
		"skill_name": name,
		"filename":   filename,
		"content":    content,
	})
}

// CapabilityForTool returns the capability name that gates the given tool, or ""
// if the tool is always available (e.g. send_message, send_file). Used to filter
// tools when the agent has capabilities disabled in global settings.
func CapabilityForTool(name string) string {
	if idx := strings.Index(name, ":"); idx >= 0 {
		return "mcp" // MCP tools use qualified names like "server:tool"
	}
	m := map[string]string{
		"browser_fetch": "browser", "browser_screenshot": "browser",
		"browser_click": "browser", "browser_fill_input": "browser",
		"terminal_exec": "terminal", "terminal_spawn": "terminal",
		"terminal_list_processes": "terminal", "terminal_get_output": "terminal",
		"add_memory": "memory", "search_memory": "memory",
		"set_user_property": "memory", "edit_memory_node": "memory",
		"delete_memory_node": "memory",
		"add_relation":       "memory", "delete_relation": "memory", "update_relation": "memory",
		"subagent_spawn": "subagents",
		"read_file":      "filesystem", "write_file": "filesystem",
		"edit_file": "filesystem", "list_content": "filesystem",
		"list_conversations": "sessions", "get_conversation_messages": "sessions",
	}
	if cap, ok := m[name]; ok {
		return cap
	}
	return "" // send_message, send_file, schedule_cron, task_*, load_skill, read_skill_file
}

// BuiltinToolNames returns the canonical list of all built-in internal tool names.
// This is the single source of truth used by the permission system to enumerate
// which tools exist independently of whether the registry has been initialised.
func BuiltinToolNames() []string {
	return []string{
		"send_message", "send_file",
		"terminal_exec", "terminal_spawn", "terminal_list_processes", "terminal_get_output", "terminal_list_processes",
		"add_memory", "search_memory", "set_user_property", "edit_memory_node", "delete_memory_node",
		"add_relation", "delete_relation", "update_relation",
		"add_user_relation",
		"schedule_cron",
		"browser_fetch", "browser_screenshot", "browser_click", "browser_fill_input",
		"subagent_spawn", "task_add", "task_done", "task_list",
		"read_file", "write_file", "edit_file", "list_content",
		"list_conversations", "get_conversation_messages",
		"load_skill", "read_skill_file",
	}
}

func RegisterAllInternalTools(reg *ToolRegistry, tools InternalTools) {
	reg.RegisterInternal("send_message", &SendMessageTool{Tools: tools})
	reg.RegisterInternal("send_file", &SendFileTool{Tools: tools})
	reg.RegisterInternal("terminal_exec", &TerminalExecTool{Tools: tools})
	reg.RegisterInternal("terminal_spawn", &TerminalSpawnTool{Tools: tools})
	reg.RegisterInternal("terminal_list_processes", &TerminalListProcessesTool{Tools: tools})
	reg.RegisterInternal("terminal_get_output", &TerminalGetOutputTool{Tools: tools})
	reg.RegisterInternal("add_memory", &AddMemoryTool{Tools: tools})
	reg.RegisterInternal("add_user_relation", &AddUserRelationTool{Tools: tools})
	reg.RegisterInternal("search_memory", &SearchMemoryTool{Tools: tools})
	reg.RegisterInternal("set_user_property", &SetUserPropertyTool{Tools: tools})
	reg.RegisterInternal("edit_memory_node", &EditMemoryNodeTool{Tools: tools})
	reg.RegisterInternal("delete_memory_node", &DeleteMemoryNodeTool{Tools: tools})
	reg.RegisterInternal("schedule_cron", &ScheduleCronTool{Tools: tools})
	reg.RegisterInternal("browser_fetch", &BrowserFetchTool{Tools: tools})
	reg.RegisterInternal("browser_screenshot", &BrowserScreenshotTool{Tools: tools})
	reg.RegisterInternal("browser_click", &BrowserClickTool{Tools: tools})
	reg.RegisterInternal("browser_fill_input", &BrowserFillInputTool{Tools: tools})
	reg.RegisterInternal("subagent_spawn", &SubAgentSpawnTool{Tools: tools})
	reg.RegisterInternal("task_add", &TaskAddTool{Tools: tools})
	reg.RegisterInternal("task_done", &TaskDoneTool{Tools: tools})
	reg.RegisterInternal("task_list", &TaskListTool{Tools: tools})
	reg.RegisterInternal("read_file", &ReadFileTool{Tools: tools})
	reg.RegisterInternal("write_file", &WriteFileTool{Tools: tools})
	reg.RegisterInternal("edit_file", &EditFileTool{Tools: tools})
	reg.RegisterInternal("list_content", &ListContentTool{Tools: tools})
	reg.RegisterInternal("list_conversations", &ListConversationsTool{Tools: tools})
	reg.RegisterInternal("get_conversation_messages", &GetConversationMessagesTool{Tools: tools})
	if tools.Skills != nil {
		reg.RegisterInternal("load_skill", &LoadSkillTool{Tools: tools})
		reg.RegisterInternal("read_skill_file", &ReadSkillFileTool{Tools: tools})
	}
}
