package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID              `json:"id"`
	ChannelID      string                 `json:"channel_id"`
	SenderName     string                 `json:"sender_name"`
	SenderID       string                 `json:"sender_id,omitempty"`
	IsGroup        bool                   `json:"is_group,omitempty"`
	IsMentioned    bool                   `json:"is_mentioned,omitempty"`
	GroupName      string                 `json:"group_name,omitempty"`
	Content        string                 `json:"content"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata"`
	Attachments    []Attachment           `json:"attachments"`
	RawPayload     json.RawMessage        `json:"raw_payload"`
	IsReply        bool                   `json:"is_reply"`
	ReplyToID      *uuid.UUID             `json:"reply_to_id"`
	Audio          *AudioContent          `json:"audio,omitempty"`
	Role           string                 `json:"role"`
	ConversationID string                 `json:"conversation_id"`
}

type AudioContent struct {
	Data           []byte        `json:"data"`
	Format         string        `json:"format"`
	Duration       time.Duration `json:"duration"`
	PlatformFormat string        `json:"platform_format"`
}

type Attachment struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MIMEType string `json:"mime_type"`
}

func NewMessage(channelID, content string) *Message {
	return &Message{
		ID:        uuid.New(),
		ChannelID: channelID,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

func (m *Message) SetReplyTo(id uuid.UUID) {
	m.IsReply = true
	m.ReplyToID = &id
}

func (m *Message) IsSystemMessage() bool {
	return m.Role == "system"
}

func (m *Message) IsEmpty() bool {
	return len(m.Content) == 0
}

type SessionType string

const (
	SessionTypeDM    SessionType = "dm"
	SessionTypeGroup SessionType = "group"
)

type Session struct {
	ID          uuid.UUID                `json:"id"`
	Type        SessionType              `json:"type"`
	UserID      string                   `json:"user_id"`
	GroupID     *uuid.UUID               `json:"group_id,omitempty"`
	ChannelType ChannelType `json:"channel_type"`
	ChannelID   string                   `json:"channel_id"`
	ModelID     string                   `json:"model_id"`
	Messages    []Message                `json:"messages"`
	Context     MessageContext           `json:"context"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	IsActive    bool                     `json:"is_active"`
}

type MessageContext struct {
	AgentsMD   string `json:"agents_md"`
	SoulMD     string `json:"soul_md"`
	IdentityMD string `json:"identity_md"`
}

func NewSession(userID string) *Session {
	now := time.Now()
	return &Session{
		ID:        uuid.New(),
		Type:      SessionTypeDM,
		UserID:    userID,
		ChannelID: "",
		Messages:  make([]Message, 0),
		Context:   MessageContext{},
		CreatedAt: now,
		UpdatedAt: now,
		IsActive:  true,
	}
}

func NewGroupSession(channelType ChannelType, channelID string, groupID uuid.UUID) *Session {
	now := time.Now()
	return &Session{
		ID:          uuid.New(),
		Type:        SessionTypeGroup,
		GroupID:     &groupID,
		ChannelType: channelType,
		ChannelID:   channelID,
		Messages:    make([]Message, 0),
		Context:     MessageContext{},
		CreatedAt:   now,
		UpdatedAt:   now,
		IsActive:    true,
	}
}

func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

func (s *Session) MarkInactive() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

type User struct {
	ID        uuid.UUID     `json:"id"`
	PrimaryID string        `json:"primary_id"`
	Name      string        `json:"name"`
	Channels  []UserChannel `json:"channels"`
	Memory    *UserMemory   `json:"memory"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type UserChannel struct {
	ChannelType   ChannelType `json:"channel_type"`
	ChannelUserID string                   `json:"channel_user_id"`
	Username      string                   `json:"username"`
	DisplayName   string                   `json:"display_name"`
	FirstSeen     time.Time                `json:"first_seen"`
	LastSeen      time.Time                `json:"last_seen"`
}

type UserMemory struct {
	ShortTerm   []Message              `json:"short_term"`
	LongTerm    *GraphMemory           `json:"long_term"`
	Facts       []Fact                 `json:"facts"`
	Preferences map[string]interface{} `json:"preferences"`
}

type Fact struct {
	Statement  string    `json:"statement"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
}

type GraphMemory struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Type       string            `json:"type"`
	Value      string            `json:"value"`
	Properties map[string]string `json:"properties"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
}

func NewUser(primaryID string) *User {
	now := time.Now()
	return &User{
		ID:        uuid.New(),
		PrimaryID: primaryID,
		Channels:  make([]UserChannel, 0),
		Memory: &UserMemory{
			ShortTerm:   make([]Message, 0),
			Facts:       make([]Fact, 0),
			Preferences: make(map[string]interface{}),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (u *User) AddChannel(ch UserChannel) {
	u.Channels = append(u.Channels, ch)
	u.UpdatedAt = time.Now()
}

func (u *User) AddFact(statement string, confidence float64, source string) {
	fact := Fact{
		Statement:  statement,
		Confidence: confidence,
		Source:     source,
		CreatedAt:  time.Now(),
	}
	u.Memory.Facts = append(u.Memory.Facts, fact)
	u.UpdatedAt = time.Now()
}
