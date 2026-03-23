// Copyright (c) OpenLobster contributors. See LICENSE for details.

package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SystemFilesAdapter — additional coverage
// ---------------------------------------------------------------------------

func TestNewSystemFilesAdapter(t *testing.T) {
	a := NewSystemFilesAdapter("/tmp/workspace")
	assert.NotNil(t, a)
}

func TestSystemFilesAdapter_ListFiles_AllPresent(t *testing.T) {
	dir := t.TempDir()
	for _, name := range AllowedWorkspaceFiles {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("content of "+name), 0644))
	}

	a := NewSystemFilesAdapter(dir)
	files, err := a.ListFiles()
	require.NoError(t, err)
	assert.Len(t, files, len(AllowedWorkspaceFiles))

	for _, f := range files {
		assert.NotEmpty(t, f.Name)
		assert.NotEmpty(t, f.Path)
		assert.NotEmpty(t, f.Content)
		assert.NotEmpty(t, f.LastModified)
	}
}

func TestSystemFilesAdapter_ListFiles_SomeMissing(t *testing.T) {
	dir := t.TempDir()
	// Only create the first allowed file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, AllowedWorkspaceFiles[0]), []byte("data"), 0644))

	a := NewSystemFilesAdapter(dir)
	files, err := a.ListFiles()
	require.NoError(t, err)
	assert.Len(t, files, len(AllowedWorkspaceFiles))

	// Files that don't exist should have empty content.
	for _, f := range files {
		if f.Name != AllowedWorkspaceFiles[0] {
			assert.Equal(t, "", f.Content)
			assert.Equal(t, "", f.LastModified)
		}
	}
}

func TestSystemFilesAdapter_WriteFile_AllAllowed(t *testing.T) {
	dir := t.TempDir()
	a := NewSystemFilesAdapter(dir)

	for _, name := range AllowedWorkspaceFiles {
		err := a.WriteFile(name, "hello "+name)
		require.NoError(t, err)

		data, readErr := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, readErr)
		assert.Equal(t, "hello "+name, string(data))
	}
}

func TestSystemFilesAdapter_WriteFile_NotAllowed_Slash(t *testing.T) {
	a := NewSystemFilesAdapter(t.TempDir())
	err := a.WriteFile("config.yaml", "data")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not writable")
}

func TestSystemFilesAdapter_WriteFile_EmptyName(t *testing.T) {
	a := NewSystemFilesAdapter(t.TempDir())
	err := a.WriteFile("", "data")
	assert.Error(t, err)
}

func TestSystemFilesAdapter_WriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	name := AllowedWorkspaceFiles[0]
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("original"), 0644))

	a := NewSystemFilesAdapter(dir)
	err := a.WriteFile(name, "updated")
	require.NoError(t, err)

	data, _ := os.ReadFile(filepath.Join(dir, name))
	assert.Equal(t, "updated", string(data))
}

// ---------------------------------------------------------------------------
// Adapter.WriteBytes and ReadFileBytes — additional coverage
// ---------------------------------------------------------------------------

func TestAdapter_WriteBytes(t *testing.T) {
	tmpDir := t.TempDir()
	a := NewAdapter("/nonexistent-config.yaml")

	path := filepath.Join(tmpDir, "bytes.bin")
	data := []byte{0x01, 0x02, 0x03}

	err := a.WriteBytes(t.Context(), path, data)
	require.NoError(t, err)

	read, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, read)
}

func TestAdapter_WriteBytes_Protected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("x"), 0644))

	a := NewAdapter(configPath)
	err := a.WriteBytes(t.Context(), configPath, []byte("evil"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestAdapter_WriteBytes_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	a := NewAdapter("/nonexistent-config.yaml")

	path := filepath.Join(tmpDir, "newdir", "data.bin")
	err := a.WriteBytes(t.Context(), path, []byte{0xFF})
	require.NoError(t, err)

	read, _ := os.ReadFile(path)
	assert.Equal(t, []byte{0xFF}, read)
}

func TestAdapter_ReadFileBytes(t *testing.T) {
	tmpDir := t.TempDir()
	a := NewAdapter("/nonexistent-config.yaml")

	path := filepath.Join(tmpDir, "file.bin")
	data := []byte{1, 2, 3}
	require.NoError(t, os.WriteFile(path, data, 0644))

	out, err := a.ReadFileBytes(t.Context(), path)
	require.NoError(t, err)
	assert.Equal(t, data, out)
}

func TestAdapter_ReadFileBytes_Protected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("x"), 0644))

	a := NewAdapter(configPath)
	_, err := a.ReadFileBytes(t.Context(), configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestAdapter_ReadFileBytes_NotFound(t *testing.T) {
	a := NewAdapter("/nonexistent-config.yaml")
	_, err := a.ReadFileBytes(t.Context(), "/nonexistent/path.bin")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// isProtected — edge cases
// ---------------------------------------------------------------------------

func TestAdapter_isProtected_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	a := NewAdapter(configPath)
	absConfig, _ := filepath.Abs(configPath)
	assert.True(t, a.isProtected(absConfig))
}

func TestAdapter_isProtected_ChildPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	a := NewAdapter(configPath)
	absConfig, _ := filepath.Abs(configPath)
	child := absConfig + string(filepath.Separator) + "child"
	assert.True(t, a.isProtected(child))
}

func TestAdapter_isProtected_UnrelatedPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	a := NewAdapter(configPath)
	assert.False(t, a.isProtected("/tmp/innocent.txt"))
}

func TestAdapter_isProtected_EmptyConfigPath(t *testing.T) {
	a := &Adapter{configPath: ""}
	assert.False(t, a.isProtected("/any/path"))
}
