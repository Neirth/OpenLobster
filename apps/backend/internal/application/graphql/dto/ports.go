package dto

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/models"
)

// MessageRepo expone operaciones sobre mensajes.
type MessageRepo interface {
	Save(ctx context.Context, msg *models.Message) error
	GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error)
	GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]models.Message, error)
	GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error)
	CountMessages(ctx context.Context) (recv, sent int64, err error)
}

// ConversationPort expone conversaciones.
type ConversationPort interface {
	ListConversations() ([]ConversationSnapshot, error)
	DeleteUser(ctx context.Context, conversationID string) error
}

// SkillsPort expone operaciones sobre skills.
type SkillsPort interface {
	ListSkills() ([]SkillSnapshot, error)
	EnableSkill(name string) error
	DisableSkill(name string) error
	ImportSkill(data []byte) error
	DeleteSkill(name string) error
}

// SystemFilesPort expone archivos del sistema.
type SystemFilesPort interface {
	ListFiles() ([]SystemFileSnapshot, error)
	WriteFile(name, content string) error
}

// ToolPermissionsRepo expone permisos de herramientas.
type ToolPermissionsRepo interface {
	Set(ctx context.Context, userID, toolName, mode string) error
	Delete(ctx context.Context, userID, toolName string) error
	ListByUser(ctx context.Context, userID string) ([]ToolPermissionRecord, error)
	ListAll(ctx context.Context) ([]ToolPermissionRecord, error)
}

// ToolNamesSource devuelve los nombres de todas las herramientas (para Deny/Allow All).
type ToolNamesSource interface {
	AllToolNames() []string
}

// MCPServerRepo expone servidores MCP.
type MCPServerRepo interface {
	Save(ctx context.Context, name, url string) error
	Delete(ctx context.Context, name string) error
	ListAll(ctx context.Context) ([]MCPServerRecord, error)
}

// McpConnectPort performs the actual MCP connection (connect + persist + register tools).
// Connect returns (requiresAuth, err). If requiresAuth is true, the server needs OAuth before connecting.
// GetConnectionStatus returns "online" if the server has tools registered, "unknown" otherwise.
// GetServerToolCount returns the number of tools exposed by the server (0 if not connected).
type McpConnectPort interface {
	Connect(ctx context.Context, name, transport, url string) (requiresAuth bool, err error)
	Disconnect(ctx context.Context, name string) error
	GetConnectionStatus(name string) string
	GetServerToolCount(name string) int
}

// McpOAuthPort handles OAuth initiation and status for MCP servers.
type McpOAuthPort interface {
	InitiateOAuth(ctx context.Context, serverName, mcpURL string) (authURL string, err error)
	Status(serverName string) (status, errMsg string)
}

// SubAgentPort expone sub-agentes.
type SubAgentPort interface {
	List(ctx context.Context) ([]SubAgentSnapshot, error)
	Spawn(ctx context.Context, name, model, task string) (string, error)
	Kill(ctx context.Context, id string) error
}

// PairingPort expone operaciones de pairing.
type PairingPort interface {
	Approve(ctx context.Context, code, userID, displayName string) (*PairingSnapshot, error)
	Deny(ctx context.Context, code, reason string) error
	ListActive(ctx context.Context) ([]PairingSnapshot, error)
}

// UserRepo expone usuarios.
type UserRepo interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	ListAll(ctx context.Context) ([]models.User, error)
}

// UserChannelRepo expone user-channel.
type UserChannelRepo interface {
	ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error)
	Create(ctx context.Context, userID, channelType, platformUserID, username string) error
	GetDisplayNameByUserID(ctx context.Context, userID string) (string, error)
}

// MessageSender envía mensajes a canales.
type MessageSender interface {
	SendTextToChannel(ctx context.Context, channelType, channelID, text string) error
}

// EventBusPort publica eventos.
type EventBusPort interface {
	Publish(ctx context.Context, eventType string, payload interface{}) error
}

// ConfigUpdatePort persiste cambios de configuración y notifica qué canales se afectaron.
// El caller puede recargar esos canales en caliente.
type ConfigUpdatePort interface {
	Apply(ctx context.Context, input map[string]interface{}) (changedChannels []string, err error)
}
