package mappers

import (
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
)

func StrPtr(s string) *string { return &s }
func BoolPtr(b bool) *bool    { return &b }
func IntPtr(i int) *int       { return &i }

func SnapshotToAgent(a *dto.AgentSnapshot) *generated.Agent {
	if a == nil {
		return nil
	}
	agent := &generated.Agent{
		ID:       a.ID,
		Name:     a.Name,
		Version:  a.Version,
		Status:   a.Status,
		Uptime:   int(a.Uptime),
		Channels: ChannelsToGenerated(a.Channels),
	}
	if a.Provider != "" {
		agent.Provider = StrPtr(a.Provider)
	}
	if a.AIProvider != "" {
		agent.AiProvider = StrPtr(a.AIProvider)
	}
	if a.MemoryBackend != "" {
		agent.MemoryBackend = StrPtr(a.MemoryBackend)
	}
	return agent
}

func ChannelsToGenerated(list []dto.ChannelStatus) []*generated.Channel {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Channel, len(list))
	for i, c := range list {
		out[i] = ChannelToGenerated(c)
	}
	return out
}

func ChannelToGenerated(c dto.ChannelStatus) *generated.Channel {
	return &generated.Channel{
		ID:      c.ID,
		Name:    c.Name,
		Type:    c.Type,
		Status:  c.Status,
		Enabled: c.Enabled,
		Capabilities: &generated.ChannelCapabilities{
			HasVoiceMessage: c.Capabilities.HasVoiceMessage,
			HasCallStream:   c.Capabilities.HasCallStream,
			HasTextStream:   c.Capabilities.HasTextStream,
			HasMediaSupport: c.Capabilities.HasMediaSupport,
		},
	}
}

func HeartbeatToGenerated(h *dto.HeartbeatSnapshot) *generated.Heartbeat {
	if h == nil {
		return nil
	}
	return &generated.Heartbeat{Status: h.Status, LastCheck: int(h.LastCheck)}
}

func ToolsToGenerated(list []dto.ToolSnapshot) []*generated.Tool {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Tool, len(list))
	for i, t := range list {
		out[i] = &generated.Tool{Name: t.Name}
		if t.Description != "" {
			out[i].Description = StrPtr(t.Description)
		}
		if t.Source != "" {
			out[i].Source = StrPtr(t.Source)
		}
	}
	return out
}

func SubAgentsToGenerated(list []dto.SubAgentSnapshot) []*generated.SubAgent {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.SubAgent, len(list))
	for i, s := range list {
		out[i] = &generated.SubAgent{ID: s.ID, Name: s.Name, Status: s.Status}
		if s.Task != "" {
			out[i].Task = StrPtr(s.Task)
		}
	}
	return out
}

func StatusToGenerated(s *dto.StatusSnapshot) *generated.Status {
	if s == nil {
		return nil
	}
	return &generated.Status{
		Agent:     SnapshotToAgent(s.Agent),
		Health:    HeartbeatToGenerated(s.Health),
		Channels:  ChannelsToGenerated(s.Channels),
		Tools:     ToolsToGenerated(s.Tools),
		SubAgents: SubAgentsToGenerated(s.SubAgents),
		Tasks:     TasksToGenerated(s.Tasks),
		Mcps:      MCPsToGenerated(s.Mcps),
	}
}

func TaskSnapshotToGeneratedFull(t dto.TaskSnapshot) *generated.Task {
	task := TaskSnapshotToGenerated(t)
	if t.IsCyclic {
		task.IsCyclic = BoolPtr(true)
	}
	if t.CreatedAt != "" {
		task.CreatedAt = StrPtr(t.CreatedAt)
	}
	if t.LastRunAt != "" {
		task.LastRunAt = StrPtr(t.LastRunAt)
	}
	if t.NextRunAt != "" {
		task.NextRunAt = StrPtr(t.NextRunAt)
	}
	return task
}

func MetricsToGenerated(m *dto.MetricsSnapshot) *generated.Metrics {
	if m == nil {
		return nil
	}
	return &generated.Metrics{
		Uptime:           int(m.Uptime),
		MessagesReceived: int(m.MessagesReceived),
		MessagesSent:     int(m.MessagesSent),
		ActiveSessions:   int(m.ActiveSessions),
		MemoryNodes:      int(m.MemoryNodes),
		MemoryEdges:      int(m.MemoryEdges),
		McpTools:         int(m.McpTools),
		TasksPending:     int(m.TasksPending),
		TasksRunning:     int(m.TasksRunning),
		TasksDone:        int(m.TasksDone),
		ErrorsTotal:      int(m.ErrorsTotal),
	}
}

func TasksToGenerated(list []dto.TaskSnapshot) []*generated.Task {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Task, len(list))
	for i, t := range list {
		out[i] = TaskSnapshotToGeneratedFull(t)
	}
	return out
}

func TaskSnapshotToGenerated(t dto.TaskSnapshot) *generated.Task {
	task := &generated.Task{
		ID:      t.ID,
		Prompt:  t.Prompt,
		Status:  t.Status,
		Enabled: t.Enabled,
	}
	if t.Schedule != "" {
		task.Schedule = StrPtr(t.Schedule)
	}
	if t.TaskType != "" {
		task.TaskType = StrPtr(t.TaskType)
	}
	return task
}

func ConversationsToGenerated(list []dto.ConversationSnapshot) []*generated.Conversation {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Conversation, len(list))
	for i, c := range list {
		out[i] = ConversationSnapshotToGenerated(c)
	}
	return out
}

func ConversationSnapshotToGenerated(c dto.ConversationSnapshot) *generated.Conversation {
	conv := &generated.Conversation{ID: c.ID, ChannelID: c.ChannelID, IsGroup: c.IsGroup}
	if c.ChannelName != "" {
		conv.ChannelName = StrPtr(c.ChannelName)
	}
	if c.GroupName != "" {
		conv.GroupName = StrPtr(c.GroupName)
	}
	if c.ParticipantID != "" {
		conv.ParticipantID = StrPtr(c.ParticipantID)
	}
	if c.ParticipantName != "" {
		conv.ParticipantName = StrPtr(c.ParticipantName)
	}
	if c.LastMessageAt != "" {
		conv.LastMessageAt = StrPtr(c.LastMessageAt)
	}
	if c.UnreadCount != 0 {
		conv.UnreadCount = IntPtr(c.UnreadCount)
	}
	return conv
}

func attachmentsToGenerated(list []dto.AttachmentSnapshot) []*generated.MessageAttachment {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.MessageAttachment, 0, len(list))
	for _, a := range list {
		att := &generated.MessageAttachment{Type: a.Type}
		if a.Filename != "" {
			att.Filename = StrPtr(a.Filename)
		}
		if a.MIMEType != "" {
			att.MimeType = StrPtr(a.MIMEType)
		}
		if a.URL != "" {
			att.URL = StrPtr(a.URL)
		}
		out = append(out, att)
	}
	return out
}

func MessagesToGenerated(list []dto.MessageSnapshot) []*generated.Message {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Message, 0, len(list))
	for _, m := range list {
		if m.Role == "compaction" {
			continue
		}
		out = append(out, &generated.Message{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			Role:           m.Role,
			Content:        m.Content,
			CreatedAt:      m.CreatedAt,
			Attachments:    attachmentsToGenerated(m.Attachments),
		})
	}
	return out
}

func SendMessageResultToGenerated(r *dto.SendMessageResult) *generated.MessageSentResult {
	if r == nil {
		return nil
	}
	success := true
	res := &generated.MessageSentResult{Success: &success}
	if r.ID != "" {
		res.ID = StrPtr(r.ID)
	}
	if r.ConversationID != "" {
		res.ConversationID = StrPtr(r.ConversationID)
	}
	if r.Role != "" {
		res.Role = StrPtr(r.Role)
	}
	if r.Content != "" {
		res.Content = StrPtr(r.Content)
	}
	if r.CreatedAt != "" {
		res.CreatedAt = StrPtr(r.CreatedAt)
	}
	return res
}

func MCPsToGenerated(list []dto.MCPSnapshot) []*generated.Mcp {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Mcp, len(list))
	for i, m := range list {
		out[i] = &generated.Mcp{Name: m.Name}
		if m.Type != "" {
			out[i].Type = StrPtr(m.Type)
		}
		if m.Status != "" {
			out[i].Status = StrPtr(m.Status)
		}
		if m.URL != "" {
			out[i].URL = StrPtr(m.URL)
		}
		out[i].Tools = ToolsToGenerated(m.Tools)
	}
	return out
}

// ─── Config ───────────────────────────────────────────────────────────────────

func AppConfigSnapshotToGenerated(cfg *dto.AppConfigSnapshot) *generated.AppConfig {
	if cfg == nil {
		return &generated.AppConfig{}
	}
	out := &generated.AppConfig{}
	if cfg.Agent != nil {
		out.Agent = &generated.AgentConfig{
			Name:                      strOrNil(cfg.Agent.Name),
			SystemPrompt:              strOrNil(cfg.Agent.SystemPrompt),
			Provider:                  strOrNil(cfg.Agent.Provider),
			Model:                     strOrNil(cfg.Agent.Model),
			APIKey:                    strOrNil(cfg.Agent.APIKey),
			BaseURL:                   strOrNil(cfg.Agent.BaseURL),
			OllamaHost:                strOrNil(cfg.Agent.OllamaHost),
			OllamaAPIKey:              strOrNil(cfg.Agent.OllamaApiKey),
			AnthropicAPIKey:           strOrNil(cfg.Agent.AnthropicApiKey),
			DockerModelRunnerEndpoint: strOrNil(cfg.Agent.DockerModelRunnerEndpoint),
			DockerModelRunnerModel:    strOrNil(cfg.Agent.DockerModelRunnerModel),
		}
	}
	if cfg.Capabilities != nil {
		out.Capabilities = &generated.CapabilitiesConfig{
			Browser:    BoolPtr(cfg.Capabilities.Browser),
			Terminal:   BoolPtr(cfg.Capabilities.Terminal),
			Subagents:  BoolPtr(cfg.Capabilities.Subagents),
			Memory:     BoolPtr(cfg.Capabilities.Memory),
			Mcp:        BoolPtr(cfg.Capabilities.MCP),
			Filesystem: BoolPtr(cfg.Capabilities.Filesystem),
			Sessions:   BoolPtr(cfg.Capabilities.Sessions),
		}
	}
	if cfg.Database != nil {
		out.Database = &generated.DatabaseConfig{
			Driver:       strOrNil(cfg.Database.Driver),
			Dsn:          strOrNil(cfg.Database.DSN),
			MaxOpenConns: intOrNil(cfg.Database.MaxOpenConns),
			MaxIdleConns: intOrNil(cfg.Database.MaxIdleConns),
		}
	}
	if cfg.Memory != nil {
		mm := &generated.MemoryConfig{
			Backend:  strOrNil(cfg.Memory.Backend),
			FilePath: strOrNil(cfg.Memory.FilePath),
		}
		if cfg.Memory.Neo4j != nil {
			mm.Neo4j = &generated.Neo4jConfig{
				URI:      strOrNil(cfg.Memory.Neo4j.URI),
				User:     strOrNil(cfg.Memory.Neo4j.User),
				Password: strOrNil(cfg.Memory.Neo4j.Password),
			}
		}
		out.Memory = mm
	}
	if cfg.Subagents != nil {
		out.Subagents = &generated.SubagentsConfig{
			MaxConcurrent:  intOrNil(cfg.Subagents.MaxConcurrent),
			DefaultTimeout: strOrNil(cfg.Subagents.DefaultTimeout),
		}
	}
	if cfg.GraphQL != nil {
		out.Graphql = &generated.GraphQLConfig{
			Enabled: BoolPtr(cfg.GraphQL.Enabled),
			Port:    IntPtr(cfg.GraphQL.Port),
			Host:    strOrNil(cfg.GraphQL.Host),
			BaseURL: strOrNil(cfg.GraphQL.BaseURL),
		}
	}
	if cfg.Logging != nil {
		out.Logging = &generated.LoggingConfig{
			Level: strOrNil(cfg.Logging.Level),
			Path:  strOrNil(cfg.Logging.Path),
		}
	}
	if cfg.Scheduler != nil {
		out.Scheduler = &generated.SchedulerConfig{
			Enabled:        BoolPtr(cfg.Scheduler.Enabled),
			MemoryEnabled:  BoolPtr(cfg.Scheduler.MemoryEnabled),
			MemoryInterval: strOrNil(cfg.Scheduler.MemoryInterval),
		}
	}
	if cfg.Secrets != nil {
		sec := &generated.SecretsConfig{Backend: strOrNil(cfg.Secrets.Backend)}
		if cfg.Secrets.File != nil {
			sec.File = &generated.FileSecretsConfig{Path: strOrNil(cfg.Secrets.File.Path)}
		}
		if cfg.Secrets.Openbao != nil {
			sec.Openbao = &generated.OpenbaoSecretsConfig{
				URL:   strOrNil(cfg.Secrets.Openbao.URL),
				Token: strOrNil(cfg.Secrets.Openbao.Token),
			}
		}
		out.Secrets = sec
	}
	for _, s := range cfg.ActiveSessions {
		sess := &generated.ActiveSession{ID: s.ID}
		if s.Address != "" {
			sess.Address = StrPtr(s.Address)
		}
		if s.Status != "" {
			sess.Status = StrPtr(s.Status)
		}
		if s.Channel != "" {
			sess.Channel = StrPtr(s.Channel)
		}
		if s.User != "" {
			sess.User = StrPtr(s.User)
		}
		out.ActiveSessions = append(out.ActiveSessions, sess)
	}
	for _, ch := range cfg.Channels {
		chCfg := &generated.ChannelConfig{ChannelID: ch.ChannelID, Enabled: ch.Enabled}
		if ch.ChannelName != "" {
			chCfg.ChannelName = StrPtr(ch.ChannelName)
		}
		out.Channels = append(out.Channels, chCfg)
	}
	if cfg.ChannelSecrets != nil {
		cs := &generated.ChannelSecretsConfig{}
		cs.TelegramEnabled = BoolPtr(cfg.ChannelSecrets.TelegramEnabled)
		cs.TelegramToken = strOrNil(cfg.ChannelSecrets.TelegramToken)
		cs.DiscordEnabled = BoolPtr(cfg.ChannelSecrets.DiscordEnabled)
		cs.DiscordToken = strOrNil(cfg.ChannelSecrets.DiscordToken)
		cs.WhatsAppEnabled = BoolPtr(cfg.ChannelSecrets.WhatsAppEnabled)
		cs.WhatsAppPhoneID = strOrNil(cfg.ChannelSecrets.WhatsAppPhoneId)
		cs.WhatsAppAPIToken = strOrNil(cfg.ChannelSecrets.WhatsAppApiToken)
		cs.TwilioEnabled = BoolPtr(cfg.ChannelSecrets.TwilioEnabled)
		cs.TwilioAccountSid = strOrNil(cfg.ChannelSecrets.TwilioAccountSid)
		cs.TwilioAuthToken = strOrNil(cfg.ChannelSecrets.TwilioAuthToken)
		cs.TwilioFromNumber = strOrNil(cfg.ChannelSecrets.TwilioFromNumber)
		cs.SlackEnabled = BoolPtr(cfg.ChannelSecrets.SlackEnabled)
		cs.SlackBotToken = strOrNil(cfg.ChannelSecrets.SlackBotToken)
		cs.SlackAppToken = strOrNil(cfg.ChannelSecrets.SlackAppToken)
		out.ChannelSecrets = cs
	}
	out.WizardCompleted = BoolPtr(cfg.WizardCompleted)
	return out
}

func AppConfigSnapshotToUpdateConfigResult(cfg *dto.AppConfigSnapshot) *generated.UpdateConfigResult {
	res := &generated.UpdateConfigResult{}
	if cfg == nil {
		return res
	}
	if cfg.Agent != nil {
		if cfg.Agent.Name != "" {
			res.AgentName = StrPtr(cfg.Agent.Name)
		}
		if cfg.Agent.SystemPrompt != "" {
			res.SystemPrompt = StrPtr(cfg.Agent.SystemPrompt)
		}
		if cfg.Agent.Provider != "" {
			res.Provider = StrPtr(cfg.Agent.Provider)
		}
	}
	if len(cfg.Channels) > 0 {
		for _, ch := range cfg.Channels {
			chCfg := &generated.ChannelConfig{ChannelID: ch.ChannelID, Enabled: ch.Enabled}
			if ch.ChannelName != "" {
				chCfg.ChannelName = StrPtr(ch.ChannelName)
			}
			res.Channels = append(res.Channels, chCfg)
		}
	}
	return res
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intOrNil(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

// UpdateConfigInputToMap convierte UpdateConfigInput a map para el servicio.
func UpdateConfigInputToMap(input generated.UpdateConfigInput) map[string]interface{} {
	m := make(map[string]interface{})
	if input.AgentName != nil {
		m["agentName"] = *input.AgentName
	}
	if input.SystemPrompt != nil {
		m["systemPrompt"] = *input.SystemPrompt
	}
	if input.Provider != nil {
		m["provider"] = *input.Provider
	}
	if input.Model != nil {
		m["model"] = *input.Model
	}
	if input.APIKey != nil {
		m["apiKey"] = *input.APIKey
	}
	if input.BaseURL != nil {
		m["baseURL"] = *input.BaseURL
	}
	if input.OllamaHost != nil {
		m["ollamaHost"] = *input.OllamaHost
	}
	if input.OllamaAPIKey != nil {
		m["ollamaApiKey"] = *input.OllamaAPIKey
	}
	if input.AnthropicAPIKey != nil {
		m["anthropicApiKey"] = *input.AnthropicAPIKey
	}
	if input.DockerModelRunnerEndpoint != nil {
		m["dockerModelRunnerEndpoint"] = *input.DockerModelRunnerEndpoint
	}
	if input.DockerModelRunnerModel != nil {
		m["dockerModelRunnerModel"] = *input.DockerModelRunnerModel
	}
	if input.Capabilities != nil {
		caps := make(map[string]interface{})
		if input.Capabilities.Browser != nil {
			caps["browser"] = *input.Capabilities.Browser
		}
		if input.Capabilities.Terminal != nil {
			caps["terminal"] = *input.Capabilities.Terminal
		}
		if input.Capabilities.Subagents != nil {
			caps["subagents"] = *input.Capabilities.Subagents
		}
		if input.Capabilities.Memory != nil {
			caps["memory"] = *input.Capabilities.Memory
		}
		if input.Capabilities.Mcp != nil {
			caps["mcp"] = *input.Capabilities.Mcp
		}
		if input.Capabilities.Filesystem != nil {
			caps["filesystem"] = *input.Capabilities.Filesystem
		}
		if input.Capabilities.Sessions != nil {
			caps["sessions"] = *input.Capabilities.Sessions
		}
		m["capabilities"] = caps
	}
	if input.DatabaseDriver != nil {
		m["databaseDriver"] = *input.DatabaseDriver
	}
	if input.DatabaseDsn != nil {
		m["databaseDSN"] = *input.DatabaseDsn
	}
	if input.DatabaseMaxOpenConns != nil {
		m["databaseMaxOpenConns"] = *input.DatabaseMaxOpenConns
	}
	if input.DatabaseMaxIdleConns != nil {
		m["databaseMaxIdleConns"] = *input.DatabaseMaxIdleConns
	}
	if input.MemoryBackend != nil {
		m["memoryBackend"] = *input.MemoryBackend
	}
	if input.MemoryFilePath != nil {
		m["memoryFilePath"] = *input.MemoryFilePath
	}
	if input.MemoryNeo4jURI != nil {
		m["memoryNeo4jURI"] = *input.MemoryNeo4jURI
	}
	if input.MemoryNeo4jUser != nil {
		m["memoryNeo4jUser"] = *input.MemoryNeo4jUser
	}
	if input.MemoryNeo4jPassword != nil {
		m["memoryNeo4jPassword"] = *input.MemoryNeo4jPassword
	}
	if input.SubagentsMaxConcurrent != nil {
		m["subagentsMaxConcurrent"] = *input.SubagentsMaxConcurrent
	}
	if input.SubagentsDefaultTimeout != nil {
		m["subagentsDefaultTimeout"] = *input.SubagentsDefaultTimeout
	}
	if input.GraphqlEnabled != nil {
		m["graphqlEnabled"] = *input.GraphqlEnabled
	}
	if input.GraphqlPort != nil {
		m["graphqlPort"] = *input.GraphqlPort
	}
	if input.GraphqlHost != nil {
		m["graphqlHost"] = *input.GraphqlHost
	}
	if input.GraphqlBaseURL != nil {
		m["graphqlBaseUrl"] = *input.GraphqlBaseURL
	}
	if input.LoggingLevel != nil {
		m["loggingLevel"] = *input.LoggingLevel
	}
	if input.LoggingPath != nil {
		m["loggingPath"] = *input.LoggingPath
	}
	if input.SecretsBackend != nil {
		m["secretsBackend"] = *input.SecretsBackend
	}
	if input.SecretsFilePath != nil {
		m["secretsFilePath"] = *input.SecretsFilePath
	}
	if input.SecretsOpenbaoURL != nil {
		m["secretsOpenbaoURL"] = *input.SecretsOpenbaoURL
	}
	if input.SecretsOpenbaoToken != nil {
		m["secretsOpenbaoToken"] = *input.SecretsOpenbaoToken
	}
	if input.SchedulerEnabled != nil {
		m["schedulerEnabled"] = *input.SchedulerEnabled
	}
	if input.SchedulerMemoryEnabled != nil {
		m["schedulerMemoryEnabled"] = *input.SchedulerMemoryEnabled
	}
	if input.SchedulerMemoryInterval != nil {
		m["schedulerMemoryInterval"] = *input.SchedulerMemoryInterval
	}
	if input.ChannelTelegramEnabled != nil {
		m["channelTelegramEnabled"] = *input.ChannelTelegramEnabled
	}
	if input.ChannelTelegramToken != nil {
		m["channelTelegramToken"] = *input.ChannelTelegramToken
	}
	if input.ChannelDiscordEnabled != nil {
		m["channelDiscordEnabled"] = *input.ChannelDiscordEnabled
	}
	if input.ChannelDiscordToken != nil {
		m["channelDiscordToken"] = *input.ChannelDiscordToken
	}
	if input.ChannelWhatsAppEnabled != nil {
		m["channelWhatsAppEnabled"] = *input.ChannelWhatsAppEnabled
	}
	if input.ChannelWhatsAppPhoneID != nil {
		m["channelWhatsAppPhoneId"] = *input.ChannelWhatsAppPhoneID
	}
	if input.ChannelWhatsAppAPIToken != nil {
		m["channelWhatsAppApiToken"] = *input.ChannelWhatsAppAPIToken
	}
	if input.ChannelTwilioEnabled != nil {
		m["channelTwilioEnabled"] = *input.ChannelTwilioEnabled
	}
	if input.ChannelTwilioAccountSid != nil {
		m["channelTwilioAccountSid"] = *input.ChannelTwilioAccountSid
	}
	if input.ChannelTwilioAuthToken != nil {
		m["channelTwilioAuthToken"] = *input.ChannelTwilioAuthToken
	}
	if input.ChannelTwilioFromNumber != nil {
		m["channelTwilioFromNumber"] = *input.ChannelTwilioFromNumber
	}
	if input.ChannelSlackEnabled != nil {
		m["channelSlackEnabled"] = *input.ChannelSlackEnabled
	}
	if input.ChannelSlackBotToken != nil {
		m["channelSlackBotToken"] = *input.ChannelSlackBotToken
	}
	if input.ChannelSlackAppToken != nil {
		m["channelSlackAppToken"] = *input.ChannelSlackAppToken
	}
	if input.WizardCompleted != nil {
		m["wizardCompleted"] = *input.WizardCompleted
	}
	return m
}

// ─── Memory / Graph ──────────────────────────────────────────────────────────

func GraphNodesToGenerated(nodes []dto.GraphNodeSnapshot) []*generated.GraphNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]*generated.GraphNode, len(nodes))
	for i, n := range nodes {
		out[i] = &generated.GraphNode{ID: n.ID}
		if n.Label != "" {
			out[i].Label = StrPtr(n.Label)
		}
		if n.Type != "" {
			out[i].Type = StrPtr(n.Type)
		}
		if n.Value != "" {
			out[i].Value = StrPtr(n.Value)
		}
		if len(n.Properties) > 0 {
			m := make(map[string]interface{})
			for k, v := range n.Properties {
				m[k] = v
			}
			out[i].Properties = m
		}
	}
	return out
}

func GraphEdgesToGenerated(edges []dto.GraphEdgeSnapshot) []*generated.GraphEdge {
	if len(edges) == 0 {
		return nil
	}
	out := make([]*generated.GraphEdge, len(edges))
	for i, e := range edges {
		out[i] = &generated.GraphEdge{Source: e.Source, Target: e.Target}
		if e.Label != "" {
			out[i].Label = StrPtr(e.Label)
		}
	}
	return out
}

func MemoryNodesFromSnapshot(nodes []dto.GraphNodeSnapshot) []*generated.MemoryNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]*generated.MemoryNode, len(nodes))
	for i, n := range nodes {
		out[i] = &generated.MemoryNode{
			ID:    n.ID,
			Label: StrPtr(n.Label),
			Type:  StrPtr(n.Type),
			Value: StrPtr(n.Value),
		}
		if len(n.Properties) > 0 {
			m := make(map[string]interface{})
			for k, v := range n.Properties {
				m[k] = v
			}
			out[i].Properties = m
		}
	}
	return out
}

func MemoryEdgesFromSnapshot(edges []dto.GraphEdgeSnapshot) []*generated.MemoryEdge {
	if len(edges) == 0 {
		return nil
	}
	out := make([]*generated.MemoryEdge, len(edges))
	for i, e := range edges {
		id := e.Source + "-" + e.Target
		out[i] = &generated.MemoryEdge{
			ID:       id,
			SourceID: e.Source,
			TargetID: e.Target,
			Relation: StrPtr(e.Label),
		}
	}
	return out
}

// ─── Tools / Pairing / User ────────────────────────────────────────────────────

func ToolPermissionToGenerated(r dto.ToolPermissionRecord) *generated.ToolPermission {
	return &generated.ToolPermission{ToolName: r.ToolName, Mode: r.Mode}
}

func ToolPermissionsToGenerated(list []dto.ToolPermissionRecord) []*generated.ToolPermission {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.ToolPermission, len(list))
	for i, r := range list {
		out[i] = ToolPermissionToGenerated(r)
	}
	return out
}

func PairingToPendingPairing(p dto.PairingSnapshot) *generated.PendingPairing {
	return &generated.PendingPairing{Code: p.Code, Status: p.Status}
}

func PairingsToGenerated(list []dto.PairingSnapshot) []*generated.PendingPairing {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.PendingPairing, len(list))
	for i, p := range list {
		out[i] = PairingToPendingPairing(p)
	}
	return out
}

func UserSnapshotToGenerated(u dto.UserSnapshot) *generated.User {
	usr := &generated.User{ID: u.ID}
	if u.DisplayName != "" {
		usr.PrimaryID = StrPtr(u.DisplayName)
	}
	return usr
}

func UsersToGenerated(list []dto.UserSnapshot) []*generated.User {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.User, len(list))
	for i, u := range list {
		out[i] = UserSnapshotToGenerated(u)
	}
	return out
}

func UsersToMcpUsers(list []dto.UserSnapshot) []*generated.MCPUser {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.MCPUser, len(list))
	for i, u := range list {
		dn := u.DisplayName
		if dn == "" {
			dn = u.ID
		}
		out[i] = &generated.MCPUser{
			ChannelID:   u.ID,
			DisplayName: StrPtr(dn),
			IsAgent:     false,
		}
	}
	return out
}

func PairingSnapshotToPairingInfo(p *dto.PairingSnapshot) *generated.PairingInfo {
	if p == nil {
		return nil
	}
	return &generated.PairingInfo{Code: p.Code, Status: p.Status}
}

// ─── MCP Server ───────────────────────────────────────────────────────────────

func MCPServerRecordToGenerated(r dto.MCPServerRecord) *generated.MCPServer {
	status := r.Status
	if status == "" {
		status = "unknown"
	}
	srv := &generated.MCPServer{Name: r.Name, Status: status, ToolCount: r.ToolCount}
	if r.URL != "" {
		srv.URL = StrPtr(r.URL)
	}
	return srv
}

func MCPServersToGenerated(list []dto.MCPServerRecord) []*generated.MCPServer {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.MCPServer, len(list))
	for i, r := range list {
		out[i] = MCPServerRecordToGenerated(r)
	}
	return out
}

func MCPToolSnapshotToGenerated(t dto.ToolSnapshot) *generated.MCPTool {
	tool := &generated.MCPTool{Name: t.Name}
	if t.Description != "" {
		tool.Description = StrPtr(t.Description)
	}
	if t.ServerName != "" {
		tool.ServerName = StrPtr(t.ServerName)
	}
	return tool
}

func MCPToolsToGenerated(list []dto.ToolSnapshot) []*generated.MCPTool {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.MCPTool, len(list))
	for i, t := range list {
		out[i] = MCPToolSnapshotToGenerated(t)
	}
	return out
}

// ─── Skills ───────────────────────────────────────────────────────────────────

func SkillSnapshotToGenerated(s dto.SkillSnapshot) *generated.Skill {
	skill := &generated.Skill{Name: s.Name, Enabled: s.Enabled}
	if s.Description != "" {
		skill.Description = StrPtr(s.Description)
	}
	if s.Path != "" {
		skill.Path = StrPtr(s.Path)
	}
	return skill
}

func SkillsToGenerated(list []dto.SkillSnapshot) []*generated.Skill {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.Skill, len(list))
	for i, s := range list {
		out[i] = SkillSnapshotToGenerated(s)
	}
	return out
}

func SystemFileSnapshotToGenerated(s dto.SystemFileSnapshot) *generated.SystemFile {
	sf := &generated.SystemFile{Name: s.Name, Path: s.Path, Content: StrPtr(s.Content)}
	if s.LastModified != "" {
		sf.LastModified = StrPtr(s.LastModified)
	}
	return sf
}

func SystemFilesToGenerated(list []dto.SystemFileSnapshot) []*generated.SystemFile {
	if len(list) == 0 {
		return nil
	}
	out := make([]*generated.SystemFile, len(list))
	for i, s := range list {
		out[i] = SystemFileSnapshotToGenerated(s)
	}
	return out
}
