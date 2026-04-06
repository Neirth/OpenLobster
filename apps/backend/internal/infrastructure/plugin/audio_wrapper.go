// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"github.com/neirth/openlobster/internal/domain/ports"
	pluginabi "github.com/neirth/openlobster/internal/infrastructure/plugin/abi"
)

// AudioWrapper re-exports the ABI audio wrapper from the root plugin package path.
type AudioWrapper = pluginabi.AudioWrapper

func NewAudioWrapper(p ports.PluginPort, cfg map[string]interface{}) *AudioWrapper {
	return pluginabi.NewAudioWrapper(p, cfg)
}
