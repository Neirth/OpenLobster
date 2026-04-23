package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// SecretsWrapper wraps a "secrets"-type PluginPort and implements
// ports.SecretsProvider.
type SecretsWrapper struct {
	plugin ports.PluginPort
	cfg    map[string]interface{}
}

func NewSecretsWrapper(p ports.PluginPort, cfg map[string]interface{}) *SecretsWrapper {
	return &SecretsWrapper{plugin: p, cfg: cfg}
}

func (w *SecretsWrapper) UpdateConfig(cfg map[string]interface{}) {
	w.cfg = cfg
}

func (w *SecretsWrapper) Plugin() ports.PluginPort {
	return w.plugin
}

func (w *SecretsWrapper) call(fn string, payload map[string]interface{}) ([]byte, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["config"] = w.cfg
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("secrets plugin %s: marshal %s: %w", w.plugin.ID(), fn, err)
	}
	return w.plugin.Call(fn, raw)
}

func (w *SecretsWrapper) Get(ctx context.Context, key string) (string, error) {
	out, err := w.call("get", map[string]interface{}{"key": key})
	if err != nil {
		return "", err
	}
	var resp struct {
		Value string `json:"value"`
		Found *bool  `json:"found,omitempty"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("secrets plugin %s: unmarshal get: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return "", fmt.Errorf("secrets plugin %s: %s", w.plugin.ID(), resp.Error)
	}
	if resp.Found != nil && !*resp.Found {
		return "", ports.ErrNotFound
	}
	return resp.Value, nil
}

func (w *SecretsWrapper) Set(ctx context.Context, key string, value string) error {
	out, err := w.call("set", map[string]interface{}{"key": key, "value": value})
	if err != nil {
		return err
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("secrets plugin %s: unmarshal set: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return fmt.Errorf("secrets plugin %s: %s", w.plugin.ID(), resp.Error)
	}
	return nil
}

func (w *SecretsWrapper) Delete(ctx context.Context, key string) error {
	out, err := w.call("delete", map[string]interface{}{"key": key})
	if err != nil {
		return err
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("secrets plugin %s: unmarshal delete: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return fmt.Errorf("secrets plugin %s: %s", w.plugin.ID(), resp.Error)
	}
	return nil
}

func (w *SecretsWrapper) List(ctx context.Context, prefix string) ([]string, error) {
	out, err := w.call("list", map[string]interface{}{"prefix": prefix})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Keys  []string `json:"keys"`
		Error string   `json:"error,omitempty"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("secrets plugin %s: unmarshal list: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("secrets plugin %s: %s", w.plugin.ID(), resp.Error)
	}
	return resp.Keys, nil
}
