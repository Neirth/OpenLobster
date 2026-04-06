// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"github.com/neirth/openlobster/internal/domain/ports"
	pluginabi "github.com/neirth/openlobster/internal/infrastructure/plugin/abi"
)

// AIWrapper re-exports the ABI AI wrapper from the root plugin package path.
type AIWrapper = pluginabi.AIWrapper

func NewAIWrapper(p ports.PluginPort, cfg map[string]interface{}) *AIWrapper {
	return pluginabi.NewAIWrapper(p, cfg)
}
