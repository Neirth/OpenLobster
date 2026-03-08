// Copyright (c) OpenLobster contributors. See LICENSE for details.

package pairing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePairingRepo struct {
	pairings map[string]*ports.Pairing
}

func newFakePairingRepo() *fakePairingRepo {
	return &fakePairingRepo{pairings: make(map[string]*ports.Pairing)}
}

func (r *fakePairingRepo) Create(_ context.Context, p *ports.Pairing) error {
	r.pairings[p.Code] = p
	return nil
}

func (r *fakePairingRepo) GetByCode(_ context.Context, code string) (*ports.Pairing, error) {
	p, ok := r.pairings[code]
	if !ok {
		return nil, errors.New("not found")
	}
	copy := *p
	return &copy, nil
}

func (r *fakePairingRepo) UpdateStatus(_ context.Context, code, status string) error {
	p, ok := r.pairings[code]
	if !ok {
		return errors.New("not found")
	}
	p.Status = status
	return nil
}

func (r *fakePairingRepo) UpdateStatusIfPending(_ context.Context, code string, newStatus string) (bool, error) {
	p, ok := r.pairings[code]
	if !ok {
		return false, errors.New("not found")
	}
	if p.Status != "pending" {
		return false, nil
	}
	p.Status = newStatus
	return true, nil
}

func (r *fakePairingRepo) DeleteExpired(_ context.Context) error {
	now := time.Now().Unix()
	for code, p := range r.pairings {
		if p.ExpiresAt < now {
			delete(r.pairings, code)
		}
	}
	return nil
}

func (r *fakePairingRepo) ListActive(_ context.Context) ([]ports.Pairing, error) {
	now := time.Now().Unix()
	var result []ports.Pairing
	for _, p := range r.pairings {
		if p.ExpiresAt > now {
			result = append(result, *p)
		}
	}
	return result, nil
}

func TestPairingService_GenerateCode(t *testing.T) {
	svc := NewService(newFakePairingRepo())

	code, err := svc.GenerateCode(context.Background(), "channel-1", "user-42", "TestUser", "telegram")
	require.NoError(t, err)
	assert.Len(t, code, CodeLength)
}

func TestPairingService_GenerateCode_Unique(t *testing.T) {
	svc := NewService(newFakePairingRepo())

	codes := make(map[string]struct{})
	for i := 0; i < 20; i++ {
		code, err := svc.GenerateCode(context.Background(), "ch", "u", "", "")
		require.NoError(t, err)
		codes[code] = struct{}{}
	}
	assert.Len(t, codes, 20)
}

func TestPairingService_ApproveCode(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, err := svc.GenerateCode(context.Background(), "channel-1", "user-42", "", "")
	require.NoError(t, err)

	pairing, err := svc.ApproveCode(context.Background(), code)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, pairing.Status)
	assert.Equal(t, code, pairing.Code)
}

func TestPairingService_ApproveCode_NotFound(t *testing.T) {
	svc := NewService(newFakePairingRepo())

	_, err := svc.ApproveCode(context.Background(), "DOESNOTEXIST")
	assert.Error(t, err)
}

func TestPairingService_ApproveCode_AlreadyApproved(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")
	_, err := svc.ApproveCode(context.Background(), code)
	require.NoError(t, err)

	_, err = svc.ApproveCode(context.Background(), code)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already")
}

func TestPairingService_ApproveCode_Expired(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")
	repo.pairings[code].ExpiresAt = time.Now().Add(-time.Minute).Unix()

	_, err := svc.ApproveCode(context.Background(), code)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestPairingService_DenyCode(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")

	err := svc.DenyCode(context.Background(), code)
	require.NoError(t, err)

	p, _ := repo.GetByCode(context.Background(), code)
	assert.Equal(t, StatusDenied, p.Status)
}

func TestPairingService_DenyCode_AlreadyResolved(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")
	_, _ = svc.ApproveCode(context.Background(), code)

	err := svc.DenyCode(context.Background(), code)
	assert.Error(t, err)
}

func TestPairingService_GetStatus_Pending(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")

	p, err := svc.GetStatus(context.Background(), code)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, p.Status)
}

func TestPairingService_GetStatus_LazyExpiry(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")
	repo.pairings[code].ExpiresAt = time.Now().Add(-time.Minute).Unix()

	p, err := svc.GetStatus(context.Background(), code)
	require.NoError(t, err)
	assert.Equal(t, StatusExpired, p.Status)
}

func TestPairingService_CleanupExpired(t *testing.T) {
	repo := newFakePairingRepo()
	svc := NewService(repo)

	code, _ := svc.GenerateCode(context.Background(), "ch", "u", "", "")
	repo.pairings[code].ExpiresAt = time.Now().Add(-time.Minute).Unix()

	err := svc.CleanupExpired(context.Background())
	require.NoError(t, err)

	assert.Empty(t, repo.pairings)
}
