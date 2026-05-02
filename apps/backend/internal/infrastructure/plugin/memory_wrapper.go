package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/neirth/openlobster/internal/domain/ports"
)

type MemoryWrapper struct {
	plugin ports.PluginPort
	cfg    map[string]interface{}
}

func NewMemoryWrapper(p ports.PluginPort, cfg map[string]interface{}) *MemoryWrapper {
	return &MemoryWrapper{plugin: p, cfg: cfg}
}

func (w *MemoryWrapper) UpdateConfig(cfg map[string]interface{}) {
	w.cfg = cfg
}

func (w *MemoryWrapper) Plugin() ports.PluginPort {
	return w.plugin
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
	_, err := w.call("delete", map[string]interface{}{
		"config":     w.cfg,
		"target_type": "relation",
		"from":        from,
		"to":          to,
		"user_id":     from,
	})
	if err != nil {
		return err
	}
	
	// Verify deletion worked by checking if relation still exists
	raw, _ := w.call("query", map[string]interface{}{
		"config":  w.cfg,
		"op":      "user_graph",
		"user_id": from,
	})
	type DeletionCheckResult struct {
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"edges"`
	}
	var gr DeletionCheckResult
	if json.Unmarshal(raw, &gr) == nil {
		for _, e := range gr.Edges {
			effectiveTo := strings.TrimPrefix(e.Target, "user:")
			if (from == e.Source && effectiveTo == to) || (e.Source == from+"-" && strings.Contains(e.Target, to)) {
				return fmt.Errorf("relation still exists after delete: %s -> %s", from, to)
			}
		}
	}
	
	return nil
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
	_, err := w.call("delete", map[string]interface{}{
		"config":      w.cfg,
		"target_id":   nodeID,
		"target_type": "node",
		"user_id":     userID,
	})
	if err != nil {
		return err
	}

	// POST-DELETE VERIFY: Check node was actually deleted
	checkRaw, _ := w.call("query", map[string]interface{}{
		"config":  w.cfg,
		"op":      "user_graph",
		"user_id": userID,
	})
	type NodeCheckResult struct {
		Nodes []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"nodes"`
	}
	var gr NodeCheckResult
	if json.Unmarshal(checkRaw, &gr) == nil {
		for _, n := range gr.Nodes {
			// Flexible matching: check ID prefix variants and Label
			nodesToCheck := []string{
				n.ID,
				strings.TrimPrefix(n.ID, "user:"),
				n.Label,
				strings.TrimPrefix(n.Label, "user:"),
			}
			for _, checkID := range nodesToCheck {
				if checkID == nodeID {
					return fmt.Errorf("node still exists after delete: %s", nodeID)
				}
			}
		}
	}

	return nil
}

// DeleteNode implements NodeMutatorPort interface (wraps DeleteMemoryNode)
func (w *MemoryWrapper) DeleteNode(ctx context.Context, id string) error {
	return w.DeleteMemoryNode(ctx, "", id)
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
