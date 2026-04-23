package resolvers

import (
	"encoding/json"
	"strings"

	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/plugin"
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
			ID:          p.Type() + ":" + p.ID(),
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			PluginType:  p.Type(),
			SchemaJSON:  schemaStr,
			ConfigJSON:  pluginConfigJSON(p.Type(), p.ID()),
			Enabled:     pluginEnabled(p),
			Available:   pluginAvailable(p),
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

func pluginEnabled(p ports.PluginPort) bool {
	pluginID := p.ID()
	if strings.HasPrefix(strings.TrimSpace(pluginID), "openlobster-messages-") {
		return true
	}
	root := plugin.GetViperRoot(p.Type(), pluginID)
	key := root + ".enabled"

	// Special cases for core backward compatibility
	if !viper.IsSet(key) {
		return true
	}
	return viper.GetBool(key)
}

func pluginConfigJSON(pluginType, pluginID string) string {
	root := plugin.GetViperRoot(pluginType, pluginID)
	cfg := viper.GetStringMap(root)
	if len(cfg) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
