// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run())
}

func run() int {
	pluginsDir := flag.String("plugins-dir", "./plugins", "Path to plugins directory")
	pluginSelector := flag.String("plugin", "", "Plugin selector: name, substring, or plugin directory path")
	smokeConfigArg := flag.String("config", "", "JSON object (or @file.json) passed as config to smoke tests")
	jsonOutput := flag.Bool("json", false, "Emit report as JSON")
	flag.Parse()

	smokeConfig, err := parseConfigArg(*smokeConfigArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 2
	}

	report, err := ValidatePluginsWithOptions(*pluginsDir, *pluginSelector, ValidateOptions{
		SmokeConfig: smokeConfig,
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

func printTextReport(report Report) {
	fmt.Printf("Plugin validation report\n")
	fmt.Printf("  Plugins dir: %s\n", report.PluginsDir)
	fmt.Printf("  Plugins scanned: %d\n", len(report.Plugins))
	fmt.Printf("  Errors: %d\n", report.ErrorCount())
	fmt.Printf("  Smoke failures: %d\n", smokeIssueSummary(report))
	fmt.Printf("\n")

	for _, plugin := range report.Plugins {
		status := "OK"
		if plugin.ErrorCount() > 0 {
			status = "ERROR"
		} else if plugin.WarningCount() > 0 {
			status = "WARN"
		}

		typeLabel := plugin.Type
		if strings.TrimSpace(typeLabel) == "" {
			typeLabel = "unknown"
		}

		fmt.Printf("[%s] %s (%s)\n", status, plugin.Name, typeLabel)
		fmt.Printf("  ID: %s\n", plugin.ID)
		if len(plugin.Exports) > 0 {
			fmt.Printf("  Exports (%d): %s\n", len(plugin.Exports), strings.Join(plugin.Exports, ", "))
		}
		for _, issue := range plugin.Issues {
			path := issue.File
			if strings.TrimSpace(path) == "" {
				path = "(n/a)"
			}
			fmt.Printf("  - %s [%s] %s (%s)\n", strings.ToUpper(string(issue.Severity)), issue.Rule, issue.Message, path)
		}
		fmt.Println()
	}
}
