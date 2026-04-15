// Copyright (c) OpenLobster contributors. See LICENSE for details.

package manifest_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/neirth/openlobster/utils/validation_layer/src/manifest"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stub client for manifest tests
// ---------------------------------------------------------------------------

type stubClient struct {
	info     protocol.PluginInfo
	metadata map[string]any
	callErr  error
}

func (s *stubClient) Info() protocol.PluginInfo { return s.info }
func (s *stubClient) HasFunction(fn string) bool { return true }
func (s *stubClient) Close() error               { return nil }
func (s *stubClient) SetVictoryCode(code string) {}
func (s *stubClient) WaitForVictory(timeout time.Duration) bool { return true }

func (s *stubClient) CallJSON(fn string, _ any) (json.RawMessage, error) {
	if s.callErr != nil {
		return nil, s.callErr
	}
	b, err := json.Marshal(s.metadata)
	return b, err
}

func (s *stubClient) CallString(fn string, _ any) (string, error) {
	raw, err := s.CallJSON(fn, nil)
	if err != nil {
		return "", err
	}
	var s2 string
	if json.Unmarshal(raw, &s2) == nil {
		return s2, nil
	}
	return string(raw), nil
}

func validMeta() map[string]any {
	return map[string]any{
		"id":          "test-plugin",
		"name":        "Test Plugin",
		"version":     "1.0.0",
		"description": "A test plugin",
		"type":        "memory",
		"schema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func TestSmokeManifest_happy(t *testing.T) {
	c := &stubClient{
		info: protocol.PluginInfo{
			ID: "test-plugin", Version: "1.0.0", Type: "memory",
			Exports: []string{"get_metadata", "configure", "store"},
		},
		metadata: validMeta(),
	}
	report := &types.PluginReport{}
	err := manifest.SmokeManifest(c, report)
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", report.ID)
	assert.Equal(t, "memory", report.Type)
	assert.Contains(t, report.Exports, "configure")
}

func TestSmokeManifest_missingID(t *testing.T) {
	meta := validMeta()
	delete(meta, "id")
	c := &stubClient{
		info:     protocol.PluginInfo{ID: "test-plugin", Type: "memory", Exports: []string{"get_metadata", "configure"}},
		metadata: meta,
	}
	report := &types.PluginReport{}
	err := manifest.SmokeManifest(c, report)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.id")
}

func TestSmokeManifest_missingName(t *testing.T) {
	meta := validMeta()
	delete(meta, "name")
	c := &stubClient{info: protocol.PluginInfo{ID: "test-plugin", Type: "memory"}, metadata: meta}
	report := &types.PluginReport{}
	err := manifest.SmokeManifest(c, report)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestSmokeManifest_missingVersion(t *testing.T) {
	meta := validMeta()
	delete(meta, "version")
	c := &stubClient{info: protocol.PluginInfo{ID: "test-plugin", Type: "memory"}, metadata: meta}
	report := &types.PluginReport{}
	err := manifest.SmokeManifest(c, report)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestSmokeManifest_missingDescription(t *testing.T) {
	meta := validMeta()
	delete(meta, "description")
	c := &stubClient{info: protocol.PluginInfo{ID: "test-plugin", Type: "memory"}, metadata: meta}
	report := &types.PluginReport{}
	err := manifest.SmokeManifest(c, report)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestSmokeManifest_missingType(t *testing.T) {
	meta := validMeta()
	delete(meta, "type")
	c := &stubClient{info: protocol.PluginInfo{ID: "test-plugin"}, metadata: meta}
	report := &types.PluginReport{}
	err := manifest.SmokeManifest(c, report)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

func TestSmokeManifest_typeMismatch(t *testing.T) {
	meta := validMeta()
	meta["type"] = "ai"
	c := &stubClient{info: protocol.PluginInfo{ID: "test-plugin", Type: "memory"}, metadata: meta}
	report := &types.PluginReport{Type: "memory"}
	err := manifest.SmokeManifest(c, report)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestBaseRequiredExports(t *testing.T) {
	assert.Contains(t, manifest.BaseRequiredExports, "get_metadata")
	assert.Contains(t, manifest.BaseRequiredExports, "configure")
}
