package terminal

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterOpenLobsterFromEnv_SystemEnv(t *testing.T) {
	// Set some OPENLOBSTER_ vars and a normal var
	os.Setenv("OPENLOBSTER_SECRET_KEY", "test-key")
	os.Setenv("OPENLOBSTER_TOKEN", "token123")
	os.Setenv("PATH", "/usr/bin")
	os.Setenv("HOME", "/home/user")
	defer func() {
		os.Unsetenv("OPENLOBSTER_SECRET_KEY")
		os.Unsetenv("OPENLOBSTER_TOKEN")
	}()

	env := FilterOpenLobsterFromEnv(os.Environ())

	hasPath := false
	hasHome := false
	hasSecretKey := false
	hasToken := false
	for _, e := range env {
		if len(e) >= 4 && e[:4] == "PATH" {
			hasPath = true
		}
		if len(e) >= 4 && e[:4] == "HOME" {
			hasHome = true
		}
		if len(e) >= 20 && e[:20] == "OPENLOBSTER_SECRET_K" {
			hasSecretKey = true
		}
		if len(e) >= 17 && e[:17] == "OPENLOBSTER_TOKEN" {
			hasToken = true
		}
	}

	assert.True(t, hasPath, "PATH should be present")
	assert.True(t, hasHome, "HOME should be present")
	assert.False(t, hasSecretKey, "OPENLOBSTER_SECRET_KEY must not leak")
	assert.False(t, hasToken, "OPENLOBSTER_TOKEN must not leak")
}

func TestFilterOpenLobsterFromEnv(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"OPENLOBSTER_SECRET_KEY=secret123",
		"HOME=/home/user",
		"openlobster_token=leak",
		"OPENLOBSTER_CONFIG_PATH=/etc/config",
	}
	filtered := FilterOpenLobsterFromEnv(env)

	hasPath := false
	hasHome := false
	for _, e := range filtered {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
		}
		if strings.HasPrefix(e, "HOME=") {
			hasHome = true
		}
		if strings.HasPrefix(e, "OPENLOBSTER_") {
			t.Errorf("OPENLOBSTER_* must be filtered from user env, got %q", e)
		}
	}
	assert.True(t, hasPath)
	assert.True(t, hasHome)
	assert.Len(t, filtered, 2) // PATH and HOME only
}
