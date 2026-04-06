// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import pluginabi "github.com/neirth/openlobster/internal/infrastructure/plugin/abi"

func ValidateConfigSchema(schemaJSON []byte, cfg map[string]interface{}) error {
	return pluginabi.ValidateConfigSchema(schemaJSON, cfg)
}
