// Package slack provides a Slack messaging adapter for OpenLobster.
//
// The adapter uses the Socket Mode API (via github.com/slack-go/slack) so that
// no public webhook URL is required. It listens to all messages in channels
// where the bot is a member and to direct messages.
//
// Required Slack App scopes:
//   - bot token (xoxb-…): channels:history, groups:history, im:history,
//     mpim:history, chat:write, users:read, reactions:write
//   - app-level token (xapp-…): connections:write  (for Socket Mode)
//
// # License
// See LICENSE in the root of the repository.
package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// Adapter implements ports.MessagingPort for Slack via Socket Mode.
type Adapter struct {
	client    *slackapi.Client
	sm        *socketmode.Client
	botUserID string
}

// NewAdapter creates a new Slack adapter.
//
//   - botToken is the Bot User OAuth Token (starts with "xoxb-").
//   - appToken is the App-Level Token (starts with "xapp-"), required for
//     Socket Mode. Generate it in Slack app settings → Basic Information →
//     App-Level Tokens with the connections:write scope.
func NewAdapter(botToken, appToken string) (*Adapter, error) {
	client := slackapi.New(
		botToken,
		slackapi.OptionAppLevelToken(appToken),
	)

	sm := socketmode.New(
		client,
		socketmode.OptionDebug(false),
	)

	return &Adapter{client: client, sm: sm}, nil
}

// Start connects to the Slack Socket Mode gateway, resolves the bot's own user
// ID, and dispatches incoming messages to onMessage. The loop runs until ctx
// is cancelled.
func (a *Adapter) Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error {
	authResp, err := a.client.AuthTestContext(ctx)
	if err != nil {
		return fmt.Errorf("slack auth test: %w", err)
	}
	a.botUserID = authResp.UserID

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-a.sm.Events:
				if !ok {
					return
				}
				a.handleEvent(ctx, evt, onMessage)
			}
		}
	}()

	go func() {
		if err := a.sm.RunContext(ctx); err != nil && ctx.Err() == nil {
			// Only log if the context was not intentionally cancelled.
			_ = err
		}
	}()

	return nil
}

// handleEvent processes a single Socket Mode event and, for message events,
// dispatches to onMessage.
func (a *Adapter) handleEvent(ctx context.Context, evt socketmode.Event, onMessage func(context.Context, *models.Message)) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		a.sm.Ack(*evt.Request)
		eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		if eventsAPI.Type != slackevents.CallbackEvent {
			return
		}
		innerEvent := eventsAPI.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			a.dispatchMessage(ctx, ev, onMessage)
		}
	}
}

// dispatchMessage converts a Slack MessageEvent to models.Message.
func (a *Adapter) dispatchMessage(ctx context.Context, ev *slackevents.MessageEvent, onMessage func(context.Context, *models.Message)) {
	// Ignore messages from bots (including this bot itself) and sub-types like
	// message edits or channel join notifications.
	if ev.BotID != "" || ev.SubType != "" {
		return
	}
	if ev.User == "" || ev.User == a.botUserID {
		return
	}

	// DMs have channel IDs starting with "D"; group DMs with "G" or "C".
	isGroup := !strings.HasPrefix(ev.Channel, "D")

	// Detect whether the bot is explicitly mentioned in the message.
	isMentioned := strings.Contains(ev.Text, "<@"+a.botUserID+">")

	// Resolve the sender's display name.
	senderName := ev.User
	if info, err := a.client.GetUserInfoContext(ctx, ev.User); err == nil {
		if info.Profile.DisplayName != "" {
			senderName = info.Profile.DisplayName
		} else if info.Profile.RealName != "" {
			senderName = info.Profile.RealName
		}
	}

	// Strip the bot mention from the content for cleaner processing.
	content := strings.TrimSpace(strings.ReplaceAll(ev.Text, "<@"+a.botUserID+">", ""))

	// Resolve the channel/group name for group conversations.
	groupName := ""
	if isGroup {
		if ch, err := a.client.GetConversationInfoContext(ctx, &slackapi.GetConversationInfoInput{
			ChannelID: ev.Channel,
		}); err == nil {
			groupName = ch.Name
		}
	}

	ts := time.Now()
	if ev.TimeStamp != "" {
		// Slack timestamps are Unix seconds with a decimal fraction ("1234567890.123456").
		var sec, usec int64
		fmt.Sscanf(ev.TimeStamp, "%d.%d", &sec, &usec)
		if sec > 0 {
			ts = time.Unix(sec, usec*int64(time.Microsecond))
		}
	}

	msg := &models.Message{
		ID:          uuid.New(),
		ChannelID:   ev.Channel,
		SenderID:    ev.User,
		SenderName:  senderName,
		IsGroup:     isGroup,
		IsMentioned: isMentioned,
		GroupName:   groupName,
		Content:     content,
		Timestamp:   ts,
		Metadata:    map[string]interface{}{"slack_ts": ev.TimeStamp},
	}
	onMessage(ctx, msg)
}

// SendTyping is a no-op for Slack (typing not implemented).
func (a *Adapter) SendTyping(_ context.Context, _ string) error { return nil }

// SendMessage sends a plain text (Markdown-rendered) message to a Slack channel.
func (a *Adapter) SendMessage(ctx context.Context, msg *models.Message) error {
	_, _, err := a.client.PostMessageContext(
		ctx,
		msg.ChannelID,
		slackapi.MsgOptionText(msg.Content, false),
	)
	if err != nil {
		return fmt.Errorf("slack send message: %w", err)
	}
	return nil
}

// SendMedia sends a message with an optional image URL attachment.
func (a *Adapter) SendMedia(ctx context.Context, media *ports.Media) error {
	opts := []slackapi.MsgOption{slackapi.MsgOptionText(media.Caption, false)}
	if media.URL != "" {
		opts = append(opts, slackapi.MsgOptionAttachments(slackapi.Attachment{
			ImageURL: media.URL,
			Text:     media.Caption,
		}))
	}
	_, _, err := a.client.PostMessageContext(ctx, media.ChatID, opts...)
	if err != nil {
		return fmt.Errorf("slack send media: %w", err)
	}
	return nil
}

// HandleWebhook is a no-op for Slack when using Socket Mode.
func (a *Adapter) HandleWebhook(_ context.Context, _ []byte) (*models.Message, error) {
	return nil, nil
}

// GetUserInfo retrieves basic user information by Slack user ID.
func (a *Adapter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	info, err := a.client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return &ports.UserInfo{ID: userID, Username: userID, DisplayName: userID}, nil
	}
	displayName := info.Profile.DisplayName
	if displayName == "" {
		displayName = info.Profile.RealName
	}
	if displayName == "" {
		displayName = info.Name
	}
	return &ports.UserInfo{
		ID:          userID,
		Username:    info.Name,
		DisplayName: displayName,
	}, nil
}

// React adds an emoji reaction to a Slack message. messageID must be in the
// format "channelID:timestamp" (e.g. "C1234567890:1234567890.123456").
func (a *Adapter) React(ctx context.Context, messageID string, emoji string) error {
	parts := strings.SplitN(messageID, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("slack react: invalid message id format (expected 'channelID:timestamp'): %q", messageID)
	}
	ref := slackapi.NewRefToMessage(parts[0], parts[1])
	// Remove surrounding colons from emoji name if present (":thumbsup:" → "thumbsup").
	emoji = strings.Trim(emoji, ":")
	if err := a.client.AddReactionContext(ctx, emoji, ref); err != nil {
		return fmt.Errorf("slack react: %w", err)
	}
	return nil
}

// GetCapabilities returns capability flags for the Slack channel.
func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{
		HasVoiceMessage: false,
		HasCallStream:   false,
		HasTextStream:   true,
		HasMediaSupport: true,
	}
}

// ConvertAudioForPlatform returns the audio data unchanged; Slack does not
// require a specific audio transcoding.
func (a *Adapter) ConvertAudioForPlatform(_ context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, format, nil
}

var _ ports.MessagingPort = (*Adapter)(nil)
