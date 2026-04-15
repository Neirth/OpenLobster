// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/neirth/openlobster/utils/validation_layer/src/config"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

func runSecretsSmoke(client protocol.PluginClient, report *types.PluginReport, opts types.ValidateOptions, file, tmpDir string) {
	cfg := cloneMap(opts.SmokeConfig)
	config.EnsureConfigValue(cfg, "path", filepath.Join(tmpDir, "secrets.json"))
	config.EnsureConfigValue(cfg, "key", "smoke-key")
	config.FillMissingConfigFromEnv(cfg, map[string][]string{
		"url":   {"OPENLOBSTER_SMOKE_SECRETS_URL", "OPENLOBSTER_SMOKE_OPENBAO_URL"},
		"token": {"OPENLOBSTER_SMOKE_SECRETS_TOKEN", "OPENLOBSTER_SMOKE_OPENBAO_TOKEN"},
		"mount": {"OPENLOBSTER_SMOKE_SECRETS_MOUNT", "OPENLOBSTER_SMOKE_OPENBAO_MOUNT"},
	})
	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, "secrets", err.Error(), file)
		return
	}

	const key = "smoke/key"
	const value = "smoke-value"

	if _, err := client.CallJSON("set", map[string]any{"config": cfg, "key": key, "value": value}); err != nil {
		addSmokeFailure(report, "secrets.set", err.Error(), file)
		return
	}

	getRaw, err := client.CallJSON("get", map[string]any{"config": cfg, "key": key})
	if err != nil {
		addSmokeFailure(report, "secrets.get", err.Error(), file)
		return
	}
	var getResp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(getRaw, &getResp); err != nil || getResp.Value != value {
		addSmokeFailure(report, "secrets.get", "value mismatch", file)
		return
	}

	// Verify the key appears in the list.
	if client.HasFunction("list") {
		if err := assertSecretsKeyInList(client, cfg, key, true, report, file); err != nil {
			return
		}
	}

	if _, err := client.CallJSON("delete", map[string]any{"config": cfg, "key": key}); err != nil {
		addSmokeFailure(report, "secrets.delete", err.Error(), file)
		return
	}

	// After deletion the key must be absent from the list.
	if client.HasFunction("list") {
		_ = assertSecretsKeyInList(client, cfg, key, false, report, file)
	}
}

// assertSecretsKeyInList calls list and checks whether key is present or
// absent. shouldExist controls which outcome is the pass condition.
func assertSecretsKeyInList(
	client protocol.PluginClient,
	cfg map[string]any,
	key string,
	shouldExist bool,
	report *types.PluginReport,
	file string,
) error {
	listRaw, err := client.CallJSON("list", map[string]any{"config": cfg})
	if err != nil {
		addSmokeFailure(report, "secrets.list", err.Error(), file)
		return err
	}

	keys := parseSecretsList(listRaw)
	if keys == nil {
		addSmokeFailure(report, "secrets.list", "invalid list response format", file)
		return fmt.Errorf("invalid list response")
	}

	found := false
	for _, k := range keys {
		if strings.TrimSpace(k) == key {
			found = true
			break
		}
	}

	if shouldExist && !found {
		msg := fmt.Errorf("key %q not found in list after set", key)
		addSmokeFailure(report, "secrets.list", msg.Error(), file)
		return msg
	}
	if !shouldExist && found {
		msg := fmt.Errorf("key %q still present in list after delete", key)
		addSmokeFailure(report, "secrets.list", msg.Error(), file)
		return msg
	}
	return nil
}

// parseSecretsList accepts both `["key"]` and `{"keys":["key"]}` formats.
func parseSecretsList(raw json.RawMessage) []string {
	// Try flat array first.
	var flat []string
	if json.Unmarshal(raw, &flat) == nil {
		return flat
	}
	// Try object with "keys" field.
	var obj struct {
		Keys []string `json:"keys"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Keys
	}
	return nil
}
