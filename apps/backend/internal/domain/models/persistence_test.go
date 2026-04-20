package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserModel_TableName(t *testing.T) {
	var m UserModel
	assert.Equal(t, "users", m.TableName())
}

func TestChannelModel_TableName(t *testing.T) {
	var m ChannelModel
	assert.Equal(t, "channels", m.TableName())
}

func TestGroupModel_TableName(t *testing.T) {
	var m GroupModel
	assert.Equal(t, "groups", m.TableName())
}

func TestGroupUserModel_TableName(t *testing.T) {
	var m GroupUserModel
	assert.Equal(t, "group_users", m.TableName())
}

func TestUserChannelModel_TableName(t *testing.T) {
	var m UserChannelModel
	assert.Equal(t, "user_channels", m.TableName())
}

func TestConversationModel_TableName(t *testing.T) {
	var m ConversationModel
	assert.Equal(t, "conversations", m.TableName())
}

func TestMessageModel_TableName(t *testing.T) {
	var m MessageModel
	assert.Equal(t, "messages", m.TableName())
}

func TestTaskModel_TableName(t *testing.T) {
	var m TaskModel
	assert.Equal(t, "tasks", m.TableName())
}

func TestPairingModel_TableName(t *testing.T) {
	var m PairingModel
	assert.Equal(t, "pairings", m.TableName())
}

func TestMCPServerModel_TableName(t *testing.T) {
	var m MCPServerModel
	assert.Equal(t, "mcp_servers", m.TableName())
}

func TestToolPermissionModel_TableName(t *testing.T) {
	var m ToolPermissionModel
	assert.Equal(t, "tool_permissions", m.TableName())
}
