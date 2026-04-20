// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package plugin provides ABI-level helpers for interacting with OpenLobster plugins.
package plugin

import schemacontract "github.com/neirth/openlobster/utils/schema_contract"

// ValidateConfigSchema validates a runtime configuration map against a
// plugin-provided JSON Schema fragment. Delegates to the canonical
// implementation in utils/schema_contract.
func ValidateConfigSchema(schemaJSON []byte, cfg map[string]interface{}) error {
	return schemacontract.ValidateConfigSchema(schemaJSON, cfg)
}
