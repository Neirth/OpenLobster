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

	if err := validateSetInput(key, value); err != nil {
		addSmokeFailure(report, "secrets.set.validation", err.Error(), file)
		return
	}

	setRaw, err := client.CallJSON("set", map[string]any{"config": cfg, "key": key, "value": value})
	if err != nil {
		addSmokeFailure(report, "secrets.set", err.Error(), file)
		return
	}

	if err := validateSetResponse(setRaw, key, value); err != nil {
		addSmokeFailure(report, "secrets.set.response_validation", err.Error(), file)
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

	if err := validateGetResponse(getRaw, key); err != nil {
		addSmokeFailure(report, "secrets.get.validation", err.Error(), file)
		return
	}

	if client.HasFunction("list") {
		if err := assertSecretsKeyInList(client, cfg, key, true, report, file); err != nil {
			return
		}

		if err := validateListKeysExist(client, cfg, []string{key}); err != nil {
			addSmokeFailure(report, "secrets.list.validation", err.Error(), file)
			return
		}
	}

	delRaw, err := client.CallJSON("delete", map[string]any{"config": cfg, "key": key})
	if err != nil {
		addSmokeFailure(report, "secrets.delete", err.Error(), file)
		return
	}

	if err := validateDeleteResponse(delRaw, key); err != nil {
		addSmokeFailure(report, "secrets.delete.validation", err.Error(), file)
		return
	}

	if client.HasFunction("list") {
		if err := assertSecretsKeyInList(client, cfg, key, false, report, file); err != nil {
			return
		}
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

func validateSetInput(key, value string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("FATAL: set operation has empty key - key is required")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("FATAL: set operation has empty value - value is required")
	}
	return nil
}

func validateSetResponse(raw []byte, expectedKey, expectedValue string) error {
	if len(raw) == 0 {
		return fmt.Errorf("FATAL: set operation returned empty response")
	}
	return nil
}

func validateGetResponse(raw []byte, key string) error {
	if len(raw) == 0 {
		return fmt.Errorf("FATAL: get operation returned empty response")
	}
	var resp struct {
		Value    string `json:"value"`
		Data     any    `json:"data"`
		Response any    `json:"response"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("FATAL: get operation returned invalid JSON: %w", err)
	}
	if resp.Value == "" && resp.Data == nil && resp.Response == nil {
		return fmt.Errorf("FATAL: get operation returned nil/empty value for key %q - key must exist before get", key)
	}
	return nil
}

func validateDeleteResponse(raw []byte, key string) error {
	if len(raw) == 0 {
		return fmt.Errorf("FATAL: delete operation returned empty response")
	}
	var resp struct {
		Deleted bool   `json:"deleted"`
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err == nil {
		if resp.Error != "" && strings.TrimSpace(resp.Error) != "" {
			return fmt.Errorf("FATAL: delete operation returned error: %s", resp.Error)
		}
	}
	return nil
}

func validateListKeysExist(client protocol.PluginClient, cfg map[string]any, expectedKeys []string) error {
	listRaw, err := client.CallJSON("list", map[string]any{"config": cfg})
	if err != nil {
		return fmt.Errorf("FATAL: list validation failed - list call error: %w", err)
	}

	keys := parseSecretsList(listRaw)
	if keys == nil {
		return fmt.Errorf("FATAL: list validation failed - could not parse list response")
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("FATAL: list returned key with empty string")
		}
		keySet[strings.TrimSpace(k)] = true
	}

	for _, expected := range expectedKeys {
		if !keySet[expected] {
			return fmt.Errorf("FATAL: list validation failed - key %q not found in list response", expected)
		}
	}

	return nil
}
