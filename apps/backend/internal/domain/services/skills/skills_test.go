package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)
	assert.Empty(t, r.skills)
}

func TestRegisterSkill(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterSkill(Skill{
		Name:        "test_skill",
		Description: "A test skill",
		Enabled:     true,
	})
	assert.NoError(t, err)
	assert.Len(t, r.skills, 1)
}

func TestRegisterSkill_NoName(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterSkill(Skill{
		Description: "A test skill",
	})
	assert.Error(t, err)
}

func TestGetSkill(t *testing.T) {
	r := NewRegistry()
	r.RegisterSkill(Skill{Name: "test", Enabled: true})

	skill, ok := r.GetSkill("test")
	assert.True(t, ok)
	assert.Equal(t, "test", skill.Name)
}

func TestGetSkill_NotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.GetSkill("nonexistent")
	assert.False(t, ok)
}

func TestListSkills(t *testing.T) {
	r := NewRegistry()
	r.RegisterSkill(Skill{Name: "skill1", Enabled: true})
	r.RegisterSkill(Skill{Name: "skill2", Enabled: true})

	skills := r.ListSkills()
	assert.Len(t, skills, 2)
}

func TestRegisterHandler(t *testing.T) {
	r := NewRegistry()
	handler := &InlineSkillHandler{}
	r.RegisterHandler("inline", handler)

	assert.NotNil(t, r.handlers["inline"])
}

func TestExecute_SkillNotFound(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "nonexistent", nil, SkillContext{})
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestExecute_SkillDisabled(t *testing.T) {
	r := NewRegistry()
	r.RegisterSkill(Skill{Name: "disabled", Enabled: false})

	result, err := r.Execute(context.Background(), "disabled", nil, SkillContext{})
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestExecute_NoHandler(t *testing.T) {
	r := NewRegistry()
	r.RegisterSkill(Skill{Name: "test", Handler: "nonexistent_handler", Enabled: true})

	result, err := r.Execute(context.Background(), "test", nil, SkillContext{})
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestExecute_Success(t *testing.T) {
	r := NewRegistry()
	r.RegisterHandler("inline", &InlineSkillHandler{
		ExecuteFunc: func(ctx context.Context, params map[string]interface{}, context SkillContext) (SkillResult, error) {
			return SkillResult{Success: true, Output: "executed"}, nil
		},
	})
	r.RegisterSkill(Skill{Name: "test", Handler: "inline", Enabled: true})

	result, err := r.Execute(context.Background(), "test", nil, SkillContext{})
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "executed", result.Output)
}

func TestLoadFromFile(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()
	manifestFile := filepath.Join(tmpDir, "claude.json")
	manifest := `{
		"version": "1.0",
		"skills": [
			{"name": "file_read", "description": "Read a file", "enabled": true},
			{"name": "file_write", "description": "Write a file", "enabled": true}
		]
	}`
	err := os.WriteFile(manifestFile, []byte(manifest), 0644)
	require.NoError(t, err)

	err = r.LoadFromFile(manifestFile)
	assert.NoError(t, err)
	assert.Len(t, r.skills, 2)
}

func TestLoadFromFile_NotFound(t *testing.T) {
	r := NewRegistry()
	err := r.LoadFromFile("/nonexistent/file.json")
	assert.Error(t, err)
}

func TestLoadFromDirectory(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()

	manifest1 := `{"version": "1.0", "skills": [{"name": "skill1", "enabled": true}]}`
	os.WriteFile(filepath.Join(tmpDir, "skills1.json"), []byte(manifest1), 0644)

	manifest2 := `{"version": "1.0", "skills": [{"name": "skill2", "enabled": true}]}`
	os.WriteFile(filepath.Join(tmpDir, "skills2.json"), []byte(manifest2), 0644)

	err := r.LoadFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, r.skills, 2)
}

func TestCreateSkillManifest(t *testing.T) {
	skills := []Skill{
		{Name: "test1"},
		{Name: "test2"},
	}
	manifest := CreateSkillManifest(skills)

	assert.Equal(t, "1.0", manifest.Version)
	assert.Len(t, manifest.Skills, 2)
	assert.True(t, manifest.Settings.AllowInlineSkills)
}

func TestGenerateSkillID(t *testing.T) {
	id1 := GenerateSkillID()
	id2 := GenerateSkillID()

	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2)
}

func TestCodeExecutionHandler(t *testing.T) {
	handler := NewCodeExecutionHandler(nil)

	result, err := handler.Execute(context.Background(), Skill{}, map[string]interface{}{
		"code": "console.log('hello')",
	}, SkillContext{})

	assert.NoError(t, err)
	assert.True(t, result.Success)
}

func TestCodeExecutionHandler_NoCode(t *testing.T) {
	handler := NewCodeExecutionHandler(nil)

	result, err := handler.Execute(context.Background(), Skill{}, map[string]interface{}{}, SkillContext{})

	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "code parameter required", result.Error)
}
