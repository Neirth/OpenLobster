package plugin

import (
	"strings"
)

// GetViperRoot returns the configuration root path in Viper based on plugin type and ID.
// This is the core logic for "Dynamic Legacy Routing".
func GetViperRoot(pluginType, pluginID string) string {
	id := strings.ToLower(strings.TrimSpace(pluginID))
	t := strings.ToLower(strings.TrimSpace(pluginType))

	switch t {
	case "ai":
		return "providers." + id
	case "messaging":
		return "channels." + id
	case "memory":
		return "memory." + id
	case "secrets":
		return "secrets." + id
	case "audio":
		return "audio." + id
	default:
		return id
	}
}
