package resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
)

// MessageDispatcherPort processes messages through the unified handler.
// Ensures one channel, one message, one response.
type MessageDispatcherPort interface {
	Handle(ctx context.Context, input handlers.HandleMessageInput) error
}

// Deps groups the dependencies of the GraphQL resolvers (without the dashboard orchestrator).
type Deps struct {
	AgentRegistry     *registry.AgentRegistry
	QuerySvc          *domainservices.DashboardQueryService
	CommandSvc        *domainservices.DashboardCommandService
	TaskRepo          ports.TaskRepositoryPort
	MemoryRepo        ports.MemoryPort
	MsgRepo           dto.MessageRepo
	ConvPort          dto.ConversationPort
	SkillsPort        dto.SkillsPort
	SysFilesPort      dto.SystemFilesPort
	ToolPermRepo      dto.ToolPermissionsRepo
	ToolNamesSource   dto.ToolNamesSource
	MCPServerRepo     dto.MCPServerRepo
	McpConnectPort    dto.McpConnectPort
	McpOAuthPort      dto.McpOAuthPort
	SubAgentSvc       dto.SubAgentPort
	PairingPort       dto.PairingPort
	UserRepo          dto.UserRepo
	UserChannelRepo   dto.UserChannelRepo
	MessageSender     dto.MessageSender
	MessageDispatcher MessageDispatcherPort
	EventBus          dto.EventBusPort
	AIProvider        ports.AIProviderPort
	ConfigSnapshot    *dto.AppConfigSnapshot
	ConfigPath        string
	ConfigWriter      dto.ConfigUpdatePort
}

// Agent implements agent.Provider.
// Name and Provider come from ConfigSnapshot when available so the agent query
// always returns the latest config (e.g. after wizard completion) without relying
// on AgentRegistry being updated.
func (d *Deps) Agent(ctx context.Context) *dto.AgentSnapshot {
	a := d.AgentRegistry.GetAgent()
	if a == nil {
		return &dto.AgentSnapshot{
			ID:     "agent-unknown",
			Name:   "Unknown",
			Status: "not_initialized",
		}
	}
	uptime := int64(0)
	if start := d.AgentRegistry.StartTime(); start > 0 {
		uptime = time.Now().Unix() - start
	}
	tools := len(d.AgentRegistry.GetMCPTools())
	name := a.Name
	provider := a.Provider
	aiProvider := a.AIProvider
	if d.ConfigSnapshot != nil && d.ConfigSnapshot.Agent != nil {
		if n := d.ConfigSnapshot.Agent.Name; n != "" {
			name = n
		}
		if p := d.ConfigSnapshot.Agent.Provider; p != "" {
			provider = p
			aiProvider = p
		}
	}
	return &dto.AgentSnapshot{
		ID:            a.ID,
		Name:          name,
		Version:       a.Version,
		Status:        a.Status,
		Uptime:        uptime,
		Provider:      provider,
		AIProvider:    aiProvider,
		MemoryBackend: a.MemoryBackend,
		ToolsCount:    tools,
		TasksCount:    a.TasksCount,
		Channels:      a.Channels,
	}
}

// Channels implements agent.Provider.
func (d *Deps) Channels(ctx context.Context) []dto.ChannelStatus {
	a := d.AgentRegistry.GetAgent()
	if a == nil || len(a.Channels) == 0 {
		return nil
	}
	return a.Channels
}

// Heartbeat implements agent.Provider.
func (d *Deps) Heartbeat(ctx context.Context) *dto.HeartbeatSnapshot {
	return &dto.HeartbeatSnapshot{
		Status:    "ok",
		LastCheck: time.Now().Unix(),
	}
}

// MCPTools implements agent.Provider.
func (d *Deps) MCPTools(ctx context.Context) []dto.ToolSnapshot {
	return d.AgentRegistry.GetMCPTools()
}

// SubAgents implements agent.Provider.
func (d *Deps) SubAgents(ctx context.Context) []dto.SubAgentSnapshot {
	if d.SubAgentSvc == nil {
		return nil
	}
	list, err := d.SubAgentSvc.List(ctx)
	if err != nil {
		return nil
	}
	return list
}

// Status implements agent.Provider.
func (d *Deps) Status(ctx context.Context) *dto.StatusSnapshot {
	return &dto.StatusSnapshot{
		Agent:     d.Agent(ctx),
		Health:    d.Heartbeat(ctx),
		Channels:  d.Channels(ctx),
		Tools:     d.MCPTools(ctx),
		SubAgents: d.SubAgents(ctx),
		Tasks:     d.taskList(ctx),
		Mcps:      d.AgentRegistry.GetMCPs(),
	}
}

// Metrics implements agent.Provider.
func (d *Deps) Metrics(ctx context.Context) *dto.MetricsSnapshot {
	tools := len(d.AgentRegistry.GetMCPTools())
	errorsTotal := d.AgentRegistry.ErrorsCount()
	start := d.AgentRegistry.StartTime()
	uptime := int64(0)
	if start > 0 {
		uptime = time.Now().Unix() - start
	}
	tasksPending, tasksDone, activeSessions, messagesRecv, messagesSent := d.metricsFromDB(ctx)
	memoryNodes, memoryEdges := int64(0), int64(0)
	tasksRunning := int64(0)
	if d.QuerySvc != nil && d.MemoryRepo != nil {
		if g, err := d.QuerySvc.GetUserGraph(ctx, ""); err == nil {
			memoryNodes = int64(len(g.Nodes))
			memoryEdges = int64(len(g.Edges))
		}
	}
	if d.QuerySvc != nil {
		if tasks, err := d.QuerySvc.GetTasks(ctx); err == nil {
			for _, t := range tasks {
				if t.Status == "running" {
					tasksRunning++
				}
			}
		}
	}
	return &dto.MetricsSnapshot{
		Uptime:           uptime,
		MessagesReceived: messagesRecv,
		MessagesSent:     messagesSent,
		ActiveSessions:   activeSessions,
		MemoryNodes:      memoryNodes,
		MemoryEdges:      memoryEdges,
		McpTools:         int64(tools),
		TasksPending:     tasksPending,
		TasksRunning:     tasksRunning,
		TasksDone:        tasksDone,
		ErrorsTotal:      errorsTotal,
	}
}

func (d *Deps) metricsFromDB(ctx context.Context) (tasksPending, tasksDone, activeSessions, messagesRecv, messagesSent int64) {
	if d.QuerySvc != nil {
		if tasks, err := d.QuerySvc.GetTasks(ctx); err == nil {
			for _, t := range tasks {
				switch t.Status {
				case "pending":
					tasksPending++
				case "done":
					tasksDone++
				}
			}
		}
	}
	if d.ConvPort != nil {
		if convs, err := d.ConvPort.ListConversations(); err == nil {
			activeSessions = int64(len(convs))
		}
	}
	if d.MsgRepo != nil {
		if recv, sent, err := d.MsgRepo.CountMessages(ctx); err == nil {
			messagesRecv = recv
			messagesSent = sent
		}
	}
	return
}

func (d *Deps) taskList(ctx context.Context) []dto.TaskSnapshot {
	if d.QuerySvc == nil {
		return nil
	}
	tasks, err := d.QuerySvc.GetTasks(ctx)
	if err != nil {
		return nil
	}
	return tasksToSnapshots(tasks)
}

func tasksToSnapshots(tasks []models.Task) []dto.TaskSnapshot {
	out := make([]dto.TaskSnapshot, len(tasks))
	for i, t := range tasks {
		createdAt := ""
		if !t.AddedAt.IsZero() {
			createdAt = t.AddedAt.Format(time.RFC3339)
		}
		snap := dto.TaskSnapshot{
			ID:        t.ID,
			Prompt:    t.Prompt,
			Status:    t.Status,
			Schedule:  t.Schedule,
			TaskType:  t.TaskType,
			Enabled:   t.Enabled,
			CreatedAt: createdAt,
			IsCyclic:  t.TaskType == "cyclic",
		}
		// LastRunAt: use FinishedAt when present (one-shot tasks).
		if t.FinishedAt != nil {
			snap.LastRunAt = t.FinishedAt.Format(time.RFC3339)
		}
		// NextRunAt: compute for cyclic tasks or datetime schedules.
		if t.TaskType == "cyclic" || isDatetimeSchedule(t.Schedule) {
			next := computeNextAtLocal(t)
			if !next.IsZero() {
				snap.NextRunAt = next.Format(time.RFC3339)
			}
		}
		out[i] = snap
	}
	return out
}

// computeNextAtLocal mirrors scheduler.computeNextAt for GraphQL snapshots.
func computeNextAtLocal(task models.Task) time.Time {
	switch {
	case task.Schedule == "":
		return task.AddedAt
	case isDatetimeSchedule(task.Schedule):
		t, _ := time.Parse(time.RFC3339, task.Schedule)
		return t
	default:
		return schedulerNextCronRunLocal(task.Schedule, time.Now())
	}
}

func isDatetimeSchedule(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func schedulerNextCronRunLocal(schedule string, after time.Time) time.Time {
	fields := splitCronFields(schedule)
	if len(fields) != 5 {
		return after.Add(time.Hour)
	}

	candidate := after.Truncate(time.Minute).Add(time.Minute)
	deadline := after.Add(366 * 24 * time.Hour)

	for candidate.Before(deadline) {
		if cronFieldMatches(fields[1], candidate.Hour()) &&
			cronFieldMatches(fields[0], candidate.Minute()) &&
			cronFieldMatches(fields[2], candidate.Day()) &&
			cronFieldMatches(fields[3], int(candidate.Month())) &&
			cronFieldMatches(fields[4], int(candidate.Weekday())) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}
	return after.Add(time.Hour)
}

func splitCronFields(s string) []string {
	var fields []string
	cur := ""
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			if cur != "" {
				fields = append(fields, cur)
				cur = ""
			}
		} else {
			cur += string(ch)
		}
	}
	if cur != "" {
		fields = append(fields, cur)
	}
	return fields
}

func cronFieldMatches(f string, value int) bool {
	if f == "*" {
		return true
	}
	if len(f) > 2 && f[:2] == "*/" {
		var step int
		if _, err := fmt.Sscanf(f[2:], "%d", &step); err == nil && step > 0 {
			return value%step == 0
		}
		return false
	}
	var n int
	if _, err := fmt.Sscanf(f, "%d", &n); err == nil {
		return n == value
	}
	return false
}

// SpawnSubAgent implementa agent.Provider.
func (d *Deps) SpawnSubAgent(ctx context.Context, name, model, task string) (string, error) {
	if d.SubAgentSvc == nil {
		return "", nil
	}
	return d.SubAgentSvc.Spawn(ctx, name, model, task)
}

// KillSubAgent implementa agent.Provider.
func (d *Deps) KillSubAgent(ctx context.Context, id string) error {
	if d.SubAgentSvc == nil {
		return nil
	}
	return d.SubAgentSvc.Kill(ctx, id)
}

// ─── Conversations provider ──────────────────────────────────────────────────

// Conversations implements conversations.Provider.
func (d *Deps) Conversations(ctx context.Context) ([]dto.ConversationSnapshot, error) {
	if d.ConvPort == nil {
		return nil, nil
	}
	return d.ConvPort.ListConversations()
}

// Messages implements conversations.Provider.
// Supports keyset pagination: before is the createdAt of the oldest already loaded message,
// limit controls the page size (default 50, maximum 200).
func (d *Deps) Messages(ctx context.Context, conversationID string, before *string, limit *int) ([]dto.MessageSnapshot, error) {
	if d.MsgRepo == nil || conversationID == "" {
		return nil, nil
	}
	pageSize := 50
	if limit != nil && *limit > 0 && *limit <= 200 {
		pageSize = *limit
	}
	msgs, err := d.MsgRepo.GetByConversationPaged(ctx, conversationID, before, pageSize)
	if err != nil {
		return nil, err
	}
	out := make([]dto.MessageSnapshot, 0, len(msgs))
	for _, m := range msgs {
		// Map attachments when present. Do NOT expose URLs to attachment bytes;
		// the frontend should only indicate presence and show filenames/captions.
		attSnapshots := make([]dto.AttachmentSnapshot, 0)
		for _, a := range m.Attachments {
			attSnapshots = append(attSnapshots, dto.AttachmentSnapshot{
				Type:     a.Type,
				URL:      "",
				Filename: a.Filename,
				MIMEType: a.MIMEType,
			})
		}

		out = append(out, dto.MessageSnapshot{
			ID:             m.ID.String(),
			ConversationID: m.ConversationID,
			Role:           m.Role,
			Content:        m.Content,
			CreatedAt:      m.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			Attachments:    attSnapshots,
		})
	}
	return out, nil
}

// SendMessage implementa conversations.Provider.
// Uses the unified MessageHandler when available (one channel, one message, one response).
func (d *Deps) SendMessage(ctx context.Context, conversationID, content string) (*dto.SendMessageResult, error) {
	if d.MessageDispatcher != nil {
		convID := conversationID
		if err := d.MessageDispatcher.Handle(ctx, handlers.HandleMessageInput{
			ChannelID:      conversationID,
			Content:        content,
			ChannelType:    "dashboard",
			ConversationID: &convID,
			SenderID:       "dashboard",
			SenderName:     "Dashboard",
		}); err != nil {
			return nil, err
		}
		return &dto.SendMessageResult{
			ID:             "",
			ConversationID: conversationID,
			Role:           "user",
			Content:        content,
			CreatedAt:      time.Now().Format(time.RFC3339),
		}, nil
	}
	// Fallback when MessageDispatcher not wired (e.g. tests)
	if d.MsgRepo == nil {
		return nil, nil
	}
	userMsg := &models.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		ChannelID:      conversationID,
		Role:           "user",
		Content:        content,
		Timestamp:      time.Now(),
	}
	if err := d.MsgRepo.Save(ctx, userMsg); err != nil {
		return nil, err
	}
	if d.EventBus != nil {
		_ = d.EventBus.Publish(ctx, "message_sent", map[string]interface{}{
			"MessageID": userMsg.ID.String(),
			"ChannelID": conversationID,
			"Content":   content,
			"Role":      "user",
			"Timestamp": userMsg.Timestamp,
		})
	}
	if d.AIProvider != nil {
		go d.processWithLLM(context.Background(), conversationID, content, userMsg)
	}
	return &dto.SendMessageResult{
		ID:             userMsg.ID.String(),
		ConversationID: userMsg.ConversationID,
		Role:           userMsg.Role,
		Content:        userMsg.Content,
		CreatedAt:      userMsg.Timestamp.Format(time.RFC3339),
	}, nil
}

func (d *Deps) processWithLLM(ctx context.Context, conversationID, content string, userMsg *models.Message) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	history, _ := d.MsgRepo.GetSinceLastCompaction(ctx, conversationID)
	chatMsgs := []ports.ChatMessage{{Role: "system", Content: "You are a helpful assistant."}}
	for _, m := range history {
		chatMsgs = append(chatMsgs, ports.ChatMessage{Role: m.Role, Content: m.Content})
	}
	chatMsgs = append(chatMsgs, ports.ChatMessage{Role: "user", Content: content})
	resp, err := d.AIProvider.Chat(ctx, ports.ChatRequest{Model: "default", Messages: chatMsgs})
	if err != nil || resp.Content == "" || d.MsgRepo == nil {
		return
	}
	if mcp.ContainsNO_REPLY(resp.Content) {
		return
	}
	agentMsg := &models.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		ChannelID:      conversationID,
		Role:           "assistant",
		Content:        resp.Content,
		Timestamp:      time.Now(),
	}
	_ = d.MsgRepo.Save(ctx, agentMsg)
	if d.EventBus != nil {
		_ = d.EventBus.Publish(ctx, "message_sent", map[string]interface{}{
			"MessageID": agentMsg.ID.String(),
			"ChannelID": conversationID,
			"Content":   resp.Content,
			"Role":      "assistant",
			"Timestamp": agentMsg.Timestamp,
		})
	}
}

// DeleteUser implementa conversations.Provider.
func (d *Deps) DeleteUser(ctx context.Context, conversationID string) error {
	if d.ConvPort == nil {
		return nil
	}
	return d.ConvPort.DeleteUser(ctx, conversationID)
}

// ─── Config provider ─────────────────────────────────────────────────────────

// Config implements config.Provider.
func (d *Deps) Config(ctx context.Context) *dto.AppConfigSnapshot {
	return d.ConfigSnapshot
}

// UpdateConfig implements config.Provider.
func (d *Deps) UpdateConfig(ctx context.Context, input map[string]interface{}) error {
	if d.ConfigWriter == nil {
		return nil
	}
	changed, err := d.ConfigWriter.Apply(ctx, input)
	if err != nil {
		return err
	}
	_ = changed // los canales se recargan dentro de ConfigWriter.Apply
	return nil
}

// ─── Tasks provider ──────────────────────────────────────────────────────────

// Tasks implements tasks.Provider.
func (d *Deps) Tasks(ctx context.Context) ([]dto.TaskSnapshot, error) {
	return d.taskList(ctx), nil
}

// AddTask implements tasks.Provider.
func (d *Deps) AddTask(ctx context.Context, prompt, schedule string) (string, error) {
	if d.CommandSvc == nil {
		return "", nil
	}
	return d.CommandSvc.AddTask(ctx, prompt, schedule)
}

// CompleteTask implements tasks.Provider.
func (d *Deps) CompleteTask(ctx context.Context, taskID string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.CompleteTask(ctx, taskID)
}

// RemoveTask implements tasks.Provider.
func (d *Deps) RemoveTask(ctx context.Context, taskID string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.DeleteTask(ctx, taskID)
}

// UpdateTask implements tasks.Provider.
func (d *Deps) UpdateTask(ctx context.Context, id, prompt, schedule string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.UpdateTask(ctx, id, prompt, schedule)
}

// ToggleTask implements tasks.Provider.
func (d *Deps) ToggleTask(ctx context.Context, id string, enabled bool) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.ToggleTask(ctx, id, enabled)
}

// ─── MCP provider ────────────────────────────────────────────────────────────

// MCPs implements mcp.Provider.
func (d *Deps) MCPs(ctx context.Context) []dto.MCPSnapshot {
	return d.AgentRegistry.GetMCPs()
}

// MCPServers implements mcp.Provider.
func (d *Deps) MCPServers(ctx context.Context) ([]dto.MCPServerRecord, error) {
	if d.MCPServerRepo == nil {
		return nil, nil
	}
	list, err := d.MCPServerRepo.ListAll(ctx)
	if err != nil {
		return list, err
	}
	// Use McpConnectPort for status and toolCount when available (same source as the MCP client).
	// If toolCount is 0 but there are tools in AgentRegistry (e.g. due to a startup reconnect race),
	// use the registry as a fallback to show the correct count.
	if d.McpConnectPort != nil {
		toolCountByServer := make(map[string]int)
		for _, t := range d.AgentRegistry.GetMCPTools() {
			if t.ServerName != "" {
				toolCountByServer[t.ServerName]++
			}
		}
		for i := range list {
			list[i].Status = d.McpConnectPort.GetConnectionStatus(list[i].Name)
			list[i].ToolCount = d.McpConnectPort.GetServerToolCount(list[i].Name)
			if list[i].ToolCount == 0 && toolCountByServer[list[i].Name] > 0 {
				list[i].ToolCount = toolCountByServer[list[i].Name]
			}
		}
	} else {
		tools := d.AgentRegistry.GetMCPTools()
		toolCountByServer := make(map[string]int)
		for _, t := range tools {
			if t.ServerName != "" {
				toolCountByServer[t.ServerName]++
			}
		}
		for i := range list {
			list[i].Status = "unknown"
			list[i].ToolCount = toolCountByServer[list[i].Name]
		}
	}
	return list, nil
}

// MCPOAuthStatus implements mcp.Provider.
func (d *Deps) MCPOAuthStatus(ctx context.Context, serverName string) (string, error) {
	if d.McpOAuthPort == nil {
		return "unknown", nil
	}
	status, errMsg := d.McpOAuthPort.Status(serverName)
	if errMsg != "" {
		return status, fmt.Errorf("%s", errMsg)
	}
	return status, nil
}

// ConnectMCP implements mcp.Provider.
func (d *Deps) ConnectMCP(ctx context.Context, name, transport, url string) (requiresAuth bool, err error) {
	if d.McpConnectPort == nil {
		return false, nil // no-op if not wired
	}
	return d.McpConnectPort.Connect(ctx, name, transport, url)
}

// DisconnectMCP implements mcp.Provider.
func (d *Deps) DisconnectMCP(ctx context.Context, name string) error {
	if d.McpConnectPort == nil {
		return nil
	}
	return d.McpConnectPort.Disconnect(ctx, name)
}

// InitiateOAuth implements mcp.Provider.
func (d *Deps) InitiateOAuth(ctx context.Context, serverName, mcpURL string) (string, error) {
	if d.McpOAuthPort == nil {
		return "", fmt.Errorf("OAuth not configured")
	}
	return d.McpOAuthPort.InitiateOAuth(ctx, serverName, mcpURL)
}

// ─── Memory provider ─────────────────────────────────────────────────────────

// SearchMemory implements memory.Provider.
func (d *Deps) SearchMemory(ctx context.Context, userID, query string) (string, error) {
	if d.QuerySvc == nil {
		return "", nil
	}
	return d.QuerySvc.SearchMemory(ctx, userID, query)
}

// UserGraph implements memory.Provider.
// Empty userID ("") returns the full graph; any other value filters by that user.
func (d *Deps) UserGraph(ctx context.Context, userID string) (*dto.GraphSnapshot, error) {
	if d.QuerySvc == nil {
		return &dto.GraphSnapshot{}, nil
	}
	g, err := d.QuerySvc.GetUserGraph(ctx, userID)
	if err != nil {
		return nil, err
	}
	return portsGraphToSnapshot(g), nil
}

// MemoryGraph implements memory.Provider.
// Uses an empty userID to return the full graph (all memories from all users),
// so the dashboard can show bot-generated memories from any channel.
func (d *Deps) MemoryGraph(ctx context.Context) (*dto.GraphSnapshot, error) {
	return d.UserGraph(ctx, "")
}

// AddMemory implements memory.Provider.
func (d *Deps) AddMemory(ctx context.Context, userID, content string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.AddMemory(ctx, userID, content)
}

// AddRelation implements memory.Provider.
func (d *Deps) AddRelation(ctx context.Context, from, to, relType string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.AddRelation(ctx, from, to, relType)
}

// ExecuteCypher implements memory.Provider.
func (d *Deps) ExecuteCypher(ctx context.Context, cypher string) (*dto.GraphSnapshot, error) {
	if d.QuerySvc == nil {
		return &dto.GraphSnapshot{}, nil
	}
	_, err := d.QuerySvc.ExecuteCypher(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return &dto.GraphSnapshot{}, nil
}

// AddMemoryNode implements memory.Provider.
func (d *Deps) AddMemoryNode(ctx context.Context, label, typ, value string) (string, error) {
	if d.CommandSvc == nil || d.MemoryRepo == nil {
		return "", nil
	}
	if err := d.CommandSvc.AddMemory(ctx, "dashboard", value); err != nil {
		return "", err
	}
	return "", nil
}

// UpdateMemoryNode implements memory.Provider.
func (d *Deps) UpdateMemoryNode(ctx context.Context, id, label, typ, value string, properties map[string]string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.UpdateNode(ctx, id, label, typ, value, properties)
}

// DeleteMemoryNode implements memory.Provider.
func (d *Deps) DeleteMemoryNode(ctx context.Context, id string) error {
	if d.CommandSvc == nil {
		return nil
	}
	return d.CommandSvc.DeleteNode(ctx, id)
}

func portsGraphToSnapshot(g domainservices.PortsGraph) *dto.GraphSnapshot {
	snap := &dto.GraphSnapshot{}
	for _, n := range g.Nodes {
		snap.Nodes = append(snap.Nodes, dto.GraphNodeSnapshot{
			ID:         n.ID,
			Label:      n.Label,
			Type:       n.Type,
			Value:      n.Value,
			Properties: n.Properties,
		})
	}
	for _, e := range g.Edges {
		snap.Edges = append(snap.Edges, dto.GraphEdgeSnapshot{
			Source: e.Source,
			Target: e.Target,
			Label:  e.Label,
		})
	}
	return snap
}

// ─── Skills provider ────────────────────────────────────────────────────────

// Skills implements skills.Provider.
func (d *Deps) Skills(ctx context.Context) ([]dto.SkillSnapshot, error) {
	if d.SkillsPort == nil {
		return nil, nil
	}
	return d.SkillsPort.ListSkills()
}

// SystemFiles implements skills.Provider.
func (d *Deps) SystemFiles(ctx context.Context) ([]dto.SystemFileSnapshot, error) {
	if d.SysFilesPort == nil {
		return nil, nil
	}
	return d.SysFilesPort.ListFiles()
}

// EnableSkill implements skills.Provider.
func (d *Deps) EnableSkill(ctx context.Context, name string) error {
	if d.SkillsPort == nil {
		return nil
	}
	return d.SkillsPort.EnableSkill(name)
}

// DisableSkill implements skills.Provider.
func (d *Deps) DisableSkill(ctx context.Context, name string) error {
	if d.SkillsPort == nil {
		return nil
	}
	return d.SkillsPort.DisableSkill(name)
}

// DeleteSkill implements skills.Provider.
func (d *Deps) DeleteSkill(ctx context.Context, name string) error {
	if d.SkillsPort == nil {
		return nil
	}
	return d.SkillsPort.DeleteSkill(name)
}

// ImportSkill implements skills.Provider.
func (d *Deps) ImportSkill(ctx context.Context, data []byte) error {
	if d.SkillsPort == nil {
		return nil
	}
	return d.SkillsPort.ImportSkill(data)
}

// WriteSystemFile implements skills.Provider.
func (d *Deps) WriteSystemFile(ctx context.Context, name, content string) error {
	if d.SysFilesPort == nil {
		return nil
	}
	return d.SysFilesPort.WriteFile(name, content)
}

// ─── Tools provider ─────────────────────────────────────────────────────────

// ToolPermissions implements tools.Provider.
func (d *Deps) ToolPermissions(ctx context.Context, userID string) ([]dto.ToolPermissionRecord, error) {
	if d.ToolPermRepo == nil {
		return nil, nil
	}
	return d.ToolPermRepo.ListByUser(ctx, userID)
}

// PendingPairings implements tools.Provider.
func (d *Deps) PendingPairings(ctx context.Context) ([]dto.PairingSnapshot, error) {
	if d.PairingPort == nil {
		return nil, nil
	}
	return d.PairingPort.ListActive(ctx)
}

// Users implements tools.Provider.
func (d *Deps) Users(ctx context.Context) ([]dto.UserSnapshot, error) {
	if d.UserRepo == nil {
		return nil, nil
	}
	users, err := d.UserRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.UserSnapshot, len(users))
	for i, u := range users {
		displayName := u.PrimaryID
		if d.UserChannelRepo != nil {
			if dn, err := d.UserChannelRepo.GetDisplayNameByUserID(ctx, u.ID.String()); err == nil && dn != "" {
				displayName = dn
			}
		}
		out[i] = dto.UserSnapshot{
			ID:          u.ID.String(),
			DisplayName: displayName,
		}
	}
	return out, nil
}

// AgentName implements tools.Provider. Returns the agent name from settings.
func (d *Deps) AgentName(ctx context.Context) string {
	if d.ConfigSnapshot != nil && d.ConfigSnapshot.Agent != nil {
		return d.ConfigSnapshot.Agent.Name
	}
	return ""
}

// SetToolPermission implements tools.Provider.
func (d *Deps) SetToolPermission(ctx context.Context, userID, toolName, mode string) error {
	if d.ToolPermRepo == nil {
		return nil
	}
	return d.ToolPermRepo.Set(ctx, userID, toolName, mode)
}

// DeleteToolPermission implements tools.Provider.
func (d *Deps) DeleteToolPermission(ctx context.Context, userID, toolName string) error {
	if d.ToolPermRepo == nil {
		return nil
	}
	return d.ToolPermRepo.Delete(ctx, userID, toolName)
}

// SetAllToolPermissions implements tools.Provider. Applies mode to ALL tools.
func (d *Deps) SetAllToolPermissions(ctx context.Context, userID, mode string) error {
	if d.ToolPermRepo == nil {
		return nil
	}
	var toolNames []string
	if d.ToolNamesSource != nil {
		toolNames = d.ToolNamesSource.AllToolNames()
	}
	if len(toolNames) == 0 {
		// Fallback: solo actualizar las que ya tienen registro (comportamiento anterior)
		all, err := d.ToolPermRepo.ListAll(ctx)
		if err != nil {
			return err
		}
		for _, r := range all {
			if r.UserID == userID {
				if err := d.ToolPermRepo.Set(ctx, userID, r.ToolName, mode); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, name := range toolNames {
		if err := d.ToolPermRepo.Set(ctx, userID, name, mode); err != nil {
			return err
		}
	}
	return nil
}

// ApprovePairing implements tools.Provider.
func (d *Deps) ApprovePairing(ctx context.Context, code, userID, displayName string) (*dto.PairingSnapshot, error) {
	if d.PairingPort == nil {
		return nil, nil
	}
	return d.PairingPort.Approve(ctx, code, userID, displayName)
}

// DenyPairing implements tools.Provider.
func (d *Deps) DenyPairing(ctx context.Context, code, reason string) error {
	if d.PairingPort == nil {
		return nil
	}
	return d.PairingPort.Deny(ctx, code, reason)
}
