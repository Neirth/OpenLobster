// Copyright (c) OpenLobster contributors. See LICENSE for details.

package config_test

import (
	"os"
	"testing"

	"github.com/neirth/openlobster/utils/validation_layer/src/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureConfigValue_setsWhenAbsent(t *testing.T) {
	cfg := map[string]any{}
	config.EnsureConfigValue(cfg, "key", "default")
	assert.Equal(t, "default", cfg["key"])
}

func TestEnsureConfigValue_preservesExisting(t *testing.T) {
	cfg := map[string]any{"key": "existing"}
	config.EnsureConfigValue(cfg, "key", "default")
	assert.Equal(t, "existing", cfg["key"])
}

func TestEnsureConfigValue_replacesBlank(t *testing.T) {
	cfg := map[string]any{"key": "   "}
	config.EnsureConfigValue(cfg, "key", "default")
	assert.Equal(t, "default", cfg["key"])
}

func TestEnsureConfigValue_nilSafe(t *testing.T) {
	// Should not panic
	config.EnsureConfigValue(nil, "key", "default")
}

func TestFillMissingConfigFromEnv_picks_env(t *testing.T) {
	t.Setenv("TEST_FOO_KEY", "fromenv")
	cfg := map[string]any{}
	config.FillMissingConfigFromEnv(cfg, map[string][]string{
		"foo": {"TEST_FOO_KEY"},
	})
	assert.Equal(t, "fromenv", cfg["foo"])
}

func TestFillMissingConfigFromEnv_skipsIfPresent(t *testing.T) {
	t.Setenv("TEST_BAR_KEY", "fromenv")
	cfg := map[string]any{"bar": "already"}
	config.FillMissingConfigFromEnv(cfg, map[string][]string{
		"bar": {"TEST_BAR_KEY"},
	})
	assert.Equal(t, "already", cfg["bar"])
}

func TestFillMissingConfigFromEnv_firstWins(t *testing.T) {
	os.Setenv("TEST_FIRST_KEY", "first")
	os.Setenv("TEST_SECOND_KEY", "second")
	defer os.Unsetenv("TEST_FIRST_KEY")
	defer os.Unsetenv("TEST_SECOND_KEY")
	cfg := map[string]any{}
	config.FillMissingConfigFromEnv(cfg, map[string][]string{
		"val": {"TEST_FIRST_KEY", "TEST_SECOND_KEY"},
	})
	assert.Equal(t, "first", cfg["val"])
}

func TestConfigString_returnsValue(t *testing.T) {
	cfg := map[string]any{"k": "  hello  "}
	require.Equal(t, "hello", config.ConfigString(cfg, "k"))
}

func TestConfigString_returnsEmptyWhenMissing(t *testing.T) {
	cfg := map[string]any{}
	require.Equal(t, "", config.ConfigString(cfg, "missing"))
}

func TestConfigString_nilSafe(t *testing.T) {
	require.Equal(t, "", config.ConfigString(nil, "k"))
}

func TestFallbackString_usesFirst(t *testing.T) {
	assert.Equal(t, "a", config.FallbackString("a", "b"))
}

func TestFallbackString_usesFallback(t *testing.T) {
	assert.Equal(t, "b", config.FallbackString("", "b"))
}

func TestFallbackString_trimsBoth(t *testing.T) {
	assert.Equal(t, "b", config.FallbackString("  ", "  b  "))
}
