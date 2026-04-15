// Package ports defines the plugin port interface used by the plugin registry
// and all plugin wrapper adapters.
package ports

// PluginPort is the interface every loaded plugin must satisfy.
// The host communicates with plugins by calling exported function names
// through the active plugin runtime transport.
type PluginPort interface {
	// ID returns a unique identifier for this plugin (usually its filename stem).
	ID() string
	// Name returns the human-readable plugin name.
	Name() string
	// Version returns the plugin version string.
	Version() string
	// Description returns the plugin description.
	Description() string
	// Type returns the plugin type: "ai", "messaging", "memory", or "tool".
	Type() string
	// Schema returns the JSON Schema (as raw JSON bytes) describing the plugin's
	// required configuration properties. Used for YAML validation and frontend forms.
	Schema() ([]byte, error)
	// Properties returns technical metadata exported by the plugin during handshake.
	Properties() []byte
	// Call invokes an exported plugin function by name, passing input as JSON
	// bytes and returning the result as JSON bytes.
	Call(function string, input []byte) ([]byte, error)
	// Close releases all runtime resources held by this plugin.
	Close() error
}

// PluginStatePort is an optional extension implemented by runtime plugin
// adapters to expose lifecycle/runtime state to GraphQL and diagnostics.
type PluginStatePort interface {
	Available() bool
	LastError() string
	Builtin() bool
}

// PluginStateSetterPort is an optional extension used by loaders/managers to
// mark plugins with static metadata such as builtin catalog membership.
type PluginStateSetterPort interface {
	SetBuiltin(v bool)
}

// PluginFunctionIntrospectionPort is an optional extension implemented by
// runtime adapters that can report whether a plugin exports a given function.
// It is used by ABI validators to enforce function-level contracts without
// invoking potentially long-running exports.
type PluginFunctionIntrospectionPort interface {
	HasFunction(function string) bool
}

// PluginRegistryPort is the read interface used by GraphQL resolvers.
type PluginRegistryPort interface {
	All() []PluginPort
	Get(id string) PluginPort
	GetByType(pluginType string) []PluginPort
}
