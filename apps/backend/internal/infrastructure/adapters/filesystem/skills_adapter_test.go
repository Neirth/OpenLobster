package filesystem

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSkillsAdapter(t *testing.T) {
	a := NewSkillsAdapter("/tmp/workspace")
	require.NotNil(t, a)
}

func TestSkillsAdapter_ListSkills_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755)

	a := NewSkillsAdapter(tmpDir)
	skills, err := a.ListSkills()
	require.NoError(t, err)
	assert.Empty(t, skills)
}

func TestSkillsAdapter_ListSkills_NoSkillsDir(t *testing.T) {
	tmpDir := t.TempDir()

	a := NewSkillsAdapter(tmpDir)
	skills, err := a.ListSkills()
	require.NoError(t, err)
	assert.Empty(t, skills)
}

func TestSkillsAdapter_ListSkills_WithSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
description: A test skill for unit tests
---

# Test Skill

Some content.
`), 0644))

	a := NewSkillsAdapter(tmpDir)
	skills, err := a.ListSkills()
	require.NoError(t, err)
	require.Len(t, skills, 1)
	assert.Equal(t, "test-skill", skills[0].Name)
	assert.Equal(t, "A test skill for unit tests", skills[0].Description)
	assert.True(t, skills[0].Enabled)
	assert.Contains(t, skills[0].Path, "test-skill")
}

func TestSkillsAdapter_ListSkills_FrontmatterFallback(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "no-frontmatter")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`# My Skill

First line of body text.
`), 0644))

	a := NewSkillsAdapter(tmpDir)
	skills, err := a.ListSkills()
	require.NoError(t, err)
	require.Len(t, skills, 1)
	assert.Equal(t, "First line of body text.", skills[0].Description)
}

func TestSkillsAdapter_EnableDisableSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "toggle-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("description: Toggle test"), 0644))

	a := NewSkillsAdapter(tmpDir)

	skills, _ := a.ListSkills()
	require.Len(t, skills, 1)
	assert.True(t, skills[0].Enabled)

	require.NoError(t, a.DisableSkill("toggle-skill"))
	skills, _ = a.ListSkills()
	assert.False(t, skills[0].Enabled)

	require.NoError(t, a.EnableSkill("toggle-skill"))
	skills, _ = a.ListSkills()
	assert.True(t, skills[0].Enabled)
}

func TestSkillsAdapter_ListEnabledSkills(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "enabled-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0644))

	a := NewSkillsAdapter(tmpDir)
	enabled, err := a.ListEnabledSkills()
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	assert.Equal(t, "enabled-skill", enabled[0].Name)

	a.DisableSkill("enabled-skill")
	enabled, _ = a.ListEnabledSkills()
	assert.Empty(t, enabled)
}

func TestSkillsAdapter_DeleteSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "to-delete")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0644))

	a := NewSkillsAdapter(tmpDir)
	require.NoError(t, a.DeleteSkill("to-delete"))
	_, err := os.Stat(skillDir)
	assert.True(t, os.IsNotExist(err))
}

func TestSkillsAdapter_DeleteSkill_InvalidName(t *testing.T) {
	a := NewSkillsAdapter(t.TempDir())
	assert.Error(t, a.DeleteSkill(""))
	assert.Error(t, a.DeleteSkill("a/b"))
	assert.Error(t, a.DeleteSkill("a..b"))
}

func TestSkillsAdapter_DeleteSkill_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755)
	a := NewSkillsAdapter(tmpDir)
	err := a.DeleteSkill("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSkillsAdapter_LoadSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	content := "# My Skill\n\nFull content here."
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644))

	a := NewSkillsAdapter(tmpDir)
	loaded, err := a.LoadSkill("my-skill")
	require.NoError(t, err)
	assert.Equal(t, content, loaded)
}

func TestSkillsAdapter_LoadSkill_InvalidName(t *testing.T) {
	a := NewSkillsAdapter(t.TempDir())
	_, err := a.LoadSkill("")
	assert.Error(t, err)
	_, err = a.LoadSkill("a/b")
	assert.Error(t, err)
}

func TestSkillsAdapter_LoadSkill_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755)
	a := NewSkillsAdapter(tmpDir)
	_, err := a.LoadSkill("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSkillsAdapter_LoadSkill_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "disabled-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0644))

	a := NewSkillsAdapter(tmpDir)
	require.NoError(t, a.DisableSkill("disabled-skill"))
	_, err := a.LoadSkill("disabled-skill")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestSkillsAdapter_ReadSkillFile(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "helper.txt"), []byte("helper content"), 0644))

	a := NewSkillsAdapter(tmpDir)
	content, err := a.ReadSkillFile("my-skill", "helper.txt")
	require.NoError(t, err)
	assert.Equal(t, "helper content", content)
}

func TestSkillsAdapter_ReadSkillFile_InvalidName(t *testing.T) {
	a := NewSkillsAdapter(t.TempDir())
	_, err := a.ReadSkillFile("", "file.txt")
	assert.Error(t, err)
	_, err = a.ReadSkillFile("a/b", "file.txt")
	assert.Error(t, err)
}

func TestSkillsAdapter_ReadSkillFile_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "s")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	a := NewSkillsAdapter(tmpDir)

	_, err := a.ReadSkillFile("s", "../etc/passwd")
	assert.Error(t, err)
	_, err = a.ReadSkillFile("s", "/abs/path")
	assert.Error(t, err)
}

func TestSkillsAdapter_ReadSkillFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "s")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	a := NewSkillsAdapter(tmpDir)

	_, err := a.ReadSkillFile("s", "nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSkillsAdapter_ImportSkill_DirectoryLayout(t *testing.T) {
	tmpDir := t.TempDir()
	a := NewSkillsAdapter(tmpDir)

	// Create a minimal .skill zip with directory layout: my-skill/SKILL.md
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, _ := w.Create("my-skill/SKILL.md")
	fw.Write([]byte("description: Imported skill\n"))
	w.Close()

	err := a.ImportSkill(buf.Bytes())
	require.NoError(t, err)

	skillPath := filepath.Join(tmpDir, "skills", "my-skill", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Imported skill")
}

func TestSkillsAdapter_ImportSkill_FlatLayout(t *testing.T) {
	tmpDir := t.TempDir()
	a := NewSkillsAdapter(tmpDir)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, _ := w.Create("SKILL.md")
	fw.Write([]byte("---\nname: flat-skill\ndescription: Flat layout\n---\n"))
	w.Close()

	err := a.ImportSkill(buf.Bytes())
	require.NoError(t, err)

	skillPath := filepath.Join(tmpDir, "skills", "flat-skill", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Flat layout")
}

func TestSkillsAdapter_ImportSkill_InvalidArchive(t *testing.T) {
	a := NewSkillsAdapter(t.TempDir())
	err := a.ImportSkill([]byte("not a zip"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestSkillsAdapter_ImportSkill_InvalidLayout(t *testing.T) {
	tmpDir := t.TempDir()
	a := NewSkillsAdapter(tmpDir)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	w.Create("a/file.txt")
	w.Create("b/file.txt")
	w.Close()

	err := a.ImportSkill(buf.Bytes())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}
