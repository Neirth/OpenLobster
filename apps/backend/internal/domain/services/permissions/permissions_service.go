package permissions

import (
	"context"
	"strings"
	"sync"
)

// RoleCanMutate returns true if the given role is allowed to execute GraphQL mutations.
// Admin and user roles can mutate; read-only and other roles cannot.
func RoleCanMutate(role string) bool {
	switch strings.ToLower(role) {
	case "admin", "user":
		return true
	default:
		return false
	}
}

type ToolPermissionMode string

const (
	PermissionDeny   ToolPermissionMode = "deny"
	PermissionAlways ToolPermissionMode = "always"
)

type ToolPermission struct {
	ToolName string
	Mode     ToolPermissionMode
}

type Manager struct {
	mu          sync.RWMutex
	permissions map[string]map[string]ToolPermissionMode
}

var defaultManager *Manager

func init() {
	defaultManager = NewManager()
}

func NewManager() *Manager {
	return &Manager{
		permissions: make(map[string]map[string]ToolPermissionMode),
	}
}

func Default() *Manager {
	return defaultManager
}

func (m *Manager) SetPermission(userID, toolName string, mode ToolPermissionMode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.permissions[userID] == nil {
		m.permissions[userID] = make(map[string]ToolPermissionMode)
	}
	m.permissions[userID][toolName] = mode
}

// RemovePermission deletes the explicit entry for a user+tool pair so that the
// allow-by-default policy applies on the next GetPermission call.
func (m *Manager) RemovePermission(userID, toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if perms, ok := m.permissions[userID]; ok {
		delete(perms, toolName)
	}
}

// ResetUserPermissions removes all explicit permission entries for a given user,
// effectively reverting every tool to the allow-by-default policy.
// Use this before reloading permissions from the database to guarantee that
// entries deleted in persistent storage are also cleared from the in-memory snapshot.
func (m *Manager) ResetUserPermissions(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.permissions, userID)
}

func (m *Manager) GetPermission(userID, toolName string) ToolPermissionMode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check global policy first; if "*" explicitly denies, always deny.
	if globalPerms, ok := m.permissions["*"]; ok {
		if gmode, ok := globalPerms[toolName]; ok {
			if gmode == PermissionDeny {
				return PermissionDeny
			}
			// Global is "always": allow unless the user has an explicit deny.
			if perms, ok := m.permissions[userID]; ok {
				if umode, ok := perms[toolName]; ok {
					return umode
				}
			}
			return gmode
		}
	}

	// No global entry: check per-user permissions.
	if perms, ok := m.permissions[userID]; ok {
		if mode, ok := perms[toolName]; ok {
			return mode
		}
	}

	// Tools with no explicit entry are allowed by default.
	return PermissionAlways
}

func (m *Manager) CheckPermission(userID, toolName string) bool {
	return m.GetPermission(userID, toolName) != PermissionDeny
}

type AuthorizationService struct {
	permManager *Manager
}

func NewAuthorizationService() *AuthorizationService {
	return &AuthorizationService{
		permManager: Default(),
	}
}

func (s *AuthorizationService) Check(ctx context.Context, userID, toolName string) bool {
	return s.permManager.CheckPermission(userID, toolName)
}

func (s *AuthorizationService) SetPermission(ctx context.Context, userID, toolName string, mode ToolPermissionMode) {
	s.permManager.SetPermission(userID, toolName, mode)
}
