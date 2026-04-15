// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package types defines the shared data model for plugin validation reports.
package types

import "encoding/json"

// SmokeFailRule is the rule name used for all runtime smoke test failures.
const SmokeFailRule = "smoke-fail"

// Severity classifies the impact of a validation issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue represents a single validation finding attached to a plugin report.
type Issue struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
}

// PluginReport is the result of validating a single plugin binary.
type PluginReport struct {
	Binary  string   `json:"binary"`
	Name    string   `json:"name"`
	ID      string   `json:"id,omitempty"`
	Type    string   `json:"type"`
	Version string   `json:"version,omitempty"`
	Exports []string `json:"exports,omitempty"`
	Issues  []Issue  `json:"issues,omitempty"`
}

// ErrorCount returns the number of error-severity issues in the report.
func (p PluginReport) ErrorCount() int {
	total := 0
	for _, issue := range p.Issues {
		if issue.Severity == SeverityError {
			total++
		}
	}
	return total
}

// WarningCount returns the number of warning-severity issues in the report.
func (p PluginReport) WarningCount() int {
	total := 0
	for _, issue := range p.Issues {
		if issue.Severity == SeverityWarning {
			total++
		}
	}
	return total
}

// SmokeFailureCount returns the number of smoke-test failures in the report.
func (p PluginReport) SmokeFailureCount() int {
	total := 0
	for _, issue := range p.Issues {
		if issue.Rule == SmokeFailRule {
			total++
		}
	}
	return total
}

// AddIssue appends a new issue to the report.
func (p *PluginReport) AddIssue(severity Severity, rule, message, file string) {
	p.Issues = append(p.Issues, Issue{
		Severity: severity,
		Rule:     rule,
		Message:  message,
		File:     file,
	})
}

// ValidateOptions controls runtime behaviour of the smoke validation.
type ValidateOptions struct {
	SmokeConfig            map[string]any
	SmokeTestRecipient    string
	ExpectedInboundContent string
}

// ReceivedNotification records a fire-and-forget JSON-RPC message from the plugin.
type ReceivedNotification struct {
	Method string
	Params json.RawMessage
}
