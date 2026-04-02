// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/require"
)

type dashboardOriginChecker struct {
	existsCalls         int
	updateLastSeenCalls int
}

func (c *dashboardOriginChecker) ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error) {
	c.existsCalls++
	return false, nil
}

func (c *dashboardOriginChecker) GetUserIDByPlatformUserID(ctx context.Context, platformUserID string) (string, error) {
	return "", nil
}

func (c *dashboardOriginChecker) GetDisplayNameByPlatformUserID(ctx context.Context, platformUserID string) (string, error) {
	return "", nil
}

func (c *dashboardOriginChecker) GetDisplayNameByUserID(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", nil
	}
	return "Alice", nil
}

func (c *dashboardOriginChecker) UpdateLastSeen(ctx context.Context, channelType, platformUserID string) error {
	c.updateLastSeenCalls++
	return nil
}

type countingPairingGenerator struct {
	calls int
}

func (g *countingPairingGenerator) GenerateCode(ctx context.Context, channelID, platformUserID, platformUserName, channelType string) (string, error) {
	g.calls++
	return "PAIR-1234", nil
}

type dashboardSessionRepo struct {
	sessionByID map[string]*models.Session
	getByIDCall int
	updateCalls int
}

func (r *dashboardSessionRepo) Create(ctx context.Context, session *models.Session) error {
	return nil
}

func (r *dashboardSessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	r.getByIDCall++
	if r.sessionByID == nil {
		return nil, nil
	}
	return r.sessionByID[id], nil
}

func (r *dashboardSessionRepo) Update(ctx context.Context, session *models.Session) error {
	r.updateCalls++
	return nil
}

func (r *dashboardSessionRepo) GetActiveByUser(ctx context.Context, userID string) ([]models.Session, error) {
	return nil, nil
}

func (r *dashboardSessionRepo) GetActiveByChannel(ctx context.Context, channelID string) ([]models.Session, error) {
	return nil, nil
}

func (r *dashboardSessionRepo) GetActiveByGroup(ctx context.Context, groupID string) ([]models.Session, error) {
	return nil, nil
}

func TestHandle_DashboardOriginSkipsPairingAndLoadsConversationSession(t *testing.T) {
	convID := uuid.NewString()
	parsedID, err := uuid.Parse(convID)
	require.NoError(t, err)

	checker := &dashboardOriginChecker{}
	pairing := &countingPairingGenerator{}
	sessions := &dashboardSessionRepo{sessionByID: map[string]*models.Session{
		convID: {
			ID:        parsedID,
			UserID:    "user-42",
			ChannelID: "telegram",
		},
	}}
	ai := &fixedReplyAI{content: "respuesta de prueba"}

	h := &MessageHandler{
		runner:         agenticRunner{aiProvider: ai},
		sessionRepo:    sessions,
		channelChecker: checker,
		pairingGen:     pairing,
	}

	err = h.handle(context.Background(), HandleMessageInput{
		ChannelID:      "813579246",
		ChannelType:    "telegram",
		ConversationID: &convID,
		SenderID:       "813579246",
		SenderName:     "Alice",
		FromDashboard:  true,
		Content:        "hola",
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&ai.calls))
	require.Equal(t, 0, checker.existsCalls)
	require.Equal(t, 0, pairing.calls)
	require.Equal(t, 1, sessions.getByIDCall)
	require.Equal(t, 0, checker.updateLastSeenCalls)
}
