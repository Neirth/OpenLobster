// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package manifest validates the plugin's get_metadata response and exports.
package manifest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	schemacontract "github.com/neirth/openlobster/utils/schema_contract"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

// BaseRequiredExports lists the minimum RPC functions every plugin must export.
var BaseRequiredExports = []string{
	"configure",
}

// SmokeManifest validates the plugin manifest by calling get_metadata and
// checking the structural constraints. It also populates report fields from
// the runtime GetInfo response.
func SmokeManifest(client protocol.PluginClient, report *types.PluginReport) error {
	info := client.Info()

	if strings.TrimSpace(info.ID) != "" && strings.TrimSpace(report.ID) == "" {
		report.ID = strings.TrimSpace(info.ID)
	}
	if len(info.Exports) > 0 && len(report.Exports) == 0 {
		exps := make([]string, 0, len(info.Exports))
		for _, e := range info.Exports {
			if trimmed := strings.TrimSpace(e); trimmed != "" {
				exps = append(exps, trimmed)
			}
		}
		sort.Strings(exps)
		report.Exports = exps
	}
	if strings.TrimSpace(info.Type) != "" && strings.TrimSpace(report.Type) == "" {
		report.Type = strings.TrimSpace(info.Type)
	}

	// In the new protocol, metadata IS the info returned by get_info.
	// client.Info() already contains the unmarshaled response from get_info.
	if info.ID == "" {
		return fmt.Errorf("plugin id is required")
	}
	if strings.TrimSpace(report.ID) != "" && info.ID != strings.TrimSpace(report.ID) {
		return fmt.Errorf("plugin id %q does not match expected ID %q", info.ID, report.ID)
	}
	if info.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if info.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if info.Description == "" {
		return fmt.Errorf("plugin description is required")
	}
	if info.Type == "" {
		return fmt.Errorf("plugin type is required")
	} else if strings.TrimSpace(report.Type) != "" && info.Type != strings.TrimSpace(report.Type) {
		return fmt.Errorf("plugin type %q does not match expected type %q", info.Type, report.Type)
	}

	if len(info.Schema) > 0 && !json.Valid(info.Schema) {
		return fmt.Errorf("plugin schema must be valid JSON")
	}
	if len(info.Schema) > 0 {
		if err := schemacontract.ValidateSchemaStructure(info.Schema); err != nil {
			return fmt.Errorf("plugin schema: %w", err)
		}
	}

	if len(info.Properties) > 0 && !json.Valid(info.Properties) {
		return fmt.Errorf("plugin properties must be valid JSON object")
	}
	if len(info.Properties) > 0 {
		var propertiesObj map[string]any
		if err := json.Unmarshal(info.Properties, &propertiesObj); err != nil {
			return fmt.Errorf("plugin properties root must be an object")
		}
	}

	return nil
}
