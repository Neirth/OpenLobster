package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretKey_Default(t *testing.T) {
	os.Unsetenv(envSecretKey)
	defer func() { _ = os.Unsetenv(envSecretKey) }()

	key := SecretKey()
	assert.Len(t, key, 32, "key must be 32 bytes")
	assert.NotEmpty(t, key)
}

func TestSecretKey_FromBase64(t *testing.T) {
	// 32 bytes in base64
	b64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	os.Setenv(envSecretKey, b64)
	defer os.Unsetenv(envSecretKey)

	key := SecretKey()
	assert.Len(t, key, 32)
}

func TestSecretKey_FromPassphrase(t *testing.T) {
	os.Setenv(envSecretKey, "my-secret-passphrase")
	defer os.Unsetenv(envSecretKey)

	key := SecretKey()
	assert.Len(t, key, 32)
	// Passphrase is sha256 hashed
	key2 := SecretKey()
	assert.Equal(t, key, key2, "same passphrase must yield same key")
}

func TestConfigEncryptEnabled_Default(t *testing.T) {
	os.Unsetenv(envConfigEncrypt)
	defer os.Unsetenv(envConfigEncrypt)
	assert.True(t, ConfigEncryptEnabled(), "unset => default 1 => enabled")
}

func TestConfigEncryptEnabled_Explicit1(t *testing.T) {
	os.Setenv(envConfigEncrypt, "1")
	defer os.Unsetenv(envConfigEncrypt)
	assert.True(t, ConfigEncryptEnabled())
}

func TestConfigEncryptEnabled_Explicit0(t *testing.T) {
	os.Setenv(envConfigEncrypt, "0")
	defer os.Unsetenv(envConfigEncrypt)
	assert.False(t, ConfigEncryptEnabled())
}

func TestConfigEncryptEnabled_Invalid(t *testing.T) {
	os.Setenv(envConfigEncrypt, "xyz")
	defer os.Unsetenv(envConfigEncrypt)
	assert.True(t, ConfigEncryptEnabled(), "invalid => default enabled")
}
