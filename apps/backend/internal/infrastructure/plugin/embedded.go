package plugin

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

//go:embed embedded_binaries/*
var embeddedPlugins embed.FS

// ExtractEmbeddedPlugins ensures that the native plugins bundled within the
// binary are present in the target directory and up-to-date.
//
// It performs a SHA256 check: if the file on disk exists but has a different
// hash than the embedded one, it overwrites it. This prevents "adulterated"
// binaries from persisting.
func ExtractEmbeddedPlugins(destDir string) error {
	entries, err := embeddedPlugins.ReadDir("embedded_binaries")
	if err != nil {
		return fmt.Errorf("read embedded plugins: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		embeddedPath := filepath.Join("embedded_binaries", filename)
		destPath := filepath.Join(destDir, filename)

		// Read embedded content
		content, err := embeddedPlugins.ReadFile(embeddedPath)
		if err != nil {
			log.Printf("plugins: failed to read embedded %s: %v", filename, err)
			continue
		}

		// Calculate embedded hash
		embeddedHash := sha256Sum(content)

		// Check existing file
		shouldCopy := false
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			shouldCopy = true
		} else {
			diskContent, err := os.ReadFile(destPath)
			if err != nil {
				shouldCopy = true
			} else {
				diskHash := sha256Sum(diskContent)
				if diskHash != embeddedHash {
					log.Printf("plugins: hash mismatch for %s, overwriting", filename)
					shouldCopy = true
				}
			}
		}

		if shouldCopy {
			log.Printf("plugins: extracting bundled binary %s → %s", filename, destPath)
			if err := os.WriteFile(destPath, content, 0755); err != nil {
				return fmt.Errorf("extract plugin %s: %w", filename, err)
			}
		}
	}

	return nil
}

func sha256Sum(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
