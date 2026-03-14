package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapter(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")
	assert.NotNil(t, adapter)
}

func TestReadFile(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	result, err := adapter.ReadFile(t.Context(), testFile)
	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestReadFile_AbsolutePath(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "test content"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	absPath, _ := filepath.Abs(testFile)
	result, err := adapter.ReadFile(t.Context(), absPath)
	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestReadFile_NotFound(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	_, err := adapter.ReadFile(t.Context(), "/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestWriteFile(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "newfile.txt")
	content := "new content"

	err := adapter.WriteFile(t.Context(), testFile, content)
	assert.NoError(t, err)

	readContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, content, string(readContent))
}

func TestWriteFile_CreateDir(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "newfile.txt")
	content := "content"

	err := adapter.WriteFile(t.Context(), testFile, content)
	assert.NoError(t, err)

	readContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, content, string(readContent))
}

func TestWriteFile_Overwrite(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("original"), 0644)
	require.NoError(t, err)

	err = adapter.WriteFile(t.Context(), testFile, "new content")
	assert.NoError(t, err)

	readContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "new content", string(readContent))
}

func TestEditFile(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	original := "hello world"
	err := os.WriteFile(testFile, []byte(original), 0644)
	require.NoError(t, err)

	err = adapter.EditFile(t.Context(), testFile, "world", "go")
	assert.NoError(t, err)

	readContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "hello go", string(readContent))
}

func TestEditFile_NotFound(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	err := adapter.EditFile(t.Context(), "/nonexistent/file.txt", "old", "new")
	assert.Error(t, err)
}

func TestEditFile_ContentNotFound(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello"), 0644)
	require.NoError(t, err)

	err = adapter.EditFile(t.Context(), testFile, "nonexistent", "new")
	assert.Error(t, err)
}

func TestListContent(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte(""), 0644)

	entries, err := adapter.ListContent(t.Context(), tmpDir)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 2)
}

func TestListContent_EmptyDir(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()

	entries, err := adapter.ListContent(t.Context(), tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestListContent_NotFound(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	_, err := adapter.ListContent(t.Context(), "/nonexistent/dir")
	assert.Error(t, err)
}

func TestListContent_FileEntries(t *testing.T) {
	adapter := NewAdapter("/tmp/nonexistent-config.yaml")

	tmpDir := t.TempDir()
	subdir := filepath.Join(tmpDir, "mydir")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "myfile.txt"), []byte("content"), 0644)

	entries, err := adapter.ListContent(t.Context(), tmpDir)
	assert.NoError(t, err)

	var fileEntry, dirEntry *mcp.FileEntry
	for i := range entries {
		e := &entries[i]
		if e.Name == "myfile.txt" {
			fileEntry = e
		}
		if e.Name == "mydir" {
			dirEntry = e
		}
	}

	assert.NotNil(t, fileEntry)
	assert.False(t, fileEntry.IsDir)
	assert.NotNil(t, dirEntry)
	assert.True(t, dirEntry.IsDir)
}

// ─── Configuration file protection ───────────────────────────────────────────

func TestReadFile_ProtectedConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("secret: key"), 0644))

	adapter := NewAdapter(configPath)

	_, err := adapter.ReadFile(t.Context(), configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "protected")
}

func TestWriteFile_ProtectedConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("original"), 0644))

	adapter := NewAdapter(configPath)

	err := adapter.WriteFile(t.Context(), configPath, "malicious")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")

	content, _ := os.ReadFile(configPath)
	assert.Equal(t, "original", string(content))
}

func TestEditFile_ProtectedConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("key: val"), 0644))

	adapter := NewAdapter(configPath)

	err := adapter.EditFile(t.Context(), configPath, "val", "changed")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestListContent_ProtectedConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("x"), 0644))

	adapter := NewAdapter(configPath)

	_, err := adapter.ListContent(t.Context(), filepath.Dir(configPath))
	assert.NoError(t, err)
	// Listing the parent directory is allowed; only ReadFile/WriteFile/EditFile for the specific file are blocked.
}

func TestNewAdapter_EmptyPath(t *testing.T) {
	// Empty path: filepath.Abs fails, configPath is used as-is.
	adapter := NewAdapter("")
	assert.NotNil(t, adapter)
	// With an empty configPath, isProtected returns false; ReadFile for a normal path works.
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(f, []byte("ok"), 0644))
	content, err := adapter.ReadFile(t.Context(), f)
	assert.NoError(t, err)
	assert.Equal(t, "ok", content)
}
