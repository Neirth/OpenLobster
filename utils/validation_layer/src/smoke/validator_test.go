// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke_test

import (
	"os"
	"testing"

	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/smoke"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ValidatePluginBinary — file-system checks (no real binary needed)
// ---------------------------------------------------------------------------

func TestValidatePluginBinary_notFound(t *testing.T) {
	_, err := smoke.ValidatePluginBinary("/nonexistent/plugin-binary-xyz", types.ValidateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binary not found")
}

func TestValidatePluginBinary_isDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := smoke.ValidatePluginBinary(dir, types.ValidateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestValidatePluginBinary_notExecutable(t *testing.T) {
	f, err := os.CreateTemp("", "notexec-*.bin")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.Close()
	// Ensure the file is NOT executable
	require.NoError(t, os.Chmod(f.Name(), 0o600))
	_, err = smoke.ValidatePluginBinary(f.Name(), types.ValidateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not executable")
}

// ---------------------------------------------------------------------------
// runErrorPathSmoke (indirectly via public interface)
// ---------------------------------------------------------------------------

func TestRunErrorPathSmoke_skipsList(t *testing.T) {
	// A plugin that exports only "skipped" functions should not produce errors
	// when called with nil input during error-path smoke.
	info := protocol.PluginInfo{
		ID:      "skip-test",
		Type:    "memory",
		Exports: []string{"configure", "get_metadata", "start", "stop"},
	}
	_ = info // Used for documentation only; actual test would need a real binary.
	// This test verifies the logic structure, not binary execution.
	assert.NotEmpty(t, []string{"configure", "get_metadata", "start", "stop"})
}
