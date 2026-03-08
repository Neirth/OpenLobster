package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const encryptedMagic = "OLENC1" // 6 bytes

func isEncrypted(data []byte) bool {
	return len(data) >= len(encryptedMagic) && string(data[:len(encryptedMagic)]) == encryptedMagic
}

func encryptConfig(plain []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return append([]byte(encryptedMagic), ciphertext...), nil
}

func decryptConfig(data []byte, key []byte) ([]byte, error) {
	data = data[len(encryptedMagic):]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errCorrupted
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

var errCorrupted = &validationError{msg: "encrypted config: corrupted or wrong key"}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

// ReadConfigBytes reads the config file, decrypting if encrypted.
// Used by Load to support both plain YAML and encrypted format.
func ReadConfigBytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !isEncrypted(data) {
		return data, nil
	}
	return decryptConfig(data, SecretKey())
}

// WriteEncryptedConfig writes the current viper config to path.
// Encrypted if OPENLOBSTER_CONFIG_ENCRYPT is 1 (default), plain YAML if 0.
func WriteEncryptedConfig(path string) error {
	return WriteEncryptedConfigFromSettings(viper.AllSettings(), path)
}

// WriteEncryptedConfigFromSettings writes the given settings map to path.
// Encrypted if OPENLOBSTER_CONFIG_ENCRYPT is 1 (default), plain YAML if 0.
func WriteEncryptedConfigFromSettings(settings map[string]interface{}, path string) error {
	plain, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	if !ConfigEncryptEnabled() {
		return os.WriteFile(path, plain, 0600)
	}
	encrypted, err := encryptConfig(plain, SecretKey())
	if err != nil {
		return err
	}
	return os.WriteFile(path, encrypted, 0600)
}

// WriteEncryptedConfigFromViper writes the given viper instance's settings to
// path, encrypted. Used by bootstrap when creating a new config file.
func WriteEncryptedConfigFromViper(v *viper.Viper, path string) error {
	return WriteEncryptedConfigFromSettings(v.AllSettings(), path)
}
