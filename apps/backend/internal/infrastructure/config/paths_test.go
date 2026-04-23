package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvePaths(t *testing.T) {
	baseDir := "/tmp/openlobster-test"
	cfg := &Config{
		BaseDir: baseDir,
		Memory: MemoryConfig{
			File: MemoryFileConfig{Path: "data/memory.gml"},
		},
		Logging: LoggingConfig{
			Path: "logs",
		},
		Plugins: PluginsConfig{
			Dir:     "plugins",
			DataDir: ".",
		},
		Workspace: WorkspaceConfig{
			Path: "workspace",
		},
	}

	cfg.ResolvePaths()

	assert.Equal(t, filepath.Join(baseDir, "data/memory.gml"), cfg.Memory.File.Path)
	assert.Equal(t, filepath.Join(baseDir, "logs"), cfg.Logging.Path)
	assert.Equal(t, filepath.Join(baseDir, "plugins"), cfg.Plugins.Dir)
	assert.Equal(t, baseDir, cfg.Plugins.DataDir)
	assert.Equal(t, filepath.Join(baseDir, "workspace"), cfg.Workspace.Path)
}

func TestResolvePaths_AbsolutePreserved(t *testing.T) {
	baseDir := "/tmp/openlobster-test"
	absPath := "/var/lib/external-plugins"
	cfg := &Config{
		BaseDir: baseDir,
		Plugins: PluginsConfig{
			Dir: absPath,
		},
	}

	cfg.ResolvePaths()

	assert.Equal(t, absPath, cfg.Plugins.Dir, "Absolute paths should not be rebased")
}
