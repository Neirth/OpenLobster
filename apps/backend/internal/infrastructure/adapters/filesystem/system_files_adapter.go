// Copyright (c) OpenLobster contributors. See LICENSE for details.

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
)

// AllowedWorkspaceFiles are the workspace files exposed for read/write via the dashboard.
var AllowedWorkspaceFiles = []string{"AGENTS.md", "SOUL.md", "IDENTITY.md", "BOOTSTRAP.md", "MEMORY.md"}

// SystemFilesAdapter reads and writes workspace files (AGENTS.md, SOUL.md, IDENTITY.md).
type SystemFilesAdapter struct {
	workspacePath string
}

// NewSystemFilesAdapter returns a SystemFilesAdapter for the given workspace path.
func NewSystemFilesAdapter(workspacePath string) *SystemFilesAdapter {
	return &SystemFilesAdapter{workspacePath: workspacePath}
}

// ListFiles returns the allowed workspace files with their content and last modified time.
func (a *SystemFilesAdapter) ListFiles() ([]dto.SystemFileSnapshot, error) {
	var out []dto.SystemFileSnapshot
	for _, name := range AllowedWorkspaceFiles {
		fp := filepath.Join(a.workspacePath, name)
		content, err := os.ReadFile(fp)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, dto.SystemFileSnapshot{
					Name:         name,
					Path:         fp,
					Content:      "",
					LastModified: "",
				})
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", fp, err)
		}
		mod := ""
		if fi, err := os.Stat(fp); err == nil {
			mod = fi.ModTime().Format(time.RFC3339)
		}
		out = append(out, dto.SystemFileSnapshot{
			Name:         name,
			Path:         fp,
			Content:      string(content),
			LastModified: mod,
		})
	}
	return out, nil
}

// WriteFile writes content to an allowed workspace file. Only AllowedWorkspaceFiles are accepted.
func (a *SystemFilesAdapter) WriteFile(name, content string) error {
	allowed := false
	for _, n := range AllowedWorkspaceFiles {
		if n == name {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("workspace file %q is not writable (allowed: %v)", name, AllowedWorkspaceFiles)
	}
	fp := filepath.Join(a.workspacePath, name)
	return os.WriteFile(fp, []byte(content), 0644)
}
