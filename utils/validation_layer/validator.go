// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

const (
	metadataExportName = "get_metadata"
)

var baseRequiredExports = []string{
	metadataExportName,
	"configure",
}

var forbiddenMetadataExports = []string{
	"getManifest",
	"metadata",
	"get_name",
	"get_version",
	"get_description",
	"get_type",
	"get_schema",
	"supports_audio_input",
	"supports_audio_output",
}

type Issue struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
}

type PluginReport struct {
	Name    string   `json:"name"`
	ID      string   `json:"id,omitempty"`
	Type    string   `json:"type"`
	Exports []string `json:"exports,omitempty"`
	Issues  []Issue  `json:"issues,omitempty"`
}

type Report struct {
	PluginsDir string         `json:"pluginsDir"`
	Plugins    []PluginReport `json:"plugins"`
}

func (r Report) ErrorCount() int {
	total := 0
	for _, plugin := range r.Plugins {
		total += plugin.ErrorCount()
	}
	return total
}

func (r Report) WarningCount() int {
	total := 0
	for _, plugin := range r.Plugins {
		total += plugin.WarningCount()
	}
	return total
}

func (p PluginReport) ErrorCount() int {
	total := 0
	for _, issue := range p.Issues {
		if issue.Severity == SeverityError {
			total++
		}
	}
	return total
}

func (p PluginReport) WarningCount() int {
	total := 0
	for _, issue := range p.Issues {
		if issue.Severity == SeverityWarning {
			total++
		}
	}
	return total
}

func ValidatePlugins(pluginsDir string, filter string) (Report, error) {
	dirs, err := resolvePluginTargets(pluginsDir, filter)
	if err != nil {
		return Report{}, err
	}

	report := Report{PluginsDir: pluginsDir, Plugins: make([]PluginReport, 0, len(dirs))}
	for _, dir := range dirs {
		pluginReport, err := ValidatePluginDir(dir)
		if err != nil {
			pluginReport.addIssue(SeverityError, "plugin-validation", err.Error(), "")
		}
		report.Plugins = append(report.Plugins, pluginReport)
	}

	sort.Slice(report.Plugins, func(i, j int) bool {
		return report.Plugins[i].Name < report.Plugins[j].Name
	})

	return report, nil
}

func resolvePluginTargets(pluginsDir string, selector string) ([]string, error) {
	dirs, err := discoverPluginDirs(pluginsDir)
	if err != nil {
		return nil, err
	}

	selector = strings.TrimSpace(selector)
	if selector == "" {
		return dirs, nil
	}

	if path, ok := resolvePluginSelectorPath(pluginsDir, selector); ok {
		if _, typeOK := pluginTypeFromDir(filepath.Base(path)); !typeOK {
			return nil, fmt.Errorf("plugin path %q is not a supported OpenLobster plugin directory", selector)
		}
		return []string{path}, nil
	}

	needle := strings.ToLower(selector)
	matches := make([]string, 0)
	for _, dir := range dirs {
		name := strings.ToLower(filepath.Base(dir))
		if strings.Contains(name, needle) {
			matches = append(matches, dir)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no plugins matched selector %q", selector)
	}

	sort.Strings(matches)
	return matches, nil
}

func resolvePluginSelectorPath(pluginsDir string, selector string) (string, bool) {
	if strings.TrimSpace(selector) == "" {
		return "", false
	}

	candidates := []string{selector}
	if !filepath.IsAbs(selector) {
		candidates = append(candidates, filepath.Join(pluginsDir, selector))
		candidates = append(candidates, filepath.Join(pluginsDir, filepath.Base(selector)))
	}

	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		info, err := os.Stat(clean)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return clean, true
		}
		return filepath.Dir(clean), true
	}

	return "", false
}

func ValidatePluginDir(dir string) (PluginReport, error) {
	name := filepath.Base(dir)
	pluginType, ok := pluginTypeFromDir(name)
	if !ok {
		return PluginReport{Name: name}, fmt.Errorf("unsupported plugin directory naming: %s", name)
	}

	report := PluginReport{Name: name, Type: pluginType}
	files, err := listGoSourceFiles(dir)
	if err != nil {
		return report, fmt.Errorf("list go files: %w", err)
	}
	if len(files) == 0 {
		report.addIssue(SeverityError, "source-files", "no Go source files found", dir)
		return report, nil
	}

	parsed := make(map[string]*ast.File, len(files))
	funcDecls := make(map[string]*ast.FuncDecl)
	funcFiles := make(map[string]string)

	for _, path := range files {
		fileSet := token.NewFileSet()
		fileNode, parseErr := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
		if parseErr != nil {
			report.addIssue(SeverityError, "parse-go", fmt.Sprintf("failed to parse file: %v", parseErr), path)
			continue
		}
		parsed[path] = fileNode

		for _, decl := range fileNode.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
				continue
			}
			funcDecls[fn.Name.Name] = fn
			funcFiles[fn.Name.Name] = path
		}
	}

	def, found := findPluginDefinition(parsed)
	if !found {
		report.addIssue(SeverityError, "exports-map", "no pdk.MustRun(pdk.Plugin{...}) definition found", "")
		return report, nil
	}

	report.ID = def.ID
	report.Exports = sortedKeys(def.Exports)

	if strings.TrimSpace(def.ID) == "" {
		report.addIssue(SeverityError, "plugin-id", "plugin ID is empty", def.File)
	} else if def.ID != name {
		report.addIssue(SeverityError, "plugin-id", fmt.Sprintf("plugin ID %q does not match directory name %q", def.ID, name), def.File)
	}

	for _, exp := range baseRequiredExports {
		if _, exists := def.Exports[exp]; !exists {
			report.addIssue(SeverityError, "missing-export", fmt.Sprintf("required export %q is missing", exp), def.File)
		}
	}
	validateMetadataExports(&report, funcDecls, funcFiles, def)

	return report, nil
}

func (p *PluginReport) addIssue(severity Severity, rule string, message string, file string) {
	p.Issues = append(p.Issues, Issue{
		Severity: severity,
		Rule:     rule,
		Message:  message,
		File:     file,
	})
}

type pluginDefinition struct {
	ID      string
	Exports map[string]string
	File    string
}

func findPluginDefinition(files map[string]*ast.File) (pluginDefinition, bool) {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	best := pluginDefinition{}
	found := false

	for _, path := range paths {
		candidate, ok := extractPluginDefinition(files[path], path)
		if !ok {
			continue
		}
		if !found || len(candidate.Exports) > len(best.Exports) {
			best = candidate
			found = true
		}
	}

	return best, found
}

func extractPluginDefinition(fileNode *ast.File, path string) (pluginDefinition, bool) {
	result := pluginDefinition{}
	found := false

	ast.Inspect(fileNode, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "MustRun" {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}

		pluginLit, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			return true
		}

		def := pluginDefinition{ID: "", Exports: map[string]string{}, File: path}
		for _, elt := range pluginLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key := exprKeyName(kv.Key)
			switch key {
			case "ID":
				if id, ok := extractStringLiteral(kv.Value); ok {
					def.ID = id
				}
			case "Exports":
				def.Exports = extractExportsMap(kv.Value)
			}
		}

		if def.ID != "" || len(def.Exports) > 0 {
			result = def
			found = true
			return false
		}
		return true
	})

	return result, found
}

func exprKeyName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, ok := extractStringLiteral(v); ok {
				return s
			}
		}
	}
	return ""
}

func extractExportsMap(expr ast.Expr) map[string]string {
	out := map[string]string{}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return out
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		exportName, ok := extractStringLiteral(kv.Key)
		if !ok || exportName == "" {
			continue
		}
		funcName := ""
		switch v := kv.Value.(type) {
		case *ast.Ident:
			funcName = v.Name
		case *ast.SelectorExpr:
			if v.Sel != nil {
				funcName = v.Sel.Name
			}
		}
		out[exportName] = funcName
	}

	return out
}

func extractStringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	unquoted, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return unquoted, true
}

func extractOutputStringLiteral(fn *ast.FuncDecl) (string, bool) {
	if fn == nil || fn.Body == nil {
		return "", false
	}

	var value string
	found := false

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "OutputString" {
			return true
		}
		if len(call.Args) != 1 {
			return true
		}
		if literal, ok := extractStringLiteral(call.Args[0]); ok {
			value = literal
			found = true
			return false
		}
		return true
	})

	return value, found
}

func hasExport(exports map[string]string, key string) bool {
	_, ok := exports[key]
	return ok
}

func fileForFunc(funcName string, funcFiles map[string]string, fallback string) string {
	if path, ok := funcFiles[funcName]; ok {
		return path
	}
	return fallback
}

func validateMetadataExports(report *PluginReport, funcs map[string]*ast.FuncDecl, funcFiles map[string]string, def pluginDefinition) {
	metadataFnName, ok := def.Exports[metadataExportName]
	if !ok {
		report.addIssue(SeverityError, "metadata-export", fmt.Sprintf("required export %q is missing", metadataExportName), def.File)
		return
	}

	legacy := make([]string, 0)
	for _, exp := range forbiddenMetadataExports {
		if hasExport(def.Exports, exp) {
			legacy = append(legacy, exp)
		}
	}
	if len(legacy) > 0 {
		sort.Strings(legacy)
		report.addIssue(
			SeverityError,
			"metadata-export",
			fmt.Sprintf("legacy metadata exports are forbidden when %q is used: %s", metadataExportName, strings.Join(legacy, ", ")),
			def.File,
		)
	}

	fn := funcs[metadataFnName]
	if fn == nil {
		return
	}

	rawMetadata, ok := extractOutputStringLiteral(fn)
	if !ok {
		return
	}

	if !json.Valid([]byte(rawMetadata)) {
		report.addIssue(SeverityError, "metadata-json", fmt.Sprintf("%s does not return valid JSON", metadataExportName), fileForFunc(metadataFnName, funcFiles, def.File))
		return
	}

	var metadata struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Version     string          `json:"version"`
		Description string          `json:"description"`
		Type        string          `json:"type"`
		Schema      json.RawMessage `json:"schema"`
		Properties  json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(rawMetadata), &metadata); err != nil {
		report.addIssue(SeverityError, "metadata-json", fmt.Sprintf("failed to decode metadata JSON: %v", err), fileForFunc(metadataFnName, funcFiles, def.File))
		return
	}

	metadata.ID = strings.TrimSpace(metadata.ID)
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Version = strings.TrimSpace(metadata.Version)
	metadata.Description = strings.TrimSpace(metadata.Description)
	metadata.Type = strings.TrimSpace(metadata.Type)

	if metadata.ID == "" {
		report.addIssue(SeverityError, "metadata-json", "metadata.id is required", fileForFunc(metadataFnName, funcFiles, def.File))
	}
	if metadata.ID != "" && strings.TrimSpace(report.ID) != "" && metadata.ID != strings.TrimSpace(report.ID) {
		report.addIssue(SeverityError, "metadata-json", fmt.Sprintf("metadata.id %q does not match plugin ID %q", metadata.ID, report.ID), fileForFunc(metadataFnName, funcFiles, def.File))
	}
	if metadata.Name == "" {
		report.addIssue(SeverityError, "metadata-json", "metadata.name is required", fileForFunc(metadataFnName, funcFiles, def.File))
	}
	if metadata.Version == "" {
		report.addIssue(SeverityError, "metadata-json", "metadata.version is required", fileForFunc(metadataFnName, funcFiles, def.File))
	}
	if metadata.Description == "" {
		report.addIssue(SeverityError, "metadata-json", "metadata.description is required", fileForFunc(metadataFnName, funcFiles, def.File))
	}
	if metadata.Type == "" {
		report.addIssue(SeverityError, "metadata-json", "metadata.type is required", fileForFunc(metadataFnName, funcFiles, def.File))
	} else if metadata.Type != strings.TrimSpace(report.Type) {
		report.addIssue(SeverityError, "metadata-json", fmt.Sprintf("metadata.type %q does not match plugin directory type %q", metadata.Type, report.Type), fileForFunc(metadataFnName, funcFiles, def.File))
	}

	if !json.Valid(metadata.Schema) {
		report.addIssue(SeverityError, "metadata-json", "metadata.schema must be valid JSON", fileForFunc(metadataFnName, funcFiles, def.File))
	} else {
		var schemaObj map[string]any
		if err := json.Unmarshal(metadata.Schema, &schemaObj); err != nil {
			report.addIssue(SeverityError, "metadata-json", "metadata.schema root must be an object", fileForFunc(metadataFnName, funcFiles, def.File))
		} else if schemaType, ok := schemaObj["type"].(string); ok {
			schemaType = strings.TrimSpace(schemaType)
			if schemaType == "" || schemaType != "object" {
				report.addIssue(SeverityError, "metadata-json", "metadata.schema.type must be object", fileForFunc(metadataFnName, funcFiles, def.File))
			}
		}
	}

	if !json.Valid(metadata.Properties) {
		report.addIssue(SeverityError, "metadata-json", "metadata.properties must be valid JSON object", fileForFunc(metadataFnName, funcFiles, def.File))
		return
	}

	var propertiesObj map[string]any
	if err := json.Unmarshal(metadata.Properties, &propertiesObj); err != nil {
		report.addIssue(SeverityError, "metadata-json", "metadata.properties root must be an object", fileForFunc(metadataFnName, funcFiles, def.File))
	}
}

func discoverPluginDirs(pluginsDir string) ([]string, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("read plugins directory %q: %w", pluginsDir, err)
	}

	dirs := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := pluginTypeFromDir(name); !ok {
			continue
		}
		dirs = append(dirs, filepath.Join(pluginsDir, name))
	}

	sort.Strings(dirs)
	return dirs, nil
}

func listGoSourceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasSuffix(name, "_tinygo.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func pluginTypeFromDir(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, "openlobster-ai-"):
		return "ai", true
	case strings.HasPrefix(name, "openlobster-audio-"):
		return "audio", true
	case strings.HasPrefix(name, "openlobster-memory-"):
		return "memory", true
	case strings.HasPrefix(name, "openlobster-messages-"):
		return "messaging", true
	case strings.HasPrefix(name, "openlobster-secrets-"):
		return "secrets", true
	default:
		return "", false
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
