package models

type ChannelType string

const (
	ChannelDiscord  ChannelType = "discord"
	ChannelWhatsApp ChannelType = "whatsapp"
	ChannelTelegram ChannelType = "telegram"
	ChannelTwilio   ChannelType = "twilio"
	ChannelSlack    ChannelType = "slack"
	// ChannelLoopback is the virtual channel used by the Scheduler for
	// system-initiated agentic executions. It has no external messaging
	// adapter; execution is ephemeral and no conversation history is
	// persisted to the database.
	ChannelLoopback ChannelType = "loopback"
)
