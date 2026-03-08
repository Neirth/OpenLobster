package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenBAOProvider(t *testing.T) {
	p, err := NewOpenBAOProvider("http://localhost:8200", "token", "secret")
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewOpenBAOProvider_DefaultMount(t *testing.T) {
	p, err := NewOpenBAOProvider("http://localhost", "", "")
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestOpenBAOProvider_Get(t *testing.T) {
	p, _ := NewOpenBAOProvider("http://localhost", "t", "m")
	ctx := context.Background()
	_, err := p.Get(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOpenBAOProvider_Set(t *testing.T) {
	p, _ := NewOpenBAOProvider("http://localhost", "t", "m")
	ctx := context.Background()
	err := p.Set(ctx, "key", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOpenBAOProvider_Delete(t *testing.T) {
	p, _ := NewOpenBAOProvider("http://localhost", "t", "m")
	ctx := context.Background()
	err := p.Delete(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOpenBAOProvider_List(t *testing.T) {
	p, _ := NewOpenBAOProvider("http://localhost", "t", "m")
	ctx := context.Background()
	_, err := p.List(ctx, "prefix")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
