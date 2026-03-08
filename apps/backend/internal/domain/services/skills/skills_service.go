package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Skill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  SkillParameters `json:"parameters,omitempty"`
	Handler     string          `json:"handler,omitempty"`
	Enabled     bool            `json:"enabled,omitempty"`
}

type SkillParameters struct {
	Type       string            `json:"type,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Required   []string          `json:"required,omitempty"`
}

type SkillResult struct {
	Success bool        `json:"success"`
	Output  string      `json:"output,omitempty"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type SkillContext struct {
	UserID    string
	ChannelID string
	Variables map[string]string
}

type SkillHandler interface {
	Execute(ctx context.Context, skill Skill, params map[string]interface{}, context SkillContext) (SkillResult, error)
}

type Manifest struct {
	Version  string   `json:"version"`
	Skills   []Skill  `json:"skills"`
	Settings Settings `json:"settings,omitempty"`
}

type Settings struct {
	AllowInlineSkills bool `json:"allowInlineSkills,omitempty"`
	MaxConcurrent     int  `json:"maxConcurrent,omitempty"`
}

type Registry struct {
	skills   map[string]Skill
	handlers map[string]SkillHandler
	manifest *Manifest
}

func NewRegistry() *Registry {
	return &Registry{
		skills:   make(map[string]Skill),
		handlers: make(map[string]SkillHandler),
	}
}

func (r *Registry) RegisterHandler(name string, handler SkillHandler) {
	r.handlers[name] = handler
}

func (r *Registry) RegisterSkill(skill Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	r.skills[skill.Name] = skill
	return nil
}

func (r *Registry) GetSkill(name string) (Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

func (r *Registry) ListSkills() []Skill {
	skills := make([]Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

func (r *Registry) Execute(ctx context.Context, name string, params map[string]interface{}, skillCtx SkillContext) (SkillResult, error) {
	skill, ok := r.skills[name]
	if !ok {
		return SkillResult{Success: false, Error: "skill not found"}, fmt.Errorf("skill not found: %s", name)
	}

	if !skill.Enabled {
		return SkillResult{Success: false, Error: "skill is disabled"}, fmt.Errorf("skill is disabled: %s", name)
	}

	handler, ok := r.handlers[skill.Handler]
	if !ok {
		return SkillResult{Success: false, Error: "no handler registered"}, fmt.Errorf("no handler for skill: %s", name)
	}

	return handler.Execute(ctx, skill, params, skillCtx)
}

func (r *Registry) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}

	r.manifest = &manifest

	for _, skill := range manifest.Skills {
		if err := r.RegisterSkill(skill); err != nil {
			return err
		}
	}

	return nil
}

func (r *Registry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := r.LoadFromFile(path); err != nil {
			return fmt.Errorf("failed to load skill from %s: %w", path, err)
		}
	}

	return nil
}

func (r *Registry) Manifest() *Manifest {
	return r.manifest
}

type InlineSkillHandler struct {
	ExecuteFunc func(ctx context.Context, params map[string]interface{}, context SkillContext) (SkillResult, error)
}

func (h *InlineSkillHandler) Execute(ctx context.Context, skill Skill, params map[string]interface{}, context SkillContext) (SkillResult, error) {
	if h.ExecuteFunc != nil {
		return h.ExecuteFunc(ctx, params, context)
	}
	return SkillResult{Success: false, Error: "no execution function defined"}, nil
}

type CodeExecutionHandler struct {
	allowedDirs []string
}

func NewCodeExecutionHandler(allowedDirs []string) *CodeExecutionHandler {
	return &CodeExecutionHandler{
		allowedDirs: allowedDirs,
	}
}

func (h *CodeExecutionHandler) Execute(ctx context.Context, skill Skill, params map[string]interface{}, context SkillContext) (SkillResult, error) {
	code, ok := params["code"].(string)
	if !ok {
		return SkillResult{Success: false, Error: "code parameter required"}, nil
	}

	output, err := h.executeCode(ctx, code, context)
	if err != nil {
		return SkillResult{Success: false, Error: err.Error()}, nil
	}

	return SkillResult{Success: true, Output: output}, nil
}

func (h *CodeExecutionHandler) executeCode(ctx context.Context, code string, context SkillContext) (string, error) {
	return fmt.Sprintf("Executed: %s (sandbox not implemented)", code[:min(50, len(code))]), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func CreateSkillManifest(skills []Skill) Manifest {
	return Manifest{
		Version: "1.0",
		Skills:  skills,
		Settings: Settings{
			AllowInlineSkills: true,
			MaxConcurrent:     3,
		},
	}
}

func GenerateSkillID() string {
	return uuid.New().String()
}
