package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.permissions)
}

func TestSetAndGetPermission(t *testing.T) {
	m := NewManager()

	m.SetPermission("user1", "read_file", PermissionAlways)
	m.SetPermission("user2", "read_file", PermissionDeny)

	assert.Equal(t, PermissionAlways, m.GetPermission("user1", "read_file"))
	assert.Equal(t, PermissionDeny, m.GetPermission("user2", "read_file"))
}

func TestDefaultPermission(t *testing.T) {
	m := NewManager()
	// Unknown tool with no explicit entry defaults to allow-by-default.
	assert.Equal(t, PermissionAlways, m.GetPermission("user1", "unknown_tool"))
}

func TestGlobalPermission(t *testing.T) {
	m := NewManager()

	m.SetPermission("*", "read_file", PermissionAlways)

	assert.Equal(t, PermissionAlways, m.GetPermission("any_user", "read_file"))
	// write_file has no explicit entry, falls back to allow-by-default.
	assert.Equal(t, PermissionAlways, m.GetPermission("any_user", "write_file"))
}

func TestCheckPermission_Always(t *testing.T) {
	m := NewManager()
	m.SetPermission("user1", "read_file", PermissionAlways)

	assert.True(t, m.CheckPermission("user1", "read_file"))
}

func TestCheckPermission_Deny(t *testing.T) {
	m := NewManager()
	m.SetPermission("user1", "read_file", PermissionDeny)

	assert.False(t, m.CheckPermission("user1", "read_file"))
}

func TestRemovePermission(t *testing.T) {
	m := NewManager()
	m.SetPermission("user1", "read_file", PermissionDeny)
	m.RemovePermission("user1", "read_file")

	// Reverts to allow-by-default.
	assert.Equal(t, PermissionAlways, m.GetPermission("user1", "read_file"))
}

func TestResetUserPermissions(t *testing.T) {
	m := NewManager()
	m.SetPermission("user1", "read_file", PermissionDeny)
	m.SetPermission("user1", "write_file", PermissionAlways)
	m.ResetUserPermissions("user1")

	assert.Equal(t, PermissionAlways, m.GetPermission("user1", "read_file"))
	assert.Equal(t, PermissionAlways, m.GetPermission("user1", "write_file"))
}

func TestGlobalDenyOverridesUser(t *testing.T) {
	m := NewManager()
	m.SetPermission("*", "dangerous_tool", PermissionDeny)
	// Even if user has no explicit entry, global deny wins.
	assert.Equal(t, PermissionDeny, m.GetPermission("user1", "dangerous_tool"))
}

func TestRoleCanMutate(t *testing.T) {
	assert.True(t, RoleCanMutate("admin"))
	assert.True(t, RoleCanMutate("user"))
	assert.True(t, RoleCanMutate("ADMIN"))
	assert.True(t, RoleCanMutate("User"))
	assert.False(t, RoleCanMutate("read-only"))
	assert.False(t, RoleCanMutate(""))
	assert.False(t, RoleCanMutate("guest"))
}

func TestGlobalAllowWithUserDeny(t *testing.T) {
	m := NewManager()
	m.SetPermission("*", "tool", PermissionAlways)
	m.SetPermission("user1", "tool", PermissionDeny)
	// User explicit deny overrides global always.
	assert.Equal(t, PermissionDeny, m.GetPermission("user1", "tool"))
}

func TestDefault(t *testing.T) {
	d := Default()
	assert.NotNil(t, d)
	assert.Equal(t, d, Default())
}
