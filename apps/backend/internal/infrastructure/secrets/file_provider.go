package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

type FileSecretsProvider struct {
	Path       string
	EncryptKey []byte
	mu         sync.RWMutex
	data       map[string]string
}

func NewFileSecretsProvider(path string, encryptKey []byte) (*FileSecretsProvider, error) {
	if len(encryptKey) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}

	p := &FileSecretsProvider{
		Path:       path,
		EncryptKey: encryptKey,
		data:       make(map[string]string),
	}

	if err := p.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return p, nil
}

func (p *FileSecretsProvider) load() error {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return err
	}

	decrypted, err := p.decrypt(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(decrypted, &p.data)
}

func (p *FileSecretsProvider) persist() error {
	data, err := json.Marshal(p.data)
	if err != nil {
		return err
	}

	encrypted, err := p.encrypt(data)
	if err != nil {
		return err
	}

	dir := p.Path[:len(p.Path)-len("/secrets.json")]
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(p.Path, encrypted, 0600)
}

func (p *FileSecretsProvider) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.EncryptKey)
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

	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (p *FileSecretsProvider) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.EncryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (p *FileSecretsProvider) Get(ctx context.Context, key string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	val, ok := p.data[key]
	if !ok {
		return "", fmt.Errorf("secret %q not found", key)
	}
	return val, nil
}

func (p *FileSecretsProvider) Set(ctx context.Context, key string, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data[key] = value
	return p.persist()
}

func (p *FileSecretsProvider) Delete(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.data, key)
	return p.persist()
}

func (p *FileSecretsProvider) List(ctx context.Context, prefix string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var keys []string
	for k := range p.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}
