package mcp

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSecrets struct {
	mock.Mock
}

func (m *mockSecrets) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *mockSecrets) Set(ctx context.Context, key, value string) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *mockSecrets) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockSecrets) List(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	return args.Get(0).([]string), args.Error(1)
}

func TestNewMCPClientSDK(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	assert.NotNil(t, client)
}

func TestMCPClientSDK_UnknownServerType(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), ServerConfig{
		Name: "test",
		Type: "invalid",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown server type")
}

func TestMCPClientSDK_ListTools_Empty(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	tools, err := client.ListTools(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, tools)
}

func TestMCPClientSDK_GetServerTools_NotFound(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	tools := client.GetServerTools("nonexistent")

	assert.Nil(t, tools)
}

func TestMCPClientSDK_CallTool_InvalidName(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	_, err := client.CallTool(context.Background(), "", nil)

	assert.Error(t, err)
}

func TestMCPClientSDK_CallTool_ServerNotConnected(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	_, err := client.CallTool(context.Background(), "server:tool", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server not connected")
}

func TestMCPClientSDK_Connect_InvalidURL(t *testing.T) {
	secretsProvider := new(mockSecrets)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()
	client := NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), ServerConfig{
		Name: "test",
		Type: "http",
		URL:  "http://invalid:99999",
	})

	assert.Error(t, err)
}

func TestMCPClientSDK_Close(t *testing.T) {
	secretsProvider := new(mockSecrets)
	client := NewMCPClientSDK(secretsProvider)

	err := client.Close()
	assert.NoError(t, err)
}

var _ secrets.SecretsProvider = (*mockSecrets)(nil)
