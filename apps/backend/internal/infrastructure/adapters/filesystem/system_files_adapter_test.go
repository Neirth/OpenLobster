// Copyright (c) OpenLobster contributors. See LICENSE for details.

package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemFilesAdapter_ListFiles(t *testing.T) {
	dir := t.TempDir()
	adapter := NewSystemFilesAdapter(dir)

	files, err := adapter.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != len(AllowedWorkspaceFiles) {
		t.Errorf("expected %d files, got %d", len(AllowedWorkspaceFiles), len(files))
	}
	for _, f := range files {
		if f.Name == "" || f.Path == "" {
			t.Errorf("file missing name or path: %+v", f)
		}
	}
}

func TestSystemFilesAdapter_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	adapter := NewSystemFilesAdapter(dir)

	err := adapter.WriteFile("AGENTS.md", "# Test content")
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	files, err := adapter.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	var found bool
	for _, f := range files {
		if f.Name == "AGENTS.md" {
			found = true
			if f.Content != "# Test content" {
				t.Errorf("content mismatch: got %q", f.Content)
			}
			break
		}
	}
	if !found {
		t.Error("AGENTS.md not found in ListFiles")
	}

	// Verify on disk
	raw, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile from disk: %v", err)
	}
	if string(raw) != "# Test content" {
		t.Errorf("disk content mismatch: got %q", string(raw))
	}
}

func TestSystemFilesAdapter_WriteFile_RejectsUnknown(t *testing.T) {
	dir := t.TempDir()
	adapter := NewSystemFilesAdapter(dir)

	err := adapter.WriteFile("evil.md", "x")
	if err == nil {
		t.Error("expected error when writing non-allowed file")
	}
}
