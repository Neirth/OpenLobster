// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package filesystem provides filesystem-backed infrastructure adapters.
package filesystem

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
)

// SkillsAdapter reads Anthropic-format skills from the workspace/skills directory.
// Each subdirectory under workspace/skills/ is treated as a skill, and its
// SKILL.md file provides the human-readable description.
//
// Enabled/disabled state is held in memory and resets on process restart.
type SkillsAdapter struct {
	workspacePath string
	mu            sync.RWMutex
	disabled      map[string]bool
}

// NewSkillsAdapter returns a SkillsAdapter that reads skills from
// workspacePath/skills/. All skills are enabled by default.
func NewSkillsAdapter(workspacePath string) *SkillsAdapter {
	return &SkillsAdapter{
		workspacePath: workspacePath,
		disabled:      make(map[string]bool),
	}
}

// ListSkills scans the workspace/skills directory and returns one SkillInfo
// per subdirectory. If the directory does not exist, an empty slice is returned
// without error.
func (a *SkillsAdapter) ListSkills() ([]dto.SkillSnapshot, error) {
	skillsDir := filepath.Join(a.workspacePath, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []dto.SkillSnapshot{}, nil
		}
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	var skills []dto.SkillSnapshot
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		description := a.readSkillDescription(filepath.Join(skillsDir, name))
		skills = append(skills, dto.SkillSnapshot{
			Name:        name,
			Description: description,
			Enabled:     !a.disabled[name],
			Path:        filepath.Join("skills", name),
		})
	}
	return skills, nil
}

// readSkillDescription opens SKILL.md inside skillPath and extracts the
// description value from the YAML frontmatter block (between the two "---"
// delimiters). Falls back to the first non-empty, non-heading body line when
// no frontmatter is present. Returns an empty string when the file is absent
// or unreadable.
func (a *SkillsAdapter) readSkillDescription(skillPath string) string {
	data, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		return ""
	}
	content := string(data)

	// Parse YAML frontmatter: content must start with "---\n".
	if strings.HasPrefix(content, "---") {
		rest := strings.TrimPrefix(content, "---")
		// Find the closing "---".
		end := strings.Index(rest, "\n---")
		if end != -1 {
			frontmatter := rest[:end]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					val := strings.TrimPrefix(line, "description:")
					val = strings.TrimSpace(val)
					// Strip surrounding quotes if present.
					val = strings.Trim(val, `"'`)
					if len(val) > 200 {
						return val[:200] + "..."
					}
					return val
				}
			}
		}
	}

	// Fallback: first non-empty, non-heading line in the body.
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || line == "---" {
			continue
		}
		if len(line) > 120 {
			return line[:120] + "..."
		}
		return line
	}
	return ""
}

// EnableSkill marks the named skill as enabled.
func (a *SkillsAdapter) EnableSkill(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.disabled, name)
	return nil
}

// DisableSkill marks the named skill as disabled.
func (a *SkillsAdapter) DisableSkill(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.disabled[name] = true
	return nil
}

// DeleteSkill removes the skill directory from workspace/skills/<name>.
func (a *SkillsAdapter) DeleteSkill(name string) error {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	skillPath := filepath.Join(a.workspacePath, "skills", name)
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found", name)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.disabled, name)
	return os.RemoveAll(skillPath)
}

// ImportSkill extracts a .skill archive (ZIP format) into the workspace/skills
// directory using only Go standard library packages (archive/zip).
//
// The archive may contain either:
//   - A single top-level directory (skill-name/SKILL.md, ...) — standard layout.
//   - Files at the root level with a SKILL.md present — flat layout.
//
// In the flat layout the skill name is read from the YAML frontmatter 'name'
// field inside SKILL.md. In the directory layout the directory name is used.
func (a *SkillsAdapter) ImportSkill(data []byte) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid skill archive: %w", err)
	}

	skillName, prefix, err := detectSkillLayout(r)
	if err != nil {
		return err
	}

	destDir := filepath.Join(a.workspacePath, "skills", skillName)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	for _, f := range r.File {
		// Strip the root prefix so paths are relative to the skill directory.
		relPath := strings.TrimPrefix(f.Name, prefix)
		if relPath == "" {
			continue
		}

		// Sanitise: reject any path that escapes the destination.
		cleaned := filepath.Clean(relPath)
		if strings.HasPrefix(cleaned, "..") {
			continue
		}

		destPath := filepath.Join(destDir, cleaned)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("cannot open %s in archive: %w", f.Name, err)
		}

		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("cannot create %s: %w", destPath, err)
		}

		_, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("cannot write %s: %w", destPath, copyErr)
		}
	}
	return nil
}

// detectSkillLayout inspects a zip archive and returns the skill name and the
// path prefix to strip when extracting files.
func detectSkillLayout(r *zip.Reader) (skillName, prefix string, err error) {
	topDirs := map[string]bool{}
	hasRootSKILL := false

	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) > 1 && parts[0] != "" {
			topDirs[parts[0]] = true
		}
		if f.Name == "SKILL.md" {
			hasRootSKILL = true
		}
	}

	// Flat layout: SKILL.md is at the zip root.
	if hasRootSKILL {
		for _, f := range r.File {
			if f.Name != "SKILL.md" {
				continue
			}
			rc, openErr := f.Open()
			if openErr != nil {
				return "", "", fmt.Errorf("cannot open SKILL.md: %w", openErr)
			}
			content, readErr := io.ReadAll(rc)
			rc.Close()
			if readErr != nil {
				return "", "", readErr
			}
			name := extractNameFromFrontmatter(string(content))
			if name == "" {
				return "", "", fmt.Errorf("SKILL.md is missing the 'name' field in its frontmatter")
			}
			return name, "", nil
		}
	}

	// Directory layout: single top-level directory.
	if len(topDirs) == 1 {
		for name := range topDirs {
			return name, name + "/", nil
		}
	}

	return "", "", fmt.Errorf("invalid skill archive: expected a single root directory or SKILL.md at the root")
}

// extractNameFromFrontmatter reads the 'name' field from YAML frontmatter
// delimited by '---' markers. Returns an empty string if not found.
func extractNameFromFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	rest := strings.TrimPrefix(content, "---")
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return ""
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), `"'`)
		}
	}
	return ""
}

// ListEnabledSkills returns a compact catalog of all enabled skills for the LLM.
func (a *SkillsAdapter) ListEnabledSkills() ([]mcp.SkillCatalogEntry, error) {
	skills, err := a.ListSkills()
	if err != nil {
		return nil, err
	}
	result := make([]mcp.SkillCatalogEntry, 0, len(skills))
	for _, s := range skills {
		if s.Enabled {
			result = append(result, mcp.SkillCatalogEntry{
				Name:        s.Name,
				Description: s.Description,
			})
		}
	}
	return result, nil
}

// LoadSkill returns the full SKILL.md content for the named skill.
func (a *SkillsAdapter) LoadSkill(name string) (string, error) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid skill name: %q", name)
	}
	a.mu.RLock()
	disabled := a.disabled[name]
	a.mu.RUnlock()
	if disabled {
		return "", fmt.Errorf("skill %q is disabled", name)
	}
	skillPath := filepath.Join(a.workspacePath, "skills", name, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("skill %q not found", name)
		}
		return "", err
	}
	return string(data), nil
}

// ReadSkillFile returns the content of a supporting file inside a skill directory.
// The filename is sanitized to prevent path traversal.
func (a *SkillsAdapter) ReadSkillFile(name, filename string) (string, error) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid skill name: %q", name)
	}
	cleaned := filepath.Clean(filename)
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid filename: %q", filename)
	}
	filePath := filepath.Join(a.workspacePath, "skills", name, cleaned)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file %q not found in skill %q", filename, name)
		}
		return "", err
	}
	return string(data), nil
}

// Ensure SkillsAdapter satisfies the dto.SkillsPort interface at compile time.
var _ dto.SkillsPort = (*SkillsAdapter)(nil)
