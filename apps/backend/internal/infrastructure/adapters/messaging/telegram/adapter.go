package telegram

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// ---------------------------------------------------------------------------
// Markdown → Telegram HTML conversion
// ---------------------------------------------------------------------------

var (
	// Fenced code blocks: ```lang\ncontent\n``` or ```\ncontent\n```
	reFencedCode = regexp.MustCompile("(?s)```([a-zA-Z0-9+._-]*)\\n(.*?)\\n?```")
	// Inline code: `content`
	reInlineCode = regexp.MustCompile("`([^`]+)`")
	// Bold: **text** or __text__
	reBoldDouble = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldUnder  = regexp.MustCompile(`__(.+?)__`)
	// Italic: *text* or _text_ (not preceded/followed by another * or _).
	// Content must not contain < or > to avoid matching across existing HTML tags
	// (which would produce invalid nesting like <b>...<i>...</b>...</i>).
	reItalicStar  = regexp.MustCompile(`(?:^|[^*])\*([^*\n<>]+?)\*(?:[^*]|$)`)
	reItalicUnder = regexp.MustCompile(`(?:^|[^_])_([^_\n<>]+?)_(?:[^_]|$)`)
	// Strikethrough: ~~text~~
	reStrike = regexp.MustCompile(`~~(.+?)~~`)
	// Links: [text](url)
	reLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	// ATX headings: # H1 / ## H2 / ### H3 at line start
	reHeading = regexp.MustCompile(`(?m)^#{1,3} (.+)$`)
	// Horizontal rules
	reHRule = regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`)
)

// markdownToHTML converts a subset of Markdown to Telegram-compatible HTML.
// Code blocks and inline code are preserved verbatim; everything else goes
// through a light-weight inline formatter.
func markdownToHTML(text string) string {
	var sb strings.Builder

	// Split on fenced code blocks first to avoid double-processing their content.
	parts := reFencedCode.Split(text, -1)
	matches := reFencedCode.FindAllStringSubmatch(text, -1)

	for i, part := range parts {
		// Format the plain-text part.
		sb.WriteString(formatInline(part))
		// Append the code block that follows (if any).
		if i < len(matches) {
			lang := matches[i][1]
			code := html.EscapeString(matches[i][2])
			if lang != "" {
				sb.WriteString("<pre><code class=\"language-")
				sb.WriteString(html.EscapeString(lang))
				sb.WriteString("\">")
				sb.WriteString(code)
				sb.WriteString("</code></pre>")
			} else {
				sb.WriteString("<pre>")
				sb.WriteString(code)
				sb.WriteString("</pre>")
			}
		}
	}
	return sb.String()
}

// formatInline applies inline Markdown formatting to a plain-text segment.
// The function operates on segments that contain no fenced code blocks.
func formatInline(s string) string {
	if s == "" {
		return ""
	}

	// Process inline code first (protects its content from further substitution).
	var codeParts []string
	s = reInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		sub := reInlineCode.FindStringSubmatch(m)
		placeholder := "\x00CODE" + strconv.Itoa(len(codeParts)) + "\x00"
		codeParts = append(codeParts, "<code>"+html.EscapeString(sub[1])+"</code>")
		return placeholder
	})

	// HTML-escape the remaining plain-text parts.
	s = htmlEscapeExceptPlaceholders(s)

	// Apply block-level formatting (headings, horizontal rules).
	s = reHeading.ReplaceAllString(s, "<b>$1</b>")
	s = reHRule.ReplaceAllString(s, "")

	// Apply inline formatting in order of precedence (longest delimiters first).
	s = reStrike.ReplaceAllString(s, "<s>$1</s>")
	s = reBoldDouble.ReplaceAllString(s, "<b>$1</b>")
	s = reBoldUnder.ReplaceAllString(s, "<b>$1</b>")

	// Italic: replace the outer capture only; preserve surrounding chars.
	s = reItalicStar.ReplaceAllStringFunc(s, func(m string) string {
		sub := reItalicStar.FindStringSubmatch(m)
		return strings.Replace(m, "*"+sub[1]+"*", "<i>"+sub[1]+"</i>", 1)
	})
	s = reItalicUnder.ReplaceAllStringFunc(s, func(m string) string {
		sub := reItalicUnder.FindStringSubmatch(m)
		return strings.Replace(m, "_"+sub[1]+"_", "<i>"+sub[1]+"</i>", 1)
	})

	// Links.
	s = reLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	// Restore inline code placeholders.
	for i, code := range codeParts {
		s = strings.ReplaceAll(s, "\x00CODE"+strconv.Itoa(i)+"\x00", code)
	}

	return s
}

// htmlEscapeExceptPlaceholders HTML-escapes &, < and > but skips placeholder
// byte sequences (which contain NUL bytes) to avoid double-encoding them.
func htmlEscapeExceptPlaceholders(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x00' {
			// Placeholder: copy until closing \x00.
			end := strings.Index(s[i+1:], "\x00")
			if end >= 0 {
				sb.WriteString(s[i : i+end+2])
				i += end + 2
				continue
			}
		}
		switch s[i] {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		default:
			sb.WriteByte(s[i])
		}
		i++
	}
	return sb.String()
}

type Adapter struct {
	bot         *tgbotapi.BotAPI
	token       string
	chatID      int64
	botUserID   int64  // populated after first successful Me() call
	botUsername string // e.g. "mybot" (without @)
}

func NewAdapter(token string) (*Adapter, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	a := &Adapter{bot: bot, token: token}
	// Fetch the bot's own identity so we can detect mentions and replies.
	if me, err := bot.GetMe(); err == nil {
		a.botUserID = me.ID
		a.botUsername = strings.ToLower(me.UserName)
	}
	return a, nil
}

// downloadFile resolves a Telegram fileID via the Bot API and downloads the
// raw bytes. Returns nil on any error.
func (a *Adapter) downloadFile(fileID string) []byte {
	if fileID == "" {
		return nil
	}
	cfg := tgbotapi.FileConfig{FileID: fileID}
	file, err := a.bot.GetFile(cfg)
	if err != nil {
		return nil
	}
	resp, err := http.Get(file.Link(a.token)) //nolint:noctx
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	return data
}

// buildAttachments extracts Telegram media (photo, voice, audio, document) from
// a message and returns a slice of domain Attachment values together with the
// resolved text content (Text or Caption).
// resolveFile is called with each Telegram fileID to obtain (rawBytes, url);
// pass a.downloadFile in production and a stub in tests.
func buildAttachments(tgMsg *tgbotapi.Message, resolveFile func(fileID string) []byte) (text string, attachments []models.Attachment) {
	// Text messages carry content in Text; media messages carry it in Caption.
	text = tgMsg.Text
	if text == "" {
		text = tgMsg.Caption
	}

	// Photo: Telegram sends multiple sizes; take the largest (last element).
	if photos := tgMsg.Photo; len(photos) > 0 {
		largest := photos[len(photos)-1]
		data := resolveFile(largest.FileID)
		attachments = append(attachments, models.Attachment{
			Type:     "image",
			MIMEType: "image/jpeg",
			Data:     data,
		})
	}

	// Voice message (OGG/Opus recorded in-app).
	if v := tgMsg.Voice; v != nil {
		data := resolveFile(v.FileID)
		attachments = append(attachments, models.Attachment{
			Type:     "audio",
			MIMEType: "audio/ogg",
			Size:     int64(v.FileSize),
			Data:     data,
		})
	}

	// Audio file (MP3, FLAC, etc. sent as audio).
	if au := tgMsg.Audio; au != nil {
		mimeType := au.MimeType
		if mimeType == "" {
			mimeType = "audio/mpeg"
		}
		data := resolveFile(au.FileID)
		attachments = append(attachments, models.Attachment{
			Type:     "audio",
			MIMEType: mimeType,
			Filename: au.FileName,
			Size:     int64(au.FileSize),
			Data:     data,
		})
	}

	// Generic document (PDF, video note, sticker, etc.).
	if doc := tgMsg.Document; doc != nil {
		mimeType := doc.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		attType := "document"
		if strings.HasPrefix(mimeType, "image/") {
			attType = "image"
		} else if strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/") {
			attType = "audio"
		}
		data := resolveFile(doc.FileID)
		attachments = append(attachments, models.Attachment{
			Type:     attType,
			MIMEType: mimeType,
			Filename: doc.FileName,
			Size:     int64(doc.FileSize),
			Data:     data,
		})
	}

	return text, attachments
}

// isMentioned returns true when the Telegram update targets the bot:
// either the message is a reply to one of the bot's own messages, or the
// message text contains an @mention of the bot username.
func (a *Adapter) isMentioned(msg *tgbotapi.Message) bool {
	// Reply to the bot's own message.
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		if msg.ReplyToMessage.From.ID == a.botUserID {
			return true
		}
	}
	// Explicit @mention anywhere in the text or caption.
	if a.botUsername != "" {
		text := strings.ToLower(msg.Text + " " + msg.Caption)
		if strings.Contains(text, "@"+a.botUsername) {
			return true
		}
	}
	// Telegram entity-based mention check (covers /command@botname style too).
	for _, entity := range msg.Entities {
		if entity.Type == "mention" {
			mention := strings.ToLower(msg.Text[entity.Offset : entity.Offset+entity.Length])
			if mention == "@"+a.botUsername {
				return true
			}
		}
	}
	return false
}

func (a *Adapter) SendTyping(ctx context.Context, channelID string) error {
	chatID := a.chatID
	if channelID != "" {
		if parsed, err := strconv.ParseInt(channelID, 10, 64); err == nil {
			chatID = parsed
		}
	}
	_, err := a.bot.Request(tgbotapi.NewChatAction(chatID, "typing"))
	return err
}

func (a *Adapter) SendMessage(ctx context.Context, msg *models.Message) error {
	chatID := a.chatID
	// Prefer the channelID embedded in the message (always a numeric string for
	// Telegram DMs/groups) so outbound messages reach the correct chat.
	if msg.ChannelID != "" {
		if parsed, err := strconv.ParseInt(msg.ChannelID, 10, 64); err == nil {
			chatID = parsed
		}
	}
	htmlContent := markdownToHTML(msg.Content)
	config := tgbotapi.NewMessage(chatID, htmlContent)
	config.ParseMode = tgbotapi.ModeHTML
	_, err := a.bot.Send(config)
	return err
}

func (a *Adapter) SendMedia(ctx context.Context, media *ports.Media) error {
	chatID, _ := strconv.ParseInt(media.ChatID, 10, 64)

	if media.URL != "" {
		caption := media.Caption
		if caption == "" {
			caption = media.URL
		} else {
			caption = caption + "\n" + media.URL
		}
		htmlCaption := markdownToHTML(caption)
		config := tgbotapi.NewMessage(chatID, htmlCaption)
		config.ParseMode = tgbotapi.ModeHTML
		_, err := a.bot.Send(config)
		return err
	}

	htmlCaption := markdownToHTML(media.Caption)
	config := tgbotapi.NewMessage(chatID, htmlCaption)
	config.ParseMode = tgbotapi.ModeHTML
	_, err := a.bot.Send(config)
	return err
}

func (a *Adapter) HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error) {
	var update tgbotapi.Update
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, err
	}

	if update.Message == nil {
		return nil, nil
	}

	tgMsg := update.Message

	// Build a human-readable display name from the Telegram sender info.
	senderName := ""
	senderID := ""
	if from := tgMsg.From; from != nil {
		senderID = strconv.FormatInt(from.ID, 10)
		if from.UserName != "" {
			senderName = "@" + from.UserName
		} else {
			senderName = strings.TrimSpace(from.FirstName + " " + from.LastName)
		}
	}

	isGroup := tgMsg.Chat.IsGroup() || tgMsg.Chat.IsSuperGroup() || tgMsg.Chat.IsChannel()
	groupName := ""
	if isGroup {
		groupName = tgMsg.Chat.Title
	}

	content, attachments := buildAttachments(tgMsg, a.downloadFile)

	msg := &models.Message{
		ID:          uuid.New(),
		ChannelID:   strconv.FormatInt(tgMsg.Chat.ID, 10),
		SenderName:  senderName,
		SenderID:    senderID,
		IsGroup:     isGroup,
		IsMentioned: isGroup && a.isMentioned(tgMsg),
		GroupName:   groupName,
		Content:     content,
		Attachments: attachments,
		Timestamp:   tgMsg.Time(),
	}

	return msg, nil
}

func (a *Adapter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	return &ports.UserInfo{
		ID:          userID,
		Username:    userID,
		DisplayName: userID,
	}, nil
}

func (a *Adapter) React(ctx context.Context, messageID string, emoji string) error {
	return nil
}

func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{
		HasVoiceMessage: true,
		HasCallStream:   false,
		HasTextStream:   true,
		HasMediaSupport: true,
	}
}

// Start connects to Telegram using long-polling and calls onMessage for every
// incoming message. The polling loop runs in a goroutine and stops when ctx is
// cancelled.
func (a *Adapter) Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := a.bot.GetUpdatesChan(u)
	go func() {
		for {
			select {
			case <-ctx.Done():
				a.bot.StopReceivingUpdates()
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				if update.Message == nil {
					continue
				}
				// Ignore messages from the bot itself (prevents echo/double response).
				if from := update.Message.From; from != nil && from.ID == a.botUserID {
					continue
				}
				senderName := ""
				senderID := ""
				if from := update.Message.From; from != nil {
					senderID = strconv.FormatInt(from.ID, 10)
					if from.UserName != "" {
						senderName = "@" + from.UserName
					} else {
						senderName = strings.TrimSpace(from.FirstName + " " + from.LastName)
					}
				}
				isGroup := update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup() || update.Message.Chat.IsChannel()
				isMentioned := isGroup && a.isMentioned(update.Message)
				groupName := ""
				if isGroup {
					groupName = update.Message.Chat.Title
				}
				content, attachments := buildAttachments(update.Message, a.downloadFile)
				msg := &models.Message{
					ID:          uuid.New(),
					ChannelID:   strconv.FormatInt(update.Message.Chat.ID, 10),
					SenderName:  senderName,
					SenderID:    senderID,
					IsGroup:     isGroup,
					IsMentioned: isMentioned,
					GroupName:   groupName,
					Content:     content,
					Attachments: attachments,
					Timestamp:   update.Message.Time(),
				}
				// Update the tracked chat ID for SendMessage.
				a.chatID = update.Message.Chat.ID
				onMessage(ctx, msg)
			}
		}
	}()
	return nil
}

func (a *Adapter) ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, "ogg", nil
}
