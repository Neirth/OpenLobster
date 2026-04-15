// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/neirth/openlobster/utils/validation_layer/src/smoke"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

func main() {
	os.Exit(run())
}

func run() int {
	smokeConfigArg := flag.String("config", "", "JSON object (or @file.json) passed as config to smoke tests")
	recipient := flag.String("recipient", "", "Optional recipient/channel ID for messaging smoke tests")
	expect := flag.String("expect", "", "Optional expected content of inbound message (default: OK)")
	jsonOutput := flag.Bool("json", false, "Emit report as JSON")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: validator [--config <json|@file>] [--recipient <id>] [--expect <text>] [--json] <binary-path>\n")
		return 2
	}
	binaryPath := args[0]

	smokeConfig, err := parseConfigArg(*smokeConfigArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 2
	}

	// Use modular validator from the smoke package
	report, err := smoke.ValidatePluginBinary(binaryPath, types.ValidateOptions{
		SmokeConfig:            smokeConfig,
		SmokeTestRecipient:    *recipient,
		ExpectedInboundContent: *expect,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "validator error: %v\n", err)
		return 2
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
			return 2
		}
	} else {
		printTextReport(report)
	}

	if report.ErrorCount() > 0 {
		return 1
	}
	return 0
}

func parseConfigArg(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "@") {
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
		if path == "" {
			return nil, fmt.Errorf("config file path is empty")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config file %q: %w", path, err)
		}
		trimmed = strings.TrimSpace(string(data))
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON config: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("config must be a JSON object")
	}
	return cfg, nil
}

func printTextReport(report types.PluginReport) {
	status := "OK"
	if report.ErrorCount() > 0 {
		status = "ERROR"
	}

	typeLabel := report.Type
	if strings.TrimSpace(typeLabel) == "" {
		typeLabel = "unknown"
	}

	fmt.Printf("Plugin: %s  Binary: %s  Type: %s\n", report.Name, report.Binary, typeLabel)
	if report.ID != "" {
		fmt.Printf("ID: %s  Version: %s\n", report.ID, report.Version)
	}
	if len(report.Exports) > 0 {
		fmt.Printf("Exports (%d): %s\n", len(report.Exports), strings.Join(report.Exports, ", "))
	}
	fmt.Printf("Status: %s\n", status)
	for _, issue := range report.Issues {
		path := issue.File
		if strings.TrimSpace(path) == "" {
			path = "(n/a)"
		}
		fmt.Printf("  - %s [%s] %s (%s)\n", strings.ToUpper(string(issue.Severity)), issue.Rule, issue.Message, path)
	}
}

func configString(cfg map[string]any, key string) string {
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
