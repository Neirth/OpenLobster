package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// MessagingWrapper wraps a "messaging"-type PluginPort and implements ports.MessagingPort.
// The plugin must export openlobster_resolve_channel_id(), openlobster_send(),
// and openlobster_capabilities().
// openlobster_start() is optional — call StartLoop separately for poll-based plugins.
type MessagingWrapper struct {
	plugin      ports.PluginPort
	cfg         map[string]interface{}
	channelType string // e.g. "telegram", "discord"
}

type messagingLoopRunnerFactory interface {
	CreateLoopRunner() (ports.PluginPort, error)
}

const resolveChannelIDFn = "resolve_channel_id"
const typingFn = "typing"
const handleWebhookFn = "handle_webhook"
const webhookNotSupportedPrefix = "webhook_not_supported"

type webhookRequestEnvelope struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`
}

type webhookAdapterPayload struct {
	Request webhookRequestEnvelope `json:"request"`
}

type webhookPluginInput struct {
	Config  map[string]interface{} `json:"config"`
	Request webhookRequestEnvelope `json:"request"`
}

func (w *MessagingWrapper) currentConfig() map[string]interface{} {
	return liveConfigForPlugin(w.plugin.ID(), w.cfg)
}

// NewMessagingWrapper returns a MessagingWrapper backed by p.
func NewMessagingWrapper(p ports.PluginPort, channelType string, cfg map[string]interface{}) *MessagingWrapper {
	return &MessagingWrapper{plugin: p, channelType: channelType, cfg: cfg}
}

// ChannelType returns the channel type string this wrapper represents.
func (w *MessagingWrapper) ChannelType() string { return w.channelType }

type sendPluginInput struct {
	Config  map[string]interface{} `json:"config"`
	Message *models.Message        `json:"message"`
}

func (w *MessagingWrapper) resolveChannelID(msg *models.Message) (string, error) {
	raw, err := json.Marshal(sendPluginInput{Config: w.currentConfig(), Message: msg})
	if err != nil {
		return "", fmt.Errorf("messaging plugin %s: marshal resolve_channel_id input: %w", w.plugin.ID(), err)
	}
	out, err := w.plugin.Call(resolveChannelIDFn, raw)
	if err != nil {
		return "", fmt.Errorf("messaging plugin %s: %s: %w", w.plugin.ID(), resolveChannelIDFn, err)
	}
	resolvedChannelID := strings.TrimSpace(string(out))
	if resolvedChannelID == "" {
		return "", fmt.Errorf("messaging plugin %s: %s returned empty channel_id", w.plugin.ID(), resolveChannelIDFn)
	}
	return resolvedChannelID, nil
}

func (w *MessagingWrapper) SendMessage(ctx context.Context, msg *models.Message) error {
	_ = ctx
	if msg == nil {
		return nil
	}

	resolvedChannelID, err := w.resolveChannelID(msg)
	if err != nil {
		return err
	}

	outbound := *msg
	outbound.ChannelID = resolvedChannelID

	raw, err := json.Marshal(sendPluginInput{Config: w.currentConfig(), Message: &outbound})
	if err != nil {
		return fmt.Errorf("messaging plugin %s: marshal: %w", w.plugin.ID(), err)
	}
	_, err = w.plugin.Call("send", raw)
	return err
}

func (w *MessagingWrapper) SendMedia(_ context.Context, _ *ports.Media) error {
	return nil // optional — plugins may not support media
}

func (w *MessagingWrapper) SendTyping(ctx context.Context, channelID string) error {
	msg := models.NewMessage(channelID, "")
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]interface{})
	}
	if v, ok := ctx.Value(ports.ContextKeyChannelType).(string); ok {
		if ct := strings.TrimSpace(strings.ToLower(v)); ct != "" {
			msg.Metadata["channel_type"] = ct
		}
	}

	resolvedChannelID, err := w.resolveChannelID(msg)
	if err != nil {
		return err
	}

	outbound := *msg
	outbound.ChannelID = resolvedChannelID

	raw, err := json.Marshal(sendPluginInput{Config: w.currentConfig(), Message: &outbound})
	if err != nil {
		return fmt.Errorf("messaging plugin %s: marshal typing: %w", w.plugin.ID(), err)
	}
	_, err = w.plugin.Call(typingFn, raw)
	if err != nil && isMissingPluginFunction(err, typingFn) {
		return nil
	}
	return err
}

func isMissingPluginFunction(err error, function string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	needle := fmt.Sprintf("function %q not exported", strings.ToLower(function))
	return strings.Contains(msg, needle)
}

func isWebhookNotSupportedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), webhookNotSupportedPrefix)
}

func (w *MessagingWrapper) HandleWebhook(_ context.Context, payload []byte) (*models.Message, error) {
	var envelope webhookAdapterPayload
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("messaging plugin %s: invalid webhook payload: %w", w.plugin.ID(), err)
	}

	raw, err := json.Marshal(webhookPluginInput{
		Config:  w.currentConfig(),
		Request: envelope.Request,
	})
	if err != nil {
		return nil, fmt.Errorf("messaging plugin %s: marshal handle_webhook input: %w", w.plugin.ID(), err)
	}

	out, err := w.plugin.Call(handleWebhookFn, raw)
	if err != nil {
		if isMissingPluginFunction(err, handleWebhookFn) || isWebhookNotSupportedError(err) {
			return nil, fmt.Errorf("%s: plugin %s does not support HTTP webhooks", webhookNotSupportedPrefix, w.plugin.ID())
		}
		return nil, fmt.Errorf("messaging plugin %s: %s: %w", w.plugin.ID(), handleWebhookFn, err)
	}

	if strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}

	var msg models.Message
	if err := json.Unmarshal(out, &msg); err != nil {
		return nil, fmt.Errorf("messaging plugin %s: parse %s output: %w", w.plugin.ID(), handleWebhookFn, err)
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Metadata == nil {
		msg.Metadata = map[string]interface{}{}
	}
	if _, ok := msg.Metadata["channel_type"]; !ok {
		msg.Metadata["channel_type"] = w.channelType
	}

	return &msg, nil
}

func (w *MessagingWrapper) GetUserInfo(_ context.Context, _ string) (*ports.UserInfo, error) {
	return nil, nil
}

func (w *MessagingWrapper) React(_ context.Context, _ string, _ string) error {
	return nil
}

func (w *MessagingWrapper) GetCapabilities() ports.ChannelCapabilities {
	out, err := w.plugin.Call("capabilities", nil)
	if err != nil || len(out) == 0 {
		return ports.GetCapabilitiesForType(w.channelType)
	}
	var caps ports.ChannelCapabilities
	if err := json.Unmarshal(out, &caps); err != nil {
		return ports.GetCapabilitiesForType(w.channelType)
	}
	return caps
}

func (w *MessagingWrapper) ConvertAudioForPlatform(_ context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, format, nil
}

// Start launches the plugin's event loop in the background. onMessage is called
// for every inbound message the plugin delivers via host_emit_message.
// Plugins that are webhook-driven may export a no-op openlobster_start.
func (w *MessagingWrapper) Start(ctx context.Context, _ func(context.Context, *models.Message)) error {
	cfgJSON, err := json.Marshal(w.currentConfig())
	if err != nil {
		return fmt.Errorf("messaging plugin %s: marshal config: %w", w.plugin.ID(), err)
	}

	createRunner := func() (ports.PluginPort, bool, error) {
		factory, ok := w.plugin.(messagingLoopRunnerFactory)
		if !ok {
			return w.plugin, false, nil
		}
		runner, createErr := factory.CreateLoopRunner()
		if createErr != nil {
			return nil, false, createErr
		}
		return runner, true, nil
	}

	runner, dedicated, createErr := createRunner()
	if createErr != nil {
		log.Printf("messaging plugin %s: dedicated loop runner unavailable, using primary runner: %v", w.plugin.ID(), createErr)
		runner = w.plugin
		dedicated = false
	}

	// Run in a goroutine — the plugin's start loop is blocking. If the loop
	// exits unexpectedly, try to recover so inbound channel delivery keeps working.
	go func() {
		if dedicated {
			defer runner.Close()
		}

		backoff := time.Second
		for {
			if ctx.Err() != nil {
				return
			}

			_, startErr := runner.Call("start", cfgJSON)
			if ctx.Err() != nil {
				return
			}

			if startErr != nil {
				log.Printf("messaging plugin %s: start loop exited: %v", w.plugin.ID(), startErr)
			} else {
				log.Printf("messaging plugin %s: start loop exited", w.plugin.ID())
			}

			if dedicated {
				_ = runner.Close()
				nextRunner, nextDedicated, nextErr := createRunner()
				if nextErr != nil {
					log.Printf("messaging plugin %s: failed to recreate dedicated loop runner, falling back to primary runner: %v", w.plugin.ID(), nextErr)
					runner = w.plugin
					dedicated = false
				} else {
					runner = nextRunner
					dedicated = nextDedicated
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 8*time.Second {
				backoff *= 2
			}
		}
	}()
	return nil
}
