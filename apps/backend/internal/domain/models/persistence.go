// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package models contains domain-level persistence models.
// GORM acts as the port adapter; the struct definitions therefore belong in
// the domain layer while the concrete adapter (persistence package) uses them.
package models

import "time"

// UserModel maps to the `users` table.
type UserModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	PrimaryID string    `gorm:"uniqueIndex;column:primary_id"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime:false"`
}

func (UserModel) TableName() string { return "users" }

// ChannelModel maps to the `channels` table.
type ChannelModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Type      string    `gorm:"column:type;not null"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

func (ChannelModel) TableName() string { return "channels" }

// GroupModel maps to the `groups` table.
type GroupModel struct {
	ID              string    `gorm:"primaryKey;column:id"`
	ChannelID       string    `gorm:"column:channel_id;not null;index"`
	PlatformGroupID string    `gorm:"column:platform_group_id;not null"`
	Name            string    `gorm:"column:name"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

func (GroupModel) TableName() string { return "groups" }

// GroupUserModel maps to the `group_users` join table.
type GroupUserModel struct {
	GroupID  string    `gorm:"primaryKey;column:group_id"`
	UserID   string    `gorm:"primaryKey;column:user_id"`
	JoinedAt time.Time `gorm:"column:joined_at;autoCreateTime:false"`
	// Associations for cascade behavior
	User  UserModel  `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
	Group GroupModel `gorm:"foreignKey:GroupID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
}

func (GroupUserModel) TableName() string { return "group_users" }

// UserChannelModel maps to the `user_channels` table.
type UserChannelModel struct {
	ID             string    `gorm:"primaryKey;column:id"`
	UserID         string    `gorm:"column:user_id;not null;index:idx_uc_user_channel"`
	ChannelID      string    `gorm:"column:channel_id;not null;index:idx_uc_user_channel;uniqueIndex:idx_uc_channel_platform"`
	PlatformUserID string    `gorm:"column:platform_user_id;not null;index;uniqueIndex:idx_uc_channel_platform"`
	Username       string    `gorm:"column:username"`
	PairedAt       time.Time `gorm:"column:paired_at;autoCreateTime:false"`
	LastSeen       time.Time `gorm:"column:last_seen;autoCreateTime:false"`
	// Cascade association: deleting a user removes their channel bindings
	User UserModel `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
}

func (UserChannelModel) TableName() string { return "user_channels" }

// ConversationModel maps to the `conversations` table.
type ConversationModel struct {
	ID        string     `gorm:"primaryKey;column:id"`
	ChannelID string     `gorm:"column:channel_id;index:idx_conv_channel_active"`
	GroupID   *string    `gorm:"column:group_id;index:idx_conv_group_active"`
	UserID    string     `gorm:"column:user_id;index:idx_conv_user_active"`
	ModelID   string     `gorm:"column:model_id"`
	IsActive  bool       `gorm:"column:is_active;default:true;index:idx_conv_user_active;index:idx_conv_group_active;index:idx_conv_channel_active"`
	StartedAt time.Time  `gorm:"column:started_at;autoCreateTime:false"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime:false"`
	EndedAt   *time.Time `gorm:"column:ended_at"`
	// When a user is deleted, delete their conversations
	User  UserModel  `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
	Group GroupModel `gorm:"foreignKey:GroupID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
}

func (ConversationModel) TableName() string { return "conversations" }

// MessageModel maps to the `messages` table.
type MessageModel struct {
	ID             string    `gorm:"primaryKey;column:id"`
	ConversationID string    `gorm:"column:conversation_id;not null;index:idx_msg_conv_created,priority:1"`
	Role           string    `gorm:"column:role;index:idx_msg_conv_role,priority:2"`
	Content        string    `gorm:"column:content"`
	AudioData      []byte    `gorm:"column:audio_data"`
	IsCompaction   bool      `gorm:"column:is_compaction;default:false"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime:false;index:idx_msg_conv_created,priority:2"`
	// Attachments associated with this message (metadata only)
	Attachments []MessageAttachmentModel `gorm:"foreignKey:MessageID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"attachments,omitempty"`

	// Cascade: deleting a conversation should remove its messages
	Conversation ConversationModel `gorm:"foreignKey:ConversationID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
}

func (MessageModel) TableName() string { return "messages" }

// MessageAttachmentModel maps to the `message_attachments` table and stores
// only metadata about attachments. Raw bytes are never persisted here.
type MessageAttachmentModel struct {
	ID        uint   `gorm:"primaryKey;column:id;autoIncrement"`
	MessageID string `gorm:"column:message_id;index;not null"`
	Type      string `gorm:"column:type"`
	Filename  string `gorm:"column:filename"`
	MIMEType  string `gorm:"column:mime_type"`
	Size      int64  `gorm:"column:size"`
}

func (MessageAttachmentModel) TableName() string { return "message_attachments" }

// TaskModel maps to the `tasks` table.
type TaskModel struct {
	ID         string     `gorm:"primaryKey;column:id"`
	Prompt     string     `gorm:"column:prompt;not null"`
	Schedule   string     `gorm:"column:schedule"`
	TaskType   string     `gorm:"column:task_type;not null;default:'one-shot'"`
	Enabled    bool       `gorm:"column:enabled;not null;default:true"`
	Status     string     `gorm:"column:status;default:'pending';index"`
	AddedAt    time.Time  `gorm:"column:added_at;autoCreateTime:false"`
	FinishedAt *time.Time `gorm:"column:finished_at"`
}

func (TaskModel) TableName() string { return "tasks" }

// PairingModel maps to the `pairings` table.
type PairingModel struct {
	Code             string    `gorm:"primaryKey;column:code"`
	ChannelID        string    `gorm:"column:channel_id;not null"`
	PlatformUserID   string    `gorm:"column:platform_user_id"`
	PlatformUserName string    `gorm:"column:platform_user_name;not null;default:''"`
	ChannelType      string    `gorm:"column:channel_type;not null;default:''"`
	ExpiresAt        time.Time `gorm:"column:expires_at;index"`
	Status           string    `gorm:"column:status"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

func (PairingModel) TableName() string { return "pairings" }

// MCPServerModel maps to the `mcp_servers` table.
type MCPServerModel struct {
	Name    string    `gorm:"primaryKey;column:name"`
	URL     string    `gorm:"column:url;not null"`
	AddedAt time.Time `gorm:"column:added_at;not null;autoCreateTime:false"`
}

func (MCPServerModel) TableName() string { return "mcp_servers" }

// ToolPermissionModel maps to the `tool_permissions` table.
type ToolPermissionModel struct {
	UserID    string    `gorm:"primaryKey;column:user_id"`
	ToolName  string    `gorm:"primaryKey;column:tool_name"`
	Mode      string    `gorm:"column:mode;not null;default:'deny'"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime:false"`
	// Cascade: remove permissions when user deleted
	User UserModel `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;" json:"-"`
}

func (ToolPermissionModel) TableName() string { return "tool_permissions" }
