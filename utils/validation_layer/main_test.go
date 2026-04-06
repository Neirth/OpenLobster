// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigArgJSON(t *testing.T) {
	cfg, err := parseConfigArg(`{"api_key":"abc","timeout_ms":5000}`)
	if err != nil {
		t.Fatalf("parseConfigArg() error = %v", err)
	}
	if got := configString(cfg, "api_key"); got != "abc" {
		t.Fatalf("api_key = %q, want %q", got, "abc")
	}
	if got := configString(cfg, "timeout_ms"); got != "5000" {
		t.Fatalf("timeout_ms = %q, want %q", got, "5000")
	}
}

func TestParseConfigArgFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "smoke.json")
	if err := os.WriteFile(path, []byte(`{"base_url":"http://localhost:11434"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := parseConfigArg("@" + path)
	if err != nil {
		t.Fatalf("parseConfigArg(@file) error = %v", err)
	}
	if got := configString(cfg, "base_url"); got != "http://localhost:11434" {
		t.Fatalf("base_url = %q, want %q", got, "http://localhost:11434")
	}
}

func TestParseConfigArgInvalid(t *testing.T) {
	if _, err := parseConfigArg("[1,2,3]"); err == nil {
		t.Fatalf("expected error for non-object JSON")
	}
	if _, err := parseConfigArg("{invalid"); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
