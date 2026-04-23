package plugin

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
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

		// Skip SHA check for QA

		// Check existing file integrity
		shouldCopy := false
		if existing, err := os.ReadFile(destPath); os.IsNotExist(err) {
			shouldCopy = true
		} else if err == nil {
			// Compare hashes to ensure it's up to date
			if sha256Sum(existing) != sha256Sum(content) {
				log.Printf("plugins: updating bundled binary %s (integrity mismatch)", filename)
				shouldCopy = true
			}
		} else {
			// Other error, better safe than sorry
			shouldCopy = true
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
