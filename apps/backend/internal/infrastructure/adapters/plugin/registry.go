package plugin

import (
	"log"
	"sync"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// Registry is a thread-safe store of loaded plugins, implementing ports.PluginRegistryPort.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]ports.PluginPort
	order   []string // insertion order for All()
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]ports.PluginPort)}
}

// Register adds a plugin. Replaces any existing plugin with the same ID.
func (r *Registry) Register(p ports.PluginPort) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[p.ID()]; !exists {
		r.order = append(r.order, p.ID())
	}
	r.plugins[p.ID()] = p
}

// All returns all registered plugins in insertion order.
func (r *Registry) All() []ports.PluginPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ports.PluginPort, 0, len(r.order))
	for _, id := range r.order {
		if p, ok := r.plugins[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// Get returns the plugin with the given ID, or nil.
func (r *Registry) Get(id string) ports.PluginPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[id]
}

// GetByType returns all plugins of the given type.
func (r *Registry) GetByType(pluginType string) []ports.PluginPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ports.PluginPort
	for _, id := range r.order {
		if p, ok := r.plugins[id]; ok && p.Type() == pluginType {
			out = append(out, p)
		}
	}
	return out
}

// Close calls Close on every registered plugin and logs errors.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, p := range r.plugins {
		if err := p.Close(); err != nil {
			log.Printf("plugins: close %s: %v", id, err)
		}
	}
}
