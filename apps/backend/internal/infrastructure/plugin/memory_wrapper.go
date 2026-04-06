// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"github.com/neirth/openlobster/internal/domain/ports"
	pluginabi "github.com/neirth/openlobster/internal/infrastructure/plugin/abi"
)

// MemoryWrapper re-exports the ABI memory wrapper from the root plugin package path.
type MemoryWrapper = pluginabi.MemoryWrapper

func NewMemoryWrapper(p ports.PluginPort, cfg map[string]interface{}) *MemoryWrapper {
	return pluginabi.NewMemoryWrapper(p, cfg)
}
