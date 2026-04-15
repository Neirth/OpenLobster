// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/neirth/openlobster/utils/validation_layer/src/config"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

func runMemorySmoke(client protocol.PluginClient, report *types.PluginReport, opts types.ValidateOptions, file, tmpDir string) {
	cfg := cloneMap(opts.SmokeConfig)
	config.EnsureConfigValue(cfg, "path", filepath.Join(tmpDir, "memory.gml"))
	config.FillMissingConfigFromEnv(cfg, map[string][]string{
		"uri":      {"OPENLOBSTER_SMOKE_MEMORY_URI", "OPENLOBSTER_SMOKE_NEO4J_URI"},
		"username": {"OPENLOBSTER_SMOKE_MEMORY_USERNAME", "OPENLOBSTER_SMOKE_NEO4J_USERNAME"},
		"password": {"OPENLOBSTER_SMOKE_MEMORY_PASSWORD", "OPENLOBSTER_SMOKE_NEO4J_PASSWORD"},
		"database": {"OPENLOBSTER_SMOKE_MEMORY_DATABASE", "OPENLOBSTER_SMOKE_NEO4J_DATABASE"},
	})
	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, "memory", err.Error(), file)
		return
	}

	const stressWrites = 8
	for i := 0; i < stressWrites; i++ {
		_, err := client.CallJSON("store", map[string]any{
			"config":      cfg,
			"user_id":     "smoke-user",
			"content":     fmt.Sprintf("openlobster smoke memory entry %d", i),
			"label":       fmt.Sprintf("smoke-%d", i),
			"relation":    "HAS_FACT",
			"entity_type": "fact",
		})
		if err != nil {
			addSmokeFailure(report, "memory.store", err.Error(), file)
			return
		}
	}

	raw, err := client.CallJSON("retrieve", map[string]any{
		"config": cfg,
		"query":  "openlobster smoke memory entry",
		"limit":  64,
	})
	if err != nil {
		addSmokeFailure(report, "memory.retrieve", err.Error(), file)
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		addSmokeFailure(report, "memory.retrieve", "invalid JSON", file)
		return
	}
	if len(rows) < 3 {
		addSmokeFailure(report, "memory.retrieve", "insufficient results after stress writes", file)
		return
	}

	if err := assertMemoryCypherReturnsData(client, cfg, "MATCH (a)-[r]->(b) RETURN a, r, b"); err != nil {
		addSmokeFailure(report, "memory.cypher", err.Error(), file)
		return
	}

	if _, err := client.CallJSON("store", map[string]any{
		"config":  cfg,
		"op":      "invalidate_cache",
		"user_id": "smoke-user",
	}); err != nil {
		addSmokeFailure(report, "memory.invalidate_cache", err.Error(), file)
		return
	}

	const relationWrites = 3
	relationTypes := make([]string, 0, relationWrites)
	peerIDs := make([]string, 0, relationWrites)

	for i := 0; i < relationWrites; i++ {
		relationType := fmt.Sprintf("SMOKE_REL_%d_%d", time.Now().UnixNano(), i)
		peerID := fmt.Sprintf("smoke-peer-%d", i)
		relationTypes = append(relationTypes, relationType)
		peerIDs = append(peerIDs, peerID)

		if _, err := client.CallJSON("store", map[string]any{
			"config":   cfg,
			"op":       "add_relation",
			"from":     "smoke-user",
			"to":       peerID,
			"rel_type": relationType,
		}); err != nil {
			addSmokeFailure(report, "memory.add_relation", err.Error(), file)
			return
		}

		if err := assertMemoryRelationVisible(client, cfg, "smoke-user", "smoke-user", peerID, relationType); err != nil {
			addSmokeFailure(report, "memory.user_graph", err.Error(), file)
			return
		}
	}

	if err := deleteMemoryRelation(client, cfg, "smoke-user", peerIDs[0]); err != nil {
		addSmokeFailure(report, "memory.delete_relation", err.Error(), file)
		return
	}

	if err := assertMemoryRelationAbsent(client, cfg, "smoke-user", "smoke-user", peerIDs[0], relationTypes[0]); err != nil {
		addSmokeFailure(report, "memory.delete_relation", err.Error(), file)
	}
}

func assertMemoryRelationVisible(client protocol.PluginClient, cfg map[string]any, userID, fromID, toID, relationType string) error {
	raw, err := client.CallJSON("query", map[string]any{
		"config":  cfg,
		"op":      "user_graph",
		"user_id": userID,
	})
	if err != nil {
		return err
	}

	var graph struct {
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Label  string `json:"label"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return fmt.Errorf("invalid user_graph JSON")
	}

	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.Label) != relationType {
			continue
		}
		if memoryNodeIDMatches(edge.Source, fromID) && memoryNodeIDMatches(edge.Target, toID) {
			return nil
		}
	}
	return fmt.Errorf("relation %s not visible in user_graph", relationType)
}

func assertMemoryRelationAbsent(client protocol.PluginClient, cfg map[string]any, userID, fromID, toID, relationType string) error {
	raw, err := client.CallJSON("query", map[string]any{
		"config":  cfg,
		"op":      "user_graph",
		"user_id": userID,
	})
	if err != nil {
		return err
	}

	var graph struct {
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Label  string `json:"label"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return fmt.Errorf("invalid user_graph JSON")
	}

	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.Label) != relationType {
			continue
		}
		if memoryNodeIDMatches(edge.Source, fromID) && memoryNodeIDMatches(edge.Target, toID) {
			return fmt.Errorf("relation %s still visible after deletion", relationType)
		}
	}
	return nil
}

func assertMemoryCypherReturnsData(client protocol.PluginClient, cfg map[string]any, cypher string) error {
	raw, err := client.CallJSON("query", map[string]any{
		"config": cfg,
		"op":     "cypher",
		"cypher": cypher,
	})
	if err != nil {
		return err
	}

	var result struct {
		Data   []map[string]any `json:"data"`
		Errors []any            `json:"errors"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("invalid cypher JSON")
	}

	for _, queryErr := range result.Errors {
		msg := strings.TrimSpace(fmt.Sprintf("%v", queryErr))
		if msg != "" && msg != "<nil>" {
			return fmt.Errorf("cypher returned errors: %s", msg)
		}
	}

	if len(result.Data) == 0 {
		return fmt.Errorf("cypher returned empty data")
	}
	return nil
}

func deleteMemoryRelation(client protocol.PluginClient, cfg map[string]any, fromID, toID string) error {
	prefixedFrom := fromID
	if !strings.HasPrefix(prefixedFrom, "user:") {
		prefixedFrom = "user:" + prefixedFrom
	}
	prefixedTo := toID
	if !strings.HasPrefix(prefixedTo, "user:") {
		prefixedTo = "user:" + prefixedTo
	}

	attempts := []struct{ from, to string }{
		{from: fromID, to: toID},
		{from: prefixedFrom, to: prefixedTo},
	}

	var lastErr error
	for _, attempt := range attempts {
		_, err := client.CallJSON("store", map[string]any{
			"config": cfg,
			"op":     "delete_relation",
			"from":   attempt.from,
			"to":     attempt.to,
		})
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		return fmt.Errorf("delete_relation failed")
	}
	return lastErr
}

func memoryNodeIDMatches(actual, expected string) bool {
	actualID := strings.TrimSpace(actual)
	expectedID := strings.TrimSpace(expected)
	if actualID == expectedID {
		return true
	}
	if strings.HasPrefix(actualID, "user:") && strings.TrimPrefix(actualID, "user:") == expectedID {
		return true
	}
	return false
}
