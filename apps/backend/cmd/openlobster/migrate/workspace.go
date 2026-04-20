// # License
// See LICENSE in the root of the repository.
package migrate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// defaultOpenLobsterHome returns the default OpenLobster home directory
// (~/.openlobster), falling back to ./.openlobster if the home directory
// cannot be determined.
func defaultOpenLobsterHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./.openlobster"
	}
	return filepath.Join(home, ".openlobster")
}

// migrateWorkspace copies the OpenClaw workspace directory tree to the
// OpenLobster workspace directory, preserving relative paths.
//
// Existing files in dst are overwritten; no files are deleted from dst.
func migrateWorkspace(src, dst string, dryRun bool) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("workspace: source %q not found — skipping\n", src)
			return nil
		}
		return fmt.Errorf("workspace: cannot stat source: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace: source %q is not a directory", src)
	}

	fmt.Printf("workspace: %s → %s\n", src, dst)

	var copied int
	err = filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if fi.IsDir() {
			if dryRun {
				fmt.Printf("  mkdir  %s\n", rel)
				return nil
			}
			return os.MkdirAll(target, fi.Mode())
		}

		fmt.Printf("  copy   %s\n", rel)
		if dryRun {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		if err := copyFile(path, target, fi.Mode()); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		copied++
		return nil
	})
	if err != nil {
		return fmt.Errorf("workspace: %w", err)
	}

	if !dryRun {
		fmt.Printf("  copied %d file(s)\n", copied)
	}
	return nil
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// resolveWorkspaceSrc returns the OpenClaw workspace source directory.
// It prefers the value of agents.defaults.workspace in the config; if absent
// or empty it falls back to <configDir>/workspace.
func resolveWorkspaceSrc(cfg viperReader, configPath string) string {
	if ws := cfg.GetString("agents.defaults.workspace"); ws != "" {
		expanded := expandHome(ws)
		return expanded
	}
	return filepath.Join(filepath.Dir(configPath), "workspace")
}

// expandHome replaces a leading "~" with the user home directory.
func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
