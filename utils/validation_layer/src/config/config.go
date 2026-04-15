// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package config provides helpers for working with plugin smoke-test config maps.
package config

import (
	"fmt"
	"os"
	"strings"
)

// EnsureConfigValue sets cfg[key] = fallback if the key is absent or blank.
func EnsureConfigValue(cfg map[string]any, key string, fallback any) {
	if cfg == nil {
		return
	}
	if value, ok := cfg[key]; ok {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return
			}
		default:
			if typed != nil {
				return
			}
		}
	}
	cfg[key] = fallback
}

// FillMissingConfigFromEnv reads environment variables and populates cfg
// for any key whose current value is blank. envByKey maps config-key → env-var names.
func FillMissingConfigFromEnv(cfg map[string]any, envByKey map[string][]string) {
	if cfg == nil {
		return
	}
	for key, envKeys := range envByKey {
		if ConfigString(cfg, key) != "" {
			continue
		}
		for _, envKey := range envKeys {
			value := strings.TrimSpace(os.Getenv(envKey))
			if value == "" {
				continue
			}
			cfg[key] = value
			break
		}
	}
}

// ConfigString returns the string value of cfg[key], trimmed, or "" if missing.
func ConfigString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	value, ok := cfg[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

// FallbackString returns primary if non-blank, otherwise fallback.
func FallbackString(primary, fallback string) string {
	if trimmed := strings.TrimSpace(primary); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}
