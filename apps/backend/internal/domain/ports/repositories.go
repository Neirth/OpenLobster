package ports

import (
	"context"
	"errors"

	"github.com/neirth/openlobster/internal/domain/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

type MemoryPort interface {
	// AddKnowledge stores a new fact for a user.
	// label is a short meaningful name for the fact node (e.g. "Electronica").
	// relation is the edge label from the user node to this fact (e.g. "LIKES").
	// Both default to sensible values when empty.
	AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, embedding []float64) error
	SearchSimilar(ctx context.Context, query string, limit int) ([]Knowledge, error)
	GetUserGraph(ctx context.Context, userID string) (Graph, error)
	AddRelation(ctx context.Context, from, to string, relType string) error
	QueryGraph(ctx context.Context, cypher string) (GraphResult, error)
	InvalidateMemoryCache(ctx context.Context, userID string) error
	// SetUserProperty upserts an arbitrary key/value property on the user node
	// in the memory graph. This lets the LLM persist structured attributes
	// (e.g. preferred language, occupation) independently of free-text facts.
	SetUserProperty(ctx context.Context, userID, key, value string) error
	// EditMemoryNode updates the text value of an existing fact node owned by userID.
	EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error
	// DeleteMemoryNode removes a fact node owned by userID from the graph.
	DeleteMemoryNode(ctx context.Context, userID, nodeID string) error
	// UpdateUserLabel sets the human-readable label on the user node (e.g. the
	// display name). Safe to call if the node does not exist yet.
	UpdateUserLabel(ctx context.Context, userID, displayName string) error
}

type Knowledge struct {
	ID        string
	UserID    string
	Content   string
	Embedding []float64
	CreatedAt interface{}
}

type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

type GraphNode struct {
	ID         string
	Label      string
	Type       string
	Value      string
	Properties map[string]string
}

type GraphEdge struct {
	Source string
	Target string
	Label  string
}

type GraphResult struct {
	Data   []map[string]interface{}
	Errors []error
}

type SecretsPort interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

type TaskRepositoryPort interface {
	GetPending(ctx context.Context) ([]models.Task, error)
	ListAll(ctx context.Context) ([]models.Task, error)
	Add(ctx context.Context, task *models.Task) error
	MarkDone(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, task *models.Task) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
}

type MessageRepositoryPort interface {
	Save(ctx context.Context, message *models.Message) error
	GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error)
	GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error)
	GetLastCompaction(ctx context.Context, conversationID string) (*models.Message, error)
}

type SessionRepositoryPort interface {
	Create(ctx context.Context, session *models.Session) error
	GetByID(ctx context.Context, id string) (*models.Session, error)
	Update(ctx context.Context, session *models.Session) error
	GetActiveByUser(ctx context.Context, userID string) ([]models.Session, error)
	// GetActiveByChannel is kept for DM lookups where we only have the platform channel ID.
	GetActiveByChannel(ctx context.Context, channelID string) ([]models.Session, error)
	// GetActiveByGroup returns active conversations for a group chat (by groups.id UUID).
	GetActiveByGroup(ctx context.Context, groupID string) ([]models.Session, error)
}

type CronJobRepositoryPort interface {
	Create(ctx context.Context, job *models.CronJob) error
	GetByID(ctx context.Context, id string) (*models.CronJob, error)
	GetAll(ctx context.Context) ([]models.CronJob, error)
	Update(ctx context.Context, job *models.CronJob) error
	Delete(ctx context.Context, id string) error
}

type PairingRepositoryPort interface {
	Create(ctx context.Context, pairing *Pairing) error
	GetByCode(ctx context.Context, code string) (*Pairing, error)
	UpdateStatus(ctx context.Context, code string, status string) error
	UpdateStatusIfPending(ctx context.Context, code string, newStatus string) (bool, error)
	DeleteExpired(ctx context.Context) error
	// ListActive returns all pairings that have not yet expired.
	ListActive(ctx context.Context) ([]Pairing, error)
}

type Pairing struct {
	Code             string
	ChannelID        string
	PlatformUserID   string
	PlatformUserName string
	ChannelType      string
	ExpiresAt        int64
	Status           string
	CreatedAt        int64
}

type UserRepositoryPort interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByPrimaryID(ctx context.Context, primaryID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	ListAll(ctx context.Context) ([]models.User, error)
}

// UserChannelRepositoryPort manages the user_channels table, which binds an
// internal users.id to a platform-specific user identity (channel + platform_user_id).
type UserChannelRepositoryPort interface {
	// ExistsByPlatformUserID returns true when at least one user_channels row
	// references the given platform-native user ID.
	ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error)
	// GetUserIDByPlatformUserID resolves the users.id UUID for a given platform
	// user ID. Returns an empty string (no error) when no binding exists yet.
	GetUserIDByPlatformUserID(ctx context.Context, platformUserID string) (string, error)
	// GetDisplayNameByPlatformUserID returns the human-readable name stored for
	// the given platform user ID. Returns an empty string when not found.
	GetDisplayNameByPlatformUserID(ctx context.Context, platformUserID string) (string, error)
	// GetDisplayNameByUserID returns the first display_name found for the given
	// internal users.id UUID. Returns an empty string when not found.
	GetDisplayNameByUserID(ctx context.Context, userID string) (string, error)
	// Create inserts a new user_channels binding.
	// channelType is the platform slug (e.g. "telegram"); platformUserID is the
	// platform-native user identifier (e.g. Telegram numeric user ID as string).
	// username is the platform-reported handle (e.g. Telegram @username).
	Create(ctx context.Context, userID, channelType, platformUserID, username string) error
	// GetLastChannelForUser returns the (channelType, platformChannelID) for the
	// most recently used channel with the given user. Used to route send_message
	// to the last channel through which the bot and user communicated.
	GetLastChannelForUser(ctx context.Context, userID string) (channelType, platformChannelID string, err error)
	// UpdateLastSeen updates last_seen for the given (channelType, platformUserID).
	// Call when the user sends a message so GetLastChannelForUser returns the correct channel.
	UpdateLastSeen(ctx context.Context, channelType, platformUserID string) error
}

// ChannelRepositoryPort manages the channels table (one row per platform).
type ChannelRepositoryPort interface {
	// EnsurePlatform creates a channels row for the given platform slug if it
	// does not already exist.
	EnsurePlatform(ctx context.Context, platformSlug, name string) error
	GetByID(ctx context.Context, id string) (*Channel, error)
}

type Channel struct {
	ID        string
	Type      string
	Name      string
	CreatedAt int64
}

// GroupRepositoryPort manages the groups and group_users tables.
type GroupRepositoryPort interface {
	// GetOrCreate looks up a group by (channelType, platformGroupID) and creates
	// it if it does not exist. Returns the internal UUID of the group.
	GetOrCreate(ctx context.Context, channelType, platformGroupID, name string) (string, error)
	// AddMember ensures a user_id is registered in group_users for the group.
	AddMember(ctx context.Context, groupID, userID string) error
	// GetByID returns the group for the given internal UUID.
	GetByID(ctx context.Context, id string) (*Group, error)
	// GetMembers returns the users.id UUIDs of all known members of a group.
	GetMembers(ctx context.Context, groupID string) ([]string, error)
}

type Group struct {
	ID              string
	ChannelID       string
	PlatformGroupID string
	Name            string
	CreatedAt       int64
}
