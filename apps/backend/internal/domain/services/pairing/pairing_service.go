// Copyright (c) OpenLobster contributors. See LICENSE for details.

package pairing

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusExpired  = "expired"
	StatusDenied   = "denied"

	// CodeLength is the length of generated pairing codes.
	CodeLength     = 6
	pairingTTL     = 15 * time.Minute
)

// Service manages the pairing lifecycle between a channel and a user.
// It contains only the domain logic for codes and status. The orchestration
// (user_channel binding, sending notifications) belongs to the GraphQL resolvers.
//
// Flow:
//  1. User sends /pair or first DM → GenerateCode returns a short code.
//  2. Admin approves via dashboard → ApproveCode updates status; caller creates user_channel.
//  3. Code expires after pairingTTL if not approved.
type Service struct {
	repo ports.PairingRepositoryPort
}

// NewService constructs a PairingService backed by the given repository.
func NewService(repo ports.PairingRepositoryPort) *Service {
	return &Service{repo: repo}
}

// GenerateCode creates a new pairing code for the given channel and platform user,
// persists it, and returns the code string.
// platformUserName is the human-readable display name visible on the platform.
// channelType identifies the originating channel (telegram, discord, whatsapp, twilio).
func (s *Service) GenerateCode(ctx context.Context, channelID, platformUserID, platformUserName, channelType string) (string, error) {
	_ = s.repo.DeleteExpired(ctx)

	code, err := randomCode(CodeLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate pairing code: %w", err)
	}

	now := time.Now()
	p := &ports.Pairing{
		Code:             code,
		ChannelID:        channelID,
		PlatformUserID:   platformUserID,
		PlatformUserName: platformUserName,
		ChannelType:      channelType,
		ExpiresAt:        now.Add(pairingTTL).Unix(),
		Status:           StatusPending,
		CreatedAt:        now.Unix(),
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return "", fmt.Errorf("failed to persist pairing code: %w", err)
	}

	return code, nil
}

// ApproveCode marks the code as approved and returns the associated pairing.
// Returns an error if the code is not found, already resolved, or expired.
// The caller (e.g. GraphQL resolver) is responsible for creating user_channel
// and sending the approval notification to the user.
func (s *Service) ApproveCode(ctx context.Context, code string) (*ports.Pairing, error) {
	p, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("pairing code not found: %w", err)
	}

	if time.Now().After(time.Unix(p.ExpiresAt, 0)) {
		if err := s.repo.UpdateStatus(ctx, code, StatusExpired); err != nil {
			return nil, fmt.Errorf("failed to mark pairing code expired: %w", err)
		}
		return nil, fmt.Errorf("pairing code %q has expired", code)
	}

	updated, err := s.repo.UpdateStatusIfPending(ctx, code, StatusApproved)
	if err != nil {
		return nil, fmt.Errorf("failed to approve pairing code: %w", err)
	}
	if !updated {
		p2, err := s.repo.GetByCode(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("pairing code not found after update: %w", err)
		}
		return nil, fmt.Errorf("pairing code %q is already %s", code, p2.Status)
	}

	p.Status = StatusApproved
	return p, nil
}

// DenyCode marks the code as denied.
func (s *Service) DenyCode(ctx context.Context, code string) error {
	p, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("pairing code not found: %w", err)
	}
	if p.Status != StatusPending {
		return fmt.Errorf("pairing code %q is already %s", code, p.Status)
	}
	return s.repo.UpdateStatus(ctx, code, StatusDenied)
}

// GetStatus returns the current state of a pairing code.
// Lazily marks expired pending codes as expired when read.
func (s *Service) GetStatus(ctx context.Context, code string) (*ports.Pairing, error) {
	p, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("pairing code not found: %w", err)
	}

	if p.Status == StatusPending && time.Now().After(time.Unix(p.ExpiresAt, 0)) {
		_ = s.repo.UpdateStatus(ctx, code, StatusExpired)
		p.Status = StatusExpired
	}

	return p, nil
}

// CleanupExpired removes all expired pairing codes from the store.
func (s *Service) CleanupExpired(ctx context.Context) error {
	return s.repo.DeleteExpired(ctx)
}

// ListActive returns all pairing codes that have not yet expired.
func (s *Service) ListActive(ctx context.Context) ([]ports.Pairing, error) {
	return s.repo.ListActive(ctx)
}

func randomCode(length int) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // omit confusable chars O/0 I/1
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}
