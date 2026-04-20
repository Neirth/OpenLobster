package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/plugin/runtime/threads"
)

// MessagingWrapper wraps a "messaging"-type PluginPort and implements ports.MessagingPort.
// The plugin must export openlobster_resolve_channel_id(), openlobster_send(),
// and openlobster_capabilities().
// openlobster_start() is optional — call StartLoop separately for poll-based plugins.
type MessagingWrapper struct {
	plugin      ports.PluginPort
	cfg         map[string]interface{}
	channelType string // e.g. "telegram", "discord"

	inboundModeOnce sync.Once
	inboundMode     string

	loopMu              sync.Mutex
	loopGeneration      uint64
	loopCancel          context.CancelFunc
	loopRunner          ports.PluginPort
	loopRunnerDedicated bool
}

type messagingLoopRunnerFactory interface {
	CreateLoopRunner() (ports.PluginPort, error)
}

const resolveChannelIDFn = "resolve_channel_id"
const typingFn = "typing"
const handleWebhookFn = "handle_webhook"
const convertAudioForPlatformFn = "convert_audio_for_platform"
const webhookNotSupportedPrefix = "webhook_not_supported"
const inboundModeFn = "inbound_mode"
const minHealthyStartLoopRuntime = 2 * time.Second

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

// Close stops the active start loop and releases any dedicated loop runner.
func (w *MessagingWrapper) Close() error {
	w.loopMu.Lock()
	w.loopGeneration++
	cancel := w.loopCancel
	runner := w.loopRunner
	dedicated := w.loopRunnerDedicated
	w.loopCancel = nil
	w.loopRunner = nil
	w.loopRunnerDedicated = false
	w.loopMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if dedicated && runner != nil {
		return runner.Close()
	}
	return nil
}

type sendPluginInput struct {
	Config  map[string]interface{} `json:"config"`
	Message *models.Message        `json:"message"`
}

type typingPluginInput struct {
	Config     map[string]interface{} `json:"config"`
	Message    *models.Message        `json:"message"`
	DurationMS int                    `json:"duration_ms"`
}

type convertAudioPluginInput struct {
	Config map[string]interface{} `json:"config"`
	Audio  string                 `json:"audio"`
	Format string                 `json:"format"`
}

type convertAudioPluginOutput struct {
	Audio  string `json:"audio,omitempty"`
	Format string `json:"format,omitempty"`
	Error  string `json:"error,omitempty"`
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

func (w *MessagingWrapper) SendTyping(ctx context.Context, channelID string, duration_ms int) error {
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

	input := typingPluginInput{
		Config:     w.currentConfig(),
		Message:    &outbound,
		DurationMS: duration_ms,
	}

	raw, err := json.Marshal(input)
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

func parseInboundModeOutput(out []byte) (string, error) {
	mode := strings.TrimSpace(strings.ToLower(string(out)))
	switch mode {
	case ports.InboundModePolling, ports.InboundModeGateway, ports.InboundModeWebhook, ports.InboundModeDisabled:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid %s value %q", inboundModeFn, strings.TrimSpace(string(out)))
	}
}

// InboundMode returns the plugin declared inbound runtime mode.
func (w *MessagingWrapper) InboundMode() string {
	w.inboundModeOnce.Do(func() {
		w.inboundMode = ports.InboundModePolling

		raw := w.plugin.Properties()
		if len(raw) == 0 {
			return
		}

		var props struct {
			InboundMode string `json:"inbound_mode"`
		}
		if err := json.Unmarshal(raw, &props); err != nil {
			return
		}

		mode, parseErr := parseInboundModeOutput([]byte(props.InboundMode))
		if parseErr != nil {
			return
		}
		w.inboundMode = mode
	})

	if w.inboundMode == "" {
		return ports.InboundModePolling
	}
	return w.inboundMode
}

// RequiresBackgroundLoop reports whether this channel mode needs start-loop runtime.
func (w *MessagingWrapper) RequiresBackgroundLoop() bool {
	mode := w.InboundMode()
	return mode == ports.InboundModePolling || mode == ports.InboundModeGateway
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
	raw := w.plugin.Properties()
	if len(raw) == 0 {
		return ports.GetCapabilitiesForType(w.channelType)
	}

	var props struct {
		Capabilities ports.ChannelCapabilities `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &props); err != nil {
		return ports.GetCapabilitiesForType(w.channelType)
	}
	return props.Capabilities
}

func (w *MessagingWrapper) ConvertAudioForPlatform(_ context.Context, audioData []byte, format string) ([]byte, string, error) {
	if len(audioData) == 0 {
		return audioData, format, nil
	}

	input := convertAudioPluginInput{
		Config: w.currentConfig(),
		Audio:  base64.StdEncoding.EncodeToString(audioData),
		Format: format,
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, "", fmt.Errorf("messaging plugin %s: marshal %s input: %w", w.plugin.ID(), convertAudioForPlatformFn, err)
	}

	out, err := w.plugin.Call(convertAudioForPlatformFn, raw)
	if err != nil {
		if isMissingPluginFunction(err, convertAudioForPlatformFn) {
			return audioData, format, nil
		}
		return nil, "", fmt.Errorf("messaging plugin %s: %s: %w", w.plugin.ID(), convertAudioForPlatformFn, err)
	}
	if len(out) == 0 {
		return audioData, format, nil
	}

	var resp convertAudioPluginOutput
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, "", fmt.Errorf("messaging plugin %s: parse %s output: %w", w.plugin.ID(), convertAudioForPlatformFn, err)
	}
	if resp.Error != "" {
		return nil, "", fmt.Errorf("messaging plugin %s: %s failed: %s", w.plugin.ID(), convertAudioForPlatformFn, resp.Error)
	}

	converted := audioData
	if strings.TrimSpace(resp.Audio) != "" {
		decoded, err := base64.StdEncoding.DecodeString(resp.Audio)
		if err != nil {
			return nil, "", fmt.Errorf("messaging plugin %s: decode %s audio: %w", w.plugin.ID(), convertAudioForPlatformFn, err)
		}
		converted = decoded
	}

	resultFormat := format
	if strings.TrimSpace(resp.Format) != "" {
		resultFormat = strings.TrimSpace(resp.Format)
	}

	return converted, resultFormat, nil
}

// Start launches the plugin's event loop in the background. onMessage is called
// for every inbound message the plugin delivers via host_emit_message.
// Plugins that are webhook-driven may export a no-op openlobster_start.
func (w *MessagingWrapper) Start(ctx context.Context, _ func(context.Context, *models.Message)) error {
	if ctx == nil {
		ctx = context.Background()
	}

	inboundMode := w.InboundMode()
	if !w.RequiresBackgroundLoop() {
		log.Printf("messaging plugin %s: inbound_mode=%s, skipping background start loop", w.plugin.ID(), inboundMode)
		return nil
	}

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

	loopCtx, loopCancel := context.WithCancel(ctx)

	w.loopMu.Lock()
	w.loopGeneration++
	generation := w.loopGeneration
	prevCancel := w.loopCancel
	prevRunner := w.loopRunner
	prevDedicated := w.loopRunnerDedicated
	w.loopCancel = loopCancel
	w.loopRunner = runner
	w.loopRunnerDedicated = dedicated
	w.loopMu.Unlock()

	if prevCancel != nil {
		prevCancel()
	}
	if prevDedicated && prevRunner != nil {
		_ = prevRunner.Close()
	}

	// Run in a goroutine — the plugin's start loop is blocking. If the loop
	// exits unexpectedly, try to recover so inbound channel delivery keeps working.
	go func(loopGen uint64) {
		defer func() {
			if dedicated {
				_ = runner.Close()
			}
			w.loopMu.Lock()
			if w.loopGeneration == loopGen {
				w.loopCancel = nil
				w.loopRunner = nil
				w.loopRunnerDedicated = false
			}
			w.loopMu.Unlock()
		}()

		backoff := time.Second
		const maxBackoff = 30 * time.Second
		attempt := 0
		workerRegistry := threads.DefaultWorkerRegistry()
		for {
			if loopCtx.Err() != nil {
				return
			}

			attempt++
			activeRunner := runner
			startedAt := time.Now()
			worker, launchErr := workerRegistry.StartOrReplace(threads.StartConfig{
				Context:     loopCtx,
				PluginID:    w.plugin.ID(),
				ChannelType: w.channelType,
				Attempt:     attempt,
				Work: func(context.Context) error {
					_, callErr := activeRunner.Call("start", cfgJSON)
					return callErr
				},
			})
			var startErrCh <-chan error
			if launchErr != nil {
				startErrCh = immediateErrChannel(launchErr)
			} else {
				startErrCh = worker.Done()
			}

			var startErr error
			select {
			case <-loopCtx.Done():
				if worker != nil {
					worker.Stop()
				}
				if dedicated {
					_ = activeRunner.Close()
				}
				select {
				case <-startErrCh:
				case <-time.After(2 * time.Second):
				}
				return
			case startErr = <-startErrCh:
			}

			if loopCtx.Err() != nil {
				return
			}

			runtime := time.Since(startedAt)
			if startErr != nil {
				log.Printf("messaging plugin %s: start loop exited: %v", w.plugin.ID(), startErr)
			} else if runtime < minHealthyStartLoopRuntime {
				log.Printf(
					"messaging plugin %s: inbound_mode=%s start exited after %s; retrying with backoff",
					w.plugin.ID(), inboundMode, runtime.Round(time.Millisecond),
				)
			} else {
				log.Printf("messaging plugin %s: start loop exited", w.plugin.ID())
				backoff = time.Second
			}

			if dedicated && shouldRecreateLoopRunner(startErr) {
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

				w.loopMu.Lock()
				if w.loopGeneration == loopGen {
					w.loopRunner = runner
					w.loopRunnerDedicated = dedicated
				}
				w.loopMu.Unlock()
			} else if dedicated && startErr != nil {
				log.Printf("messaging plugin %s: reusing dedicated loop runner after start error", w.plugin.ID())
			}

			sleepFor := restartDelayWithJitter(backoff)
			select {
			case <-loopCtx.Done():
				return
			case <-time.After(sleepFor):
			}
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}(generation)
	return nil
}

func immediateErrChannel(err error) <-chan error {
	ch := make(chan error, 1)
	ch <- err
	close(ch)
	return ch
}

func restartDelayWithJitter(backoff time.Duration) time.Duration {
	if backoff <= 0 {
		backoff = time.Second
	}

	base := backoff / 2
	if base < 100*time.Millisecond {
		base = 100 * time.Millisecond
	}

	jitterWindow := backoff - base
	if jitterWindow <= 0 {
		return base
	}

	return base + time.Duration(rand.Int63n(int64(jitterWindow)+1))
}

func shouldRecreateLoopRunner(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection is shut down") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "transport is closing") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "unavailable")
}
