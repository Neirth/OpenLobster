package events

import (
	"time"
)

type Event interface {
	GetType() string
	GetTimestamp() time.Time
	GetPayload() interface{}
}

type BaseEvent struct {
	eventType string
	timestamp time.Time
	payload   interface{}
}

func (e *BaseEvent) GetType() string {
	return e.eventType
}

func (e *BaseEvent) GetTimestamp() time.Time {
	return e.timestamp
}

func (e *BaseEvent) GetPayload() interface{} {
	return e.payload
}

func NewEvent(eventType string, payload interface{}) Event {
	return &BaseEvent{
		eventType: eventType,
		timestamp: time.Now(),
		payload:   payload,
	}
}

const (
	EventMessageReceived       = "message_received"
	EventMessageSent           = "message_sent"
	EventMessageProcessed      = "message_processed"
	EventSessionStarted        = "session_started"
	EventSessionEnded          = "session_ended"
	EventUserPaired            = "user_paired"
	EventUserUnpaired          = "user_unpaired"
	EventPairingRequested      = "pairing_requested"
	EventPairingApproved       = "pairing_approved"
	EventPairingDenied         = "pairing_denied"
	EventTaskAdded             = "task_added"
	EventTaskCompleted         = "task_completed"
	EventCronJobExecuted       = "cron_job_executed"
	EventMCPServerConnected    = "mcp_server_connected"
	EventMCPServerDisconnected = "mcp_server_disconnected"
	EventMemoryUpdated         = "memory_updated"
	EventCompactionTriggered   = "compaction_triggered"
	EventCompactionCompleted   = "compaction_completed"
)

type MessageReceivedPayload struct {
	MessageID string
	ChannelID string
	Content   string
	Timestamp time.Time
}

type MessageSentPayload struct {
	MessageID   string
	ChannelID   string
	ChannelType string // "telegram", "discord", "loopback", etc. — frontend filters loopback
	RecipientID string
	Content     string
	Role        string
	Timestamp   time.Time
}

// Include attachment metadata in message sent events so subscribers can
// show that a message contained attachments (no raw bytes are published).
type AttachmentMetadata struct {
	Type     string
	Filename string
	MIMEType string
	Size     int64
}

// Extend MessageSentPayload with Attachments metadata.
type MessageSentPayloadWithAttachments struct {
	MessageID   string
	ChannelID   string
	ChannelType string
	RecipientID string
	Content     string
	Role        string
	Timestamp   time.Time
	Attachments []AttachmentMetadata
}

type SessionStartedPayload struct {
	SessionID   string
	UserID      string
	ChannelType string
	ModelID     string
	Timestamp   time.Time
}

type SessionEndedPayload struct {
	SessionID string
	UserID    string
	Duration  time.Duration
	Timestamp time.Time
}

type UserPairedPayload struct {
	UserID      string
	ChannelID   string
	PairingCode string
	Timestamp   time.Time
}

type PairingRequestedPayload struct {
	RequestID   string    `json:"requestID"`
	Code        string    `json:"code"`
	ChannelID   string    `json:"channelID"`
	ChannelType string    `json:"channelType"`
	DisplayName string    `json:"displayName"`
	Timestamp   time.Time `json:"timestamp"`
}

type PairingApprovedPayload struct {
	RequestID  string    `json:"requestID"`
	Code       string    `json:"code"`
	ApprovedBy string    `json:"approvedBy"`
	Timestamp  time.Time `json:"timestamp"`
}

type PairingDeniedPayload struct {
	RequestID string    `json:"requestID"`
	Code      string    `json:"code"`
	DeniedBy  string    `json:"deniedBy"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

type TaskCompletedPayload struct {
	TaskID    string
	Prompt    string
	Duration  time.Duration
	Timestamp time.Time
}

type MemoryUpdatedPayload struct {
	UserID    string
	Operation string
	Content   string
	Timestamp time.Time
}

type CompactionPayload struct {
	ConversationID string
	MessageCount   int
	SummaryLength  int
	Timestamp      time.Time
}
