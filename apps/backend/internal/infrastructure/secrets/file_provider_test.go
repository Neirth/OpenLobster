package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileSecretsProvider_InvalidKey(t *testing.T) {
	_, err := NewFileSecretsProvider("/tmp/secrets.json", []byte("short"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestNewFileSecretsProvider_NonExistentFile(t *testing.T) {
	key := make([]byte, 32)
	p, err := NewFileSecretsProvider(filepath.Join(t.TempDir(), "nonexistent.json"), key)
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestFileSecretsProvider_GetSetDeleteList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	p, err := NewFileSecretsProvider(path, key)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = p.Get(ctx, "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = p.Set(ctx, "api_key", "secret123")
	require.NoError(t, err)

	val, err := p.Get(ctx, "api_key")
	require.NoError(t, err)
	assert.Equal(t, "secret123", val)

	keys, err := p.List(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, keys, "api_key")

	keys, err = p.List(ctx, "api_")
	require.NoError(t, err)
	assert.Contains(t, keys, "api_key")

	keys, err = p.List(ctx, "other")
	require.NoError(t, err)
	assert.Empty(t, keys)

	err = p.Delete(ctx, "api_key")
	require.NoError(t, err)

	_, err = p.Get(ctx, "api_key")
	assert.Error(t, err)
}

func TestFileSecretsProvider_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	p1, err := NewFileSecretsProvider(path, key)
	require.NoError(t, err)
	err = p1.Set(context.Background(), "x", "y")
	require.NoError(t, err)

	p2, err := NewFileSecretsProvider(path, key)
	require.NoError(t, err)
	val, err := p2.Get(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, "y", val)
}

func TestFileSecretsProvider_ListPrefixEdgeCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	key := make([]byte, 32)
	p, err := NewFileSecretsProvider(path, key)
	require.NoError(t, err)
	ctx := context.Background()

	_ = p.Set(ctx, "abc", "v")
	_ = p.Set(ctx, "ab", "v")

	keys, _ := p.List(ctx, "ab")
	assert.Len(t, keys, 2)
}

func TestFileSecretsProvider_List_EmptyPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	key := make([]byte, 32)
	p, err := NewFileSecretsProvider(path, key)
	require.NoError(t, err)
	ctx := context.Background()

	err = p.Set(ctx, "k1", "v1")
	require.NoError(t, err)
	keys, err := p.List(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, keys, "k1")
}

func TestFileSecretsProvider_List_PrefixNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	key := make([]byte, 32)
	p, err := NewFileSecretsProvider(path, key)
	require.NoError(t, err)
	ctx := context.Background()

	err = p.Set(ctx, "api_key", "v")
	require.NoError(t, err)
	keys, err := p.List(ctx, "x")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestFileSecretsProvider_LoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	key := make([]byte, 32)
	require.NoError(t, os.WriteFile(path, []byte("x"), 0600))

	_, err := NewFileSecretsProvider(path, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext")
}
