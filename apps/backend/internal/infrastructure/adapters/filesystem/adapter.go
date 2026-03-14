package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neirth/openlobster/internal/domain/services/mcp"
)

// Adapter provides filesystem operations for the agent's internal tools.
// configPath is the absolute path to the application configuration file;
// any attempt to read, write, edit, or list it is denied for security.
type Adapter struct {
	configPath string
}

func NewAdapter(configPath string) *Adapter {
	abs, err := filepath.Abs(configPath)
	if err != nil {
		abs = configPath
	}
	return &Adapter{configPath: abs}
}

// isProtected returns true when the resolved absolute path equals or is a
// child of the application config file path.
func (a *Adapter) isProtected(absPath string) bool {
	if a.configPath == "" {
		return false
	}
	return absPath == a.configPath || strings.HasPrefix(absPath, a.configPath+string(filepath.Separator))
}

func (a *Adapter) ReadFile(ctx context.Context, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if a.isProtected(absPath) {
		return "", fmt.Errorf("access denied: the application configuration file is protected")
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (a *Adapter) ReadFileBytes(ctx context.Context, path string) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if a.isProtected(absPath) {
		return nil, fmt.Errorf("access denied: the application configuration file is protected")
	}
	return os.ReadFile(absPath)
}

func (a *Adapter) WriteFile(ctx context.Context, path, content string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if a.isProtected(absPath) {
		return fmt.Errorf("access denied: the application configuration file is protected")
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

func (a *Adapter) WriteBytes(ctx context.Context, path string, data []byte) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if a.isProtected(absPath) {
		return fmt.Errorf("access denied: the application configuration file is protected")
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(absPath, data, 0644)
}

func (a *Adapter) EditFile(ctx context.Context, path, oldContent, newContent string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if a.isProtected(absPath) {
		return fmt.Errorf("access denied: the application configuration file is protected")
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	newContentFile := strings.Replace(string(content), oldContent, newContent, 1)
	if string(content) == newContentFile {
		return fmt.Errorf("content not found")
	}

	return os.WriteFile(absPath, []byte(newContentFile), 0644)
}

func (a *Adapter) ListContent(ctx context.Context, path string) ([]mcp.FileEntry, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if a.isProtected(absPath) {
		return nil, fmt.Errorf("access denied: the application configuration file is protected")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	result := make([]mcp.FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		result = append(result, mcp.FileEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(absPath, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
			Mode:  info.Mode().String(),
		})
	}

	return result, nil
}

var _ mcp.FilesystemService = (*Adapter)(nil)
