// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/neirth/openlobster/utils/validation_layer/src/config"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

func runMessagingSmoke(client protocol.PluginClient, report *types.PluginReport, opts types.ValidateOptions, file string) {
	cfg := cloneMap(opts.SmokeConfig)
	config.EnsureConfigValue(cfg, "default_chat_id", "123456789")
	config.EnsureConfigValue(cfg, "default_channel_id", "123456789012345678")
	config.EnsureConfigValue(cfg, "channel", "C1234567890")
	config.EnsureConfigValue(cfg, "default_to_number", "+15550000001")

	_ = configurePlugin(client, cfg)

	// Use handshaken info for inbound_mode and capabilities instead of extra calls
	info := client.Info()
	if len(info.Properties) > 0 {
		var props struct {
			InboundMode  string `json:"inbound_mode"`
			Capabilities any    `json:"capabilities"`
		}
		if err := json.Unmarshal(info.Properties, &props); err == nil {
			if props.InboundMode != "" {
				if !isInboundModeValid(props.InboundMode) {
					addSmokeFailure(report, "messaging.inbound_mode", fmt.Sprintf("invalid mode: %q", props.InboundMode), file)
				}
			}
			if props.Capabilities == nil {
				// We don't fail here if it's missing, but it's good to have
			}
		}
	}

	if client.HasFunction("resolve_channel_id") {
		channelID := config.FallbackString(config.ConfigString(cfg, "channel_id"), config.ConfigString(cfg, "default_channel_id"))
		channelID = config.FallbackString(channelID, config.ConfigString(cfg, "default_chat_id"))
		channelID = config.FallbackString(channelID, config.ConfigString(cfg, "channel"))
		channelID = config.FallbackString(channelID, "smoke-channel")
		payload := map[string]any{
			"config": cfg,
			"message": map[string]any{
				"channel_id": channelID,
				"metadata":   map[string]any{},
			},
		}
		resolved, err := client.CallString("resolve_channel_id", payload)
		if err != nil || strings.TrimSpace(resolved) == "" {
			addSmokeFailure(report, "messaging.resolve_channel_id", "could not resolve channel", file)
		}
	}

	runTypingSmoke(client, report, file, cfg, "smoke-channel")

	// Only run voice smokes if capabilities claim support
	if info.Properties != nil {
		var caps struct {
			HasVoiceMessage bool `json:"HasVoiceMessage"`
		}
		if err := json.Unmarshal(info.Properties, &caps); err == nil && caps.HasVoiceMessage {
			runSpeakingSmoke(client, report, file, cfg, "smoke-channel")
			runSendVoiceSmoke(client, report, file, cfg, "smoke-channel")
		}
	}

	// Pre-start: if exported, must call it.
	if client.HasFunction("start") {
		_, err := client.CallJSON("start", map[string]any{"config": cfg})
		if err != nil {
			addSmokeFailure(report, "messaging.start", err.Error(), file)
		} else {
			// Briefly wait for async startup logs
			time.Sleep(2 * time.Second)
		}
	}

	testRecipient := opts.SmokeTestRecipient
	if testRecipient == "" {
		testRecipient = config.FallbackString(config.ConfigString(cfg, "smoke_test_recipient"), config.ConfigString(cfg, "test_destination"))
	}

	if testRecipient != "" && client.HasFunction("send") {
		vCode := "LOBSTER-FINAL"
		if opts.ExpectedInboundContent != "" {
			vCode = opts.ExpectedInboundContent
		}

		fmt.Printf("Sending test message to %s (Verification Code: %s)...\n", testRecipient, vCode)
		client.SetVictoryCode(vCode)

		_, err := client.CallJSON("send", map[string]any{
			"config": cfg,
			"message": map[string]any{
				"channel_id":   testRecipient,
				"recipient_id": testRecipient,
				"content":      fmt.Sprintf("OpenLobster Smoke Test: Messaging Vitals Check. 🤖\n\nPara validar esta prueba, responde exactamente con la palabra:\n\n**%s**\n\nTienes 300 segundos (5 minutos).", vCode),
			},
		})
		if err != nil {
			addSmokeFailure(report, "messaging.send", err.Error(), file)
		} else {
			fmt.Printf("Message sent! Waiting up to 300s (5m) for inbound verification code '%s'... \n", vCode)
			if client.WaitForVictory(300 * time.Second) {
				fmt.Printf("Inbound verification successful! ✅\n")
			} else {
				addSmokeFailure(report, "messaging.inbound", fmt.Sprintf("verification failed: '%s' not received within 300s", vCode), file)
			}
		}
	}
}

func runTypingSmoke(client protocol.PluginClient, report *types.PluginReport, file string, cfg map[string]any, channelID string) {
	if !client.HasFunction("typing") {
		addSmokeFailure(report, "messaging.typing", "missing mandatory 'typing' export", file)
		return
	}
	payload := map[string]any{
		"config": cfg,
		"message": map[string]any{
			"channel_id": channelID,
		},
		"duration_ms": 1000,
	}
	_, err := client.CallJSON("typing", payload)
	if err != nil && isTransportFailure(err.Error()) {
		addSmokeFailure(report, "messaging.typing", err.Error(), file)
	}
}

func runSpeakingSmoke(client protocol.PluginClient, report *types.PluginReport, file string, cfg map[string]any, channelID string) {
	if !client.HasFunction("speaking") {
		addSmokeFailure(report, "messaging.speaking", "missing mandatory 'speaking' export for voice-enabled plugin", file)
		return
	}
	payload := map[string]any{
		"config": cfg,
		"message": map[string]any{
			"channel_id": channelID,
		},
		"duration_ms": 1000,
	}
	_, err := client.CallJSON("speaking", payload)
	if err != nil && isTransportFailure(err.Error()) {
		addSmokeFailure(report, "messaging.speaking", err.Error(), file)
	}
}

func runSendVoiceSmoke(client protocol.PluginClient, report *types.PluginReport, file string, cfg map[string]any, channelID string) {
	if !client.HasFunction("send_voice") {
		addSmokeFailure(report, "messaging.send_voice", "missing mandatory 'send_voice' export for voice-enabled plugin", file)
		return
	}
	// Minimal voice payload
	payload := map[string]any{
		"config": cfg,
		"message": map[string]any{
			"channel_id": channelID,
			"audio": map[string]any{
				"data":   "AAA=", // invalid base64 is fine for ABI probe
				"format": "pcm",
			},
		},
	}
	_, err := client.CallJSON("send_voice", payload)
	if err != nil && isTransportFailure(err.Error()) {
		addSmokeFailure(report, "messaging.send_voice", err.Error(), file)
	}
}

func isInboundModeValid(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "polling", "gateway", "webhook", "disabled":
		return true
	default:
		return false
	}
}

func isTransportFailure(errMsg string) bool {
	msg := strings.ToLower(errMsg)
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "eof")
}
