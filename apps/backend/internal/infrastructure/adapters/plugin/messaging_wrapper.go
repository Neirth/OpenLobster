package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// MessagingWrapper wraps a "messaging"-type PluginPort and implements ports.MessagingPort.
// The plugin must export openlobster_send() and openlobster_capabilities().
// openlobster_start() is optional — call StartLoop separately for poll-based plugins.
type MessagingWrapper struct {
	plugin      ports.PluginPort
	cfg         map[string]interface{}
	channelType string // e.g. "telegram", "discord"
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

func (w *MessagingWrapper) SendMessage(ctx context.Context, msg *models.Message) error {
	raw, err := json.Marshal(sendPluginInput{Config: w.cfg, Message: msg})
	if err != nil {
		return fmt.Errorf("messaging plugin %s: marshal: %w", w.plugin.ID(), err)
	}
	_, err = w.plugin.Call("send", raw)
	return err
}

func (w *MessagingWrapper) SendMedia(_ context.Context, _ *ports.Media) error {
	return nil // optional — plugins may not support media
}

func (w *MessagingWrapper) SendTyping(_ context.Context, _ string) error {
	return nil
}

func (w *MessagingWrapper) HandleWebhook(_ context.Context, _ []byte) (*models.Message, error) {
	return nil, nil
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
	cfgJSON, err := json.Marshal(w.cfg)
	if err != nil {
		return fmt.Errorf("messaging plugin %s: marshal config: %w", w.plugin.ID(), err)
	}
	// Run in a goroutine — the plugin's start loop is blocking.
	go func() {
		if _, err := w.plugin.Call("start", cfgJSON); err != nil {
			fmt.Printf("messaging plugin %s: start loop exited: %v\n", w.plugin.ID(), err)
		}
	}()
	return nil
}
