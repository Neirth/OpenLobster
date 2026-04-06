package resolvers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/spf13/viper"
)

// pluginsFromRegistry converts domain PluginPort slice to GraphQL Plugin slice.
func pluginsFromRegistry(plugins []ports.PluginPort) []*generated.Plugin {
	result := make([]*generated.Plugin, 0, len(plugins))
	for _, p := range plugins {
		schemaBytes, err := p.Schema()
		schemaStr := "{}"
		if err == nil && len(schemaBytes) > 0 {
			schemaStr = string(schemaBytes)
		}
		result = append(result, &generated.Plugin{
			ID:          p.ID(),
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			PluginType:  p.Type(),
			SchemaJSON:  schemaStr,
			ConfigJSON:  pluginConfigJSON(p.ID()),
			Enabled:     pluginEnabled(p.ID()),
			Available:   pluginEnabled(p.ID()) && pluginAvailable(p),
			LastError:   pluginLastError(p),
			Builtin:     pluginBuiltin(p),
		})
	}
	return result
}

func pluginAvailable(p ports.PluginPort) bool {
	if state, ok := p.(ports.PluginStatePort); ok {
		return state.Available()
	}
	return true
}

func pluginLastError(p ports.PluginPort) *string {
	if state, ok := p.(ports.PluginStatePort); ok {
		err := state.LastError()
		if err == "" {
			return nil
		}
		return &err
	}
	return nil
}

func pluginBuiltin(p ports.PluginPort) bool {
	if state, ok := p.(ports.PluginStatePort); ok {
		return state.Builtin()
	}
	return false
}

func pluginEnabled(pluginID string) bool {
	if strings.HasPrefix(strings.TrimSpace(pluginID), "openlobster-messages-") {
		return true
	}
	key := fmt.Sprintf("plugins.enabled.%s", pluginID)
	if !viper.IsSet(key) {
		return true
	}
	return viper.GetBool(key)
}

func pluginConfigJSON(pluginID string) string {
	cfg := viper.GetStringMap(fmt.Sprintf("plugins.settings.%s", pluginID))
	if len(cfg) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func messagingChannelInputKeyForPluginID(pluginID string) string {
	switch pluginID {
	case "openlobster-messages-telegram":
		return "channelTelegramEnabled"
	case "openlobster-messages-discord":
		return "channelDiscordEnabled"
	case "openlobster-messages-slack":
		return "channelSlackEnabled"
	case "openlobster-messages-whatsapp":
		return "channelWhatsAppEnabled"
	case "openlobster-messages-twilio":
		return "channelTwilioEnabled"
	default:
		return ""
	}
}
