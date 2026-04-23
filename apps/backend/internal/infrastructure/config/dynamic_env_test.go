package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDynamicEnvBinding(t *testing.T) {
	// Reset viper for a clean test
	viper.Reset()
	setDefaults()

	// 1. Simular un nuevo plugin de mensajería "mattermost" que NO está en el struct Config
	os.Setenv("OPENLOBSTER_CHANNELS_MATTERMOST_URL", "https://mattermost.internal")
	os.Setenv("OPENLOBSTER_CHANNELS_MATTERMOST_TOKEN", "mm-secret-123")
	
	// 2. Simular un nuevo proveedor de AI "pepito" que NO está en el struct ProvidersConfig
	os.Setenv("OPENLOBSTER_PROVIDERS_PEPITO_MODEL", "pepito-v1")
	os.Setenv("OPENLOBSTER_PROVIDERS_PEPITO_API_KEY", "pepito-key-xyz")

	defer func() {
		os.Unsetenv("OPENLOBSTER_CHANNELS_MATTERMOST_URL")
		os.Unsetenv("OPENLOBSTER_CHANNELS_MATTERMOST_TOKEN")
		os.Unsetenv("OPENLOBSTER_PROVIDERS_PEPITO_MODEL")
		os.Unsetenv("OPENLOBSTER_PROVIDERS_PEPITO_API_KEY")
	}()

	// Ejecutar el binding dinámico
	bindEnvFromOS()

	// TEST A: Verificar Mattermost (Channels)
	mmSub := viper.Sub("channels.mattermost")
	assert.NotNil(t, mmSub, "Viper should create a subtree for mattermost even if not in struct")
	if mmSub != nil {
		settings := mmSub.AllSettings()
		assert.Equal(t, "https://mattermost.internal", settings["url"])
		assert.Equal(t, "mm-secret-123", settings["token"])
	}

	// TEST B: Verificar Pepito (Providers)
	pepitoSub := viper.Sub("providers.pepito")
	assert.NotNil(t, pepitoSub, "Viper should create a subtree for pepito provider even if not in struct")
	if pepitoSub != nil {
		settings := pepitoSub.AllSettings()
		assert.Equal(t, "pepito-v1", settings["model"])
		assert.Equal(t, "pepito-key-xyz", settings["api_key"])
	}
}
