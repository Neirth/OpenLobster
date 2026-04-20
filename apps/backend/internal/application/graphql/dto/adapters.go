// Adapters that bridge domain repositories and services to the dto port
// interfaces consumed by the GraphQL resolvers.  Follows the same pattern as
// subagent_adapter.go which already lives in this package.
package dto

import (
	"context"
	"fmt"
	"time"

	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
)

// ---------------------------------------------------------------------------
// ConversationPortAdapter
// ---------------------------------------------------------------------------

// ConversationPortAdapter adapts ConversationRepository to ConversationPort.
type ConversationPortAdapter struct {
	Repo interface {
		ListConversations() ([]repositories.ConversationRow, error)
		DeleteUser(ctx context.Context, conversationID string) error
		DeleteGroup(ctx context.Context, conversationID string) error
	}
}

func (a *ConversationPortAdapter) ListConversations() ([]ConversationSnapshot, error) {
	rows, err := a.Repo.ListConversations()
	if err != nil {
		return nil, err
	}
	result := make([]ConversationSnapshot, len(rows))
	for i, r := range rows {
		result[i] = ConversationSnapshot{
			ID:              r.ID,
			ChannelID:       r.ChannelID,
			ChannelType:     r.ChannelType,
			ChannelName:     r.ChannelName,
			GroupName:       r.GroupName,
			IsGroup:         r.IsGroup,
			ParticipantID:   r.ParticipantID,
			ParticipantName: r.ParticipantName,
			LastMessageAt:   r.LastMessageAt,
			UnreadCount:     r.UnreadCount,
		}
	}
	return result, nil
}

func (a *ConversationPortAdapter) DeleteUser(ctx context.Context, conversationID string) error {
	return a.Repo.DeleteUser(ctx, conversationID)
}

func (a *ConversationPortAdapter) DeleteGroup(ctx context.Context, conversationID string) error {
	return a.Repo.DeleteGroup(ctx, conversationID)
}

// ---------------------------------------------------------------------------
// ToolPermAdapter
// ---------------------------------------------------------------------------

// ToolPermAdapter adapts repositories.ToolPermissionRepositoryPort to ToolPermissionsRepo.
type ToolPermAdapter struct {
	Repo repositories.ToolPermissionRepositoryPort
}

func (a *ToolPermAdapter) Set(ctx context.Context, userID, toolName, mode string) error {
	return a.Repo.Set(ctx, userID, toolName, mode)
}

func (a *ToolPermAdapter) Delete(ctx context.Context, userID, toolName string) error {
	return a.Repo.Delete(ctx, userID, toolName)
}

func (a *ToolPermAdapter) ListByUser(ctx context.Context, userID string) ([]ToolPermissionRecord, error) {
	rows, err := a.Repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]ToolPermissionRecord, len(rows))
	for i, r := range rows {
		result[i] = ToolPermissionRecord{UserID: r.UserID, ToolName: r.ToolName, Mode: r.Mode}
	}
	return result, nil
}

func (a *ToolPermAdapter) ListAll(ctx context.Context) ([]ToolPermissionRecord, error) {
	rows, err := a.Repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ToolPermissionRecord, len(rows))
	for i, r := range rows {
		result[i] = ToolPermissionRecord{UserID: r.UserID, ToolName: r.ToolName, Mode: r.Mode}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// MCPServerAdapter
// ---------------------------------------------------------------------------

// MCPServerAdapter adapts repositories.MCPServerRepositoryPort to MCPServerRepo.
type MCPServerAdapter struct {
	Repo repositories.MCPServerRepositoryPort
}

func (a *MCPServerAdapter) Save(ctx context.Context, name, url string) error {
	return a.Repo.Save(ctx, name, url)
}

func (a *MCPServerAdapter) Delete(ctx context.Context, name string) error {
	return a.Repo.Delete(ctx, name)
}

func (a *MCPServerAdapter) ListAll(ctx context.Context) ([]MCPServerRecord, error) {
	rows, err := a.Repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]MCPServerRecord, len(rows))
	for i, r := range rows {
		result[i] = MCPServerRecord{Name: r.Name, URL: r.URL}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// MsgRepoAdapter
// ---------------------------------------------------------------------------

// MsgRepoAdapter adapts repositories.DashboardMessageRepository to MessageRepo.
type MsgRepoAdapter struct {
	Repo *repositories.DashboardMessageRepository
}

func (a *MsgRepoAdapter) Save(ctx context.Context, msg *models.Message) error {
	return a.Repo.Save(msg)
}

func (a *MsgRepoAdapter) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	return a.Repo.GetByConversation(conversationID, limit)
}

func (a *MsgRepoAdapter) GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]models.Message, error) {
	return a.Repo.GetByConversationPaged(ctx, conversationID, before, limit)
}

func (a *MsgRepoAdapter) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error) {
	return a.Repo.GetSinceLastCompaction(ctx, conversationID)
}

func (a *MsgRepoAdapter) CountMessages(ctx context.Context) (int64, int64, error) {
	return a.Repo.CountMessages(ctx)
}

// ---------------------------------------------------------------------------
// PairingPortAdapter
// ---------------------------------------------------------------------------

// PairingPortAdapter orchestrates the pairing approval/denial flow, bridging
// the domain pairing service and user/channel repositories to PairingPort.
type PairingPortAdapter struct {
	Svc             *domainservices.PairingService
	UserRepo        ports.UserRepositoryPort
	UserChannelRepo ports.UserChannelRepositoryPort
	ChannelRepo     ports.ChannelRepositoryPort
	MessageSender   MessageSender
	EventBus        domainservices.EventBus
}

func (a *PairingPortAdapter) Approve(ctx context.Context, code, userID, displayName string) (*PairingSnapshot, error) {
	p, err := a.Svc.ApproveCode(ctx, code)
	if err != nil {
		return nil, err
	}

	platformUserID := p.PlatformUserID
	if platformUserID == "" {
		platformUserID = p.ChannelID
	}
	platformUsername := p.PlatformUserName

	if a.ChannelRepo != nil {
		_ = a.ChannelRepo.EnsurePlatform(ctx, p.ChannelType, p.ChannelType)
	}

	resolveUserID := userID
	if resolveUserID == "" {
		if a.UserRepo != nil {
			u, err := a.UserRepo.GetByPrimaryID(ctx, platformUserID)
			if err == nil && u != nil {
				resolveUserID = u.ID.String()
			}
		}
		if resolveUserID == "" && a.UserRepo != nil {
			u := models.NewUser(platformUserID)
			u.Name = displayName
			if err := a.UserRepo.Create(ctx, u); err == nil {
				resolveUserID = u.ID.String()
			}
		}
	}
	if resolveUserID != "" && displayName != "" && a.UserRepo != nil {
		if existing, err := a.UserRepo.GetByID(ctx, resolveUserID); err == nil && existing != nil && existing.Name == "" {
			existing.Name = displayName
			_ = a.UserRepo.Update(ctx, existing)
		}
	}

	if resolveUserID != "" && a.UserChannelRepo != nil {
		if err := a.UserChannelRepo.Create(ctx, resolveUserID, p.ChannelType, platformUserID, platformUsername); err != nil {
			return nil, fmt.Errorf("create user_channel: %w", err)
		}
	}

	if a.EventBus != nil {
		_ = a.EventBus.Publish(ctx, events.NewEvent(events.EventPairingApproved, events.PairingApprovedPayload{
			RequestID:  p.Code,
			Code:       p.Code,
			ApprovedBy: "admin",
			Timestamp:  time.Now(),
		}))
	}

	if a.MessageSender != nil && p.ChannelID != "" {
		_ = a.MessageSender.SendTextToChannel(ctx, p.ChannelType, p.ChannelID, "Your access request has been approved. You can start chatting now.")
	}

	return &PairingSnapshot{Code: p.Code, Status: p.Status}, nil
}

func (a *PairingPortAdapter) Deny(ctx context.Context, code, reason string) error {
	return a.Svc.DenyCode(ctx, code)
}

func (a *PairingPortAdapter) ListActive(ctx context.Context) ([]PairingSnapshot, error) {
	list, err := a.Svc.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PairingSnapshot, len(list))
	for i, p := range list {
		out[i] = PairingSnapshot{
				Code:             p.Code,
				Status:           p.Status,
				ChannelID:        p.ChannelID,
				ChannelType:      p.ChannelType,
				PlatformUserName: p.PlatformUserName,
			}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// UserRepoAdapter
// ---------------------------------------------------------------------------

// UserRepoAdapter adapts ports.UserRepositoryPort to UserRepo.
type UserRepoAdapter struct {
	Repo ports.UserRepositoryPort
}

func (a *UserRepoAdapter) Create(ctx context.Context, user *models.User) error {
	return a.Repo.Create(ctx, user)
}

func (a *UserRepoAdapter) GetByID(ctx context.Context, id string) (*models.User, error) {
	return a.Repo.GetByID(ctx, id)
}

func (a *UserRepoAdapter) ListAll(ctx context.Context) ([]models.User, error) {
	return a.Repo.ListAll(ctx)
}

// ---------------------------------------------------------------------------
// EventBusAdapter
// ---------------------------------------------------------------------------

// EventBusAdapter adapts domainservices.EventBus to EventBusPort.
type EventBusAdapter struct {
	Eb domainservices.EventBus
}

func (e *EventBusAdapter) Publish(ctx context.Context, eventType string, payload interface{}) error {
	if e.Eb == nil {
		return nil
	}
	return e.Eb.Publish(ctx, events.NewEvent(eventType, payload))
}

// ---------------------------------------------------------------------------
// EventSubscriptionAdapter
// ---------------------------------------------------------------------------

// EventSubscriptionAdapter adapts domainservices.EventBus to the GraphQL
// subscription resolver interface (returns a channel of domain events).
type EventSubscriptionAdapter struct {
	Eb domainservices.EventBus
}

func (a *EventSubscriptionAdapter) Subscribe(ctx context.Context, eventType string) (<-chan events.Event, error) {
	if a.Eb == nil {
		ch := make(chan events.Event)
		return ch, nil
	}
	ch := make(chan events.Event, 64)
	done := ctx.Done()
	if err := a.Eb.Subscribe(eventType, func(_ context.Context, event events.Event) error {
		select {
		case ch <- event:
		case <-done:
		default:
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ch, nil
}
