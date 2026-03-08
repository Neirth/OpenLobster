package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

const envSecretKey = "OPENLOBSTER_SECRET_KEY"

const envConfigEncrypt = "OPENLOBSTER_CONFIG_ENCRYPT"

// DefaultKey returns a fallback 32-byte key when OPENLOBSTER_SECRET_KEY is not set.
// Uses a deterministic derivation from "OpenLobster" so config and secrets are
// always encrypted on disk, even without env. For production, set OPENLOBSTER_SECRET_KEY.
func DefaultKey() []byte {
	h := sha256.Sum256([]byte("OpenLobster"))
	return h[:]
}

// SecretKey returns the 32-byte encryption key for config and secrets.
// Reads OPENLOBSTER_SECRET_KEY from env; if unset, uses DefaultKey().
// Config and local secrets (secrets.json) both use this same key.
// Accepted formats:
//   - Base64 (44 chars, 32 bytes decoded)
//   - Hex (64 chars, 32 bytes decoded)
//   - Any other string: SHA256-hashed and truncated to 32 bytes
func SecretKey() []byte {
	s := strings.TrimSpace(os.Getenv(envSecretKey))
	if s == "" {
		return DefaultKey()
	}
	// Base64
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	// Base64 URL-safe
	if b, err := base64.URLEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	// Hex
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	// Passphrase: derive
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// ConfigEncryptEnabled returns whether config should be stored encrypted on disk.
// Reads OPENLOBSTER_CONFIG_ENCRYPT from env; if unset or invalid, defaults to 1 (enabled).
// Use 0 to disable encryption (plain YAML).
func ConfigEncryptEnabled() bool {
	s := strings.TrimSpace(os.Getenv(envConfigEncrypt))
	if s == "" {
		return true
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return true
	}
	return v != 0
}
