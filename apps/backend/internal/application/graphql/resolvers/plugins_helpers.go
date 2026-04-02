package resolvers

import (
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/domain/ports"
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
			Enabled:     true,
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

