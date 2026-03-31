package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// MemoryWrapper wraps a "memory"-type PluginPort and implements ports.MemoryPort.
// The plugin must export openlobster_store(), openlobster_retrieve(), openlobster_query().
type MemoryWrapper struct {
	plugin ports.PluginPort
	cfg    map[string]interface{}
}

// NewMemoryWrapper returns a MemoryWrapper backed by p.
func NewMemoryWrapper(p ports.PluginPort, cfg map[string]interface{}) *MemoryWrapper {
	return &MemoryWrapper{plugin: p, cfg: cfg}
}

func (w *MemoryWrapper) call(fn string, payload interface{}) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("memory plugin %s: marshal: %w", w.plugin.ID(), err)
	}
	return w.plugin.Call(fn, raw)
}

func (w *MemoryWrapper) AddKnowledge(ctx context.Context, userID, content, label, relation, entityType string, embedding []float64) error {
	_, err := w.call("store", map[string]interface{}{
		"config":      w.cfg,
		"user_id":     userID,
		"content":     content,
		"label":       label,
		"relation":    relation,
		"entity_type": entityType,
		"embedding":   embedding,
	})
	return err
}

func (w *MemoryWrapper) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	out, err := w.call("retrieve", map[string]interface{}{
		"config": w.cfg,
		"query":  query,
		"limit":  limit,
	})
	if err != nil {
		return nil, err
	}
	var results []ports.Knowledge
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("memory plugin %s: unmarshal SearchSimilar: %w", w.plugin.ID(), err)
	}
	return results, nil
}

func (w *MemoryWrapper) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	out, err := w.call("query", map[string]interface{}{
		"config":  w.cfg,
		"user_id": userID,
		"op":      "user_graph",
	})
	if err != nil {
		return ports.Graph{}, err
	}
	var g ports.Graph
	if err := json.Unmarshal(out, &g); err != nil {
		return ports.Graph{}, fmt.Errorf("memory plugin %s: unmarshal GetUserGraph: %w", w.plugin.ID(), err)
	}
	return g, nil
}

func (w *MemoryWrapper) AddRelation(ctx context.Context, from, to, relType string) error {
	_, err := w.call("store", map[string]interface{}{
		"config":   w.cfg,
		"op":       "add_relation",
		"from":     from,
		"to":       to,
		"rel_type": relType,
	})
	return err
}

func (w *MemoryWrapper) DeleteRelation(ctx context.Context, from, to string) error {
	_, err := w.call("store", map[string]interface{}{
		"config": w.cfg,
		"op":     "delete_relation",
		"from":   from,
		"to":     to,
	})
	return err
}

func (w *MemoryWrapper) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	out, err := w.call("query", map[string]interface{}{
		"config": w.cfg,
		"op":     "cypher",
		"cypher": cypher,
	})
	if err != nil {
		return ports.GraphResult{}, err
	}
	var gr ports.GraphResult
	_ = json.Unmarshal(out, &gr)
	return gr, nil
}

func (w *MemoryWrapper) InvalidateMemoryCache(ctx context.Context, userID string) error {
	_, err := w.call("store", map[string]interface{}{
		"config":  w.cfg,
		"op":      "invalidate_cache",
		"user_id": userID,
	})
	return err
}

func (w *MemoryWrapper) SetUserProperty(ctx context.Context, userID, key, value string) error {
	_, err := w.call("store", map[string]interface{}{
		"config":  w.cfg,
		"op":      "set_user_property",
		"user_id": userID,
		"key":     key,
		"value":   value,
	})
	return err
}

func (w *MemoryWrapper) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	_, err := w.call("store", map[string]interface{}{
		"config":    w.cfg,
		"op":        "edit_node",
		"user_id":   userID,
		"node_id":   nodeID,
		"new_value": newValue,
	})
	return err
}

func (w *MemoryWrapper) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	_, err := w.call("store", map[string]interface{}{
		"config":  w.cfg,
		"op":      "delete_node",
		"user_id": userID,
		"node_id": nodeID,
	})
	return err
}

func (w *MemoryWrapper) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	_, err := w.call("store", map[string]interface{}{
		"config":       w.cfg,
		"op":           "update_user_label",
		"user_id":      userID,
		"display_name": displayName,
	})
	return err
}
