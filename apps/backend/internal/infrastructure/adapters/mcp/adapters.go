// Package mcp provides infrastructure adapters that satisfy the service
// interfaces consumed by the MCP internal-tools system.  Each adapter bridges
// a domain port or repository to the corresponding mcp.*Service interface so
// that internal tools (send_message, add_memory, schedule_task, …) can be
// wired up without pulling infrastructure concerns into the domain layer.
package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	browser "github.com/neirth/openlobster/internal/infrastructure/adapters/browser/chromedp"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	svmcp "github.com/neirth/openlobster/internal/domain/services/mcp"
)

// ---------------------------------------------------------------------------
// MessagingAdapter — bridges ports.MessagingPort to mcp.MessagingService
// ---------------------------------------------------------------------------

// MessagingAdapter wraps a ports.MessagingPort and exposes the simplified
// mcp.MessagingService interface expected by internal tools.
type MessagingAdapter struct{ Port ports.MessagingPort }

func (m *MessagingAdapter) SendMessage(ctx context.Context, channelType, channelID, content string) error {
	if m.Port == nil {
		return fmt.Errorf("messaging: no adapter configured")
	}
	msg := models.NewMessage(channelID, content)
	if channelType != "" {
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]interface{})
		}
		msg.Metadata["channel_type"] = channelType
	}
	return m.Port.SendMessage(ctx, msg)
}

func (m *MessagingAdapter) SendMedia(ctx context.Context, media *ports.Media) error {
	if m.Port == nil {
		return fmt.Errorf("messaging: no adapter configured")
	}
	return m.Port.SendMedia(ctx, media)
}

// OutboundMessageLogAdapter persists outbound send_message deliveries so they
// are visible in conversation history.
type OutboundMessageLogAdapter struct {
	MessageRepo     ports.MessageRepositoryPort
	SessionRepo     ports.SessionRepositoryPort
	UserChannelRepo ports.UserChannelRepositoryPort
}

func (a *OutboundMessageLogAdapter) SaveOutbound(ctx context.Context, channelType, channelID, content string) error {
	if a.MessageRepo == nil || a.SessionRepo == nil {
		return nil
	}

	var userID string
	if a.UserChannelRepo != nil && channelID != "" {
		uid, err := a.UserChannelRepo.GetUserIDByPlatformUserID(ctx, channelID)
		if err != nil {
			return fmt.Errorf("resolve user by platform id: %w", err)
		}
		userID = uid
	}

	var session *models.Session
	if userID != "" {
		if sessions, err := a.SessionRepo.GetActiveByUser(ctx, userID); err == nil && len(sessions) > 0 {
			session = &sessions[0]
		}
	}
	if session == nil {
		if sessions, err := a.SessionRepo.GetActiveByChannel(ctx, channelID); err == nil && len(sessions) > 0 {
			session = &sessions[0]
		}
	}
	if session == nil {
		session = models.NewSession(userID)
		session.ChannelID = channelID
		session.UserID = userID
		if err := a.SessionRepo.Create(ctx, session); err != nil {
			return fmt.Errorf("create conversation for outbound message: %w", err)
		}
	}

	msg := models.NewMessage(channelID, content)
	msg.Role = "assistant"
	msg.ConversationID = session.ID.String()
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]interface{})
	}
	msg.Metadata["channel_type"] = channelType
	if err := a.MessageRepo.Save(ctx, msg); err != nil {
		return fmt.Errorf("save outbound message: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// MemoryAdapter — bridges ports.MemoryPort to mcp.MemoryService
// ---------------------------------------------------------------------------

// MemoryAdapter wraps a ports.MemoryPort and exposes the mcp.MemoryService interface.
type MemoryAdapter struct{ Port ports.MemoryPort }

func (m *MemoryAdapter) AddKnowledge(ctx context.Context, userID, content, label, relation, entityType string) error {
	if m.Port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.Port.AddKnowledge(ctx, userID, content, label, relation, entityType, nil)
}

func (m *MemoryAdapter) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	if m.Port == nil {
		return nil
	}
	return m.Port.UpdateUserLabel(ctx, userID, displayName)
}

func (m *MemoryAdapter) SearchMemory(ctx context.Context, userID, query string) (string, error) {
	if m.Port == nil {
		return "", fmt.Errorf("memory: no adapter configured")
	}

	graph, err := m.Port.GetUserGraph(ctx, userID)
	if err != nil {
		return "", err
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(queryLower)

	var sb strings.Builder
	count := 0
	for _, node := range graph.Nodes {
		if node.Type == "user" {
			continue
		}
		valueLower := strings.ToLower(node.Value)
		labelLower := strings.ToLower(node.Label)
		matched := false
		for _, w := range queryWords {
			if strings.Contains(valueLower, w) || strings.Contains(labelLower, w) {
				matched = true
				break
			}
		}
		if !matched && queryLower != "" && len(queryWords) > 0 {
			continue
		}
		fmt.Fprintf(&sb, "[node_id:%s] %s\n", node.ID, node.Value)
		count++
		if count >= 10 {
			break
		}
	}

	if sb.Len() == 0 && len(graph.Nodes) > 0 {
		for _, node := range graph.Nodes {
			if node.Type != "user" {
				fmt.Fprintf(&sb, "[node_id:%s] %s\n", node.ID, node.Value)
				count++
				if count >= 10 {
					break
				}
			}
		}
	}
	return sb.String(), nil
}

func (m *MemoryAdapter) SetUserProperty(ctx context.Context, userID, key, value string) error {
	if m.Port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.Port.SetUserProperty(ctx, userID, key, value)
}

func (m *MemoryAdapter) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	if m.Port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.Port.EditMemoryNode(ctx, userID, nodeID, newValue)
}

func (m *MemoryAdapter) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	if m.Port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.Port.DeleteMemoryNode(ctx, userID, nodeID)
}

func (m *MemoryAdapter) AddRelation(ctx context.Context, from, to, relType string) error {
	if m.Port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.Port.AddRelation(ctx, from, to, relType)
}

func (m *MemoryAdapter) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	if m.Port == nil {
		return ports.GraphResult{}, fmt.Errorf("memory: no adapter configured")
	}
	return m.Port.QueryGraph(ctx, cypher)
}

// ---------------------------------------------------------------------------
// TaskAdapter — bridges repositories.TaskRepository to mcp.TaskService
// ---------------------------------------------------------------------------

// TaskAdapter wraps a TaskRepository and exposes the mcp.TaskService interface.
type TaskAdapter struct {
	Repo   repositories.TaskRepository
	Notify func()
}

func (a *TaskAdapter) Add(ctx context.Context, prompt, schedule string) (string, error) {
	t := models.NewTask(prompt, schedule)
	if err := a.Repo.Add(ctx, t); err != nil {
		return "", err
	}
	if a.Notify != nil {
		a.Notify()
	}
	return t.ID, nil
}

func (a *TaskAdapter) Done(ctx context.Context, id string) error {
	if err := a.Repo.Done(ctx, id); err != nil {
		return err
	}
	if a.Notify != nil {
		a.Notify()
	}
	return nil
}

func (a *TaskAdapter) List(ctx context.Context) ([]svmcp.TaskInfo, error) {
	tasks, err := a.Repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]svmcp.TaskInfo, len(tasks))
	for i, t := range tasks {
		result[i] = svmcp.TaskInfo{
			ID:       t.ID,
			Prompt:   t.Prompt,
			Schedule: t.Schedule,
			Status:   t.Status,
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// ConversationAdapter — bridges repositories to mcp.ConversationService
// ---------------------------------------------------------------------------

// ConversationAdapter bridges the conversation and message repositories to the
// mcp.ConversationService interface used by internal tools.
type ConversationAdapter struct {
	ConvRepo *repositories.ConversationRepository
	MsgRepo  ports.MessageRepositoryPort
}

func (a *ConversationAdapter) ListConversations(ctx context.Context) ([]svmcp.ConversationSummary, error) {
	rows, err := a.ConvRepo.ListConversations()
	if err != nil {
		return nil, err
	}
	result := make([]svmcp.ConversationSummary, 0, len(rows))
	for _, r := range rows {
		result = append(result, svmcp.ConversationSummary{
			ID:              r.ID,
			ChannelID:       r.ChannelID,
			ChannelName:     r.ChannelName,
			ParticipantID:   r.ParticipantID,
			ParticipantName: r.ParticipantName,
			LastMessageAt:   r.LastMessageAt,
			MessageCount:    r.UnreadCount,
		})
	}
	return result, nil
}

func (a *ConversationAdapter) GetConversationMessages(ctx context.Context, conversationID string, limit int) ([]svmcp.ConversationMessage, error) {
	msgs, err := a.MsgRepo.GetByConversation(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]svmcp.ConversationMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "compaction" {
			continue
		}
		result = append(result, svmcp.ConversationMessage{
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// CronAdapter — bridges repositories.TaskRepository to mcp.CronService
// ---------------------------------------------------------------------------

// CronAdapter bridges the task repository to the mcp.CronService interface.
// Cyclic tasks (task_type='cyclic') in the tasks table act as cron jobs.
type CronAdapter struct {
	Repo   repositories.TaskRepository
	Notify func()
}

func (a *CronAdapter) Schedule(ctx context.Context, name, schedule, prompt, _ string) error {
	task := models.NewTask(prompt, schedule)
	if err := a.Repo.Add(ctx, task); err != nil {
		return err
	}
	if a.Notify != nil {
		a.Notify()
	}
	return nil
}

func (a *CronAdapter) List(ctx context.Context) ([]svmcp.CronJobInfo, error) {
	tasks, err := a.Repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]svmcp.CronJobInfo, 0)
	for _, t := range tasks {
		if t.TaskType == models.TaskTypeCyclic {
			result = append(result, svmcp.CronJobInfo{
				ID:       t.ID,
				Name:     t.Prompt,
				Schedule: t.Schedule,
			})
		}
	}
	return result, nil
}

func (a *CronAdapter) Delete(ctx context.Context, jobID string) error {
	if err := a.Repo.Delete(ctx, jobID); err != nil {
		return err
	}
	if a.Notify != nil {
		a.Notify()
	}
	return nil
}

// ---------------------------------------------------------------------------
// BrowserAdapter — bridges browser.ChromeDPAdapter to mcp.BrowserService
// ---------------------------------------------------------------------------

// BrowserAdapter bridges a ChromeDPAdapter to the mcp.BrowserService interface.
// Each sessionID maps to an open browser page kept alive across tool calls.
type BrowserAdapter struct {
	Port  *browser.ChromeDPAdapter
	pages sync.Map // sessionID -> ports.BrowserPage
}

func (a *BrowserAdapter) getOrCreatePage(ctx context.Context, sessionID string) (ports.BrowserPage, error) {
	if v, ok := a.pages.Load(sessionID); ok {
		return v.(ports.BrowserPage), nil
	}
	page, err := a.Port.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	a.pages.Store(sessionID, page)
	return page, nil
}

func (a *BrowserAdapter) Fetch(ctx context.Context, sessionID, url string) (string, error) {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if err := page.Navigate(ctx, url); err != nil {
		return "", err
	}
	result, err := page.Eval(ctx, "document.documentElement.innerText")
	if err != nil {
		return "", err
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", result), nil
}

func (a *BrowserAdapter) Screenshot(ctx context.Context, sessionID string) ([]byte, error) {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return page.Screenshot(ctx)
}

func (a *BrowserAdapter) Click(ctx context.Context, sessionID, selector string) error {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return err
	}
	return page.Click(ctx, selector)
}

func (a *BrowserAdapter) FillInput(ctx context.Context, sessionID, selector, text string) error {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return err
	}
	return page.Type(ctx, selector, text)
}
