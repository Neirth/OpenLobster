package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pdk "github.com/extism/go-pdk"
)

type pluginConfig struct {
	Path string `json:"path,omitempty"`
	Key  string `json:"key,omitempty"`
}

type getInput struct {
	Key    string       `json:"key"`
	Config pluginConfig `json:"config"`
}

type setInput struct {
	Key    string       `json:"key"`
	Value  string       `json:"value"`
	Config pluginConfig `json:"config"`
}

type deleteInput struct {
	Key    string       `json:"key"`
	Config pluginConfig `json:"config"`
}

type listInput struct {
	Prefix string       `json:"prefix"`
	Config pluginConfig `json:"config"`
}

type getOutput struct {
	Value string `json:"value,omitempty"`
	Found *bool  `json:"found,omitempty"`
	Error string `json:"error,omitempty"`
}

type okOutput struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type listOutput struct {
	Keys  []string `json:"keys"`
	Error string   `json:"error,omitempty"`
}

var storeMu sync.Mutex

//go:wasmexport get_name
func getName() int32 {
	pdk.OutputString("openlobster-secrets-json")
	return 0
}

//go:wasmexport get_version
func getVersion() int32 {
	pdk.OutputString("0.1.0")
	return 0
}

//go:wasmexport get_description
func getDescription() int32 {
	pdk.OutputString("Encrypted JSON secrets provider for OpenLobster")
	return 0
}

//go:wasmexport get_type
func getType() int32 {
	pdk.OutputString("secrets")
	return 0
}

//go:wasmexport get_schema
func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"path":{"type":"string","title":"Secrets File Path","default":"~/.openlobster/secrets.json","description":"Encrypted JSON file used to store secrets"},"key":{"type":"string","title":"Encryption Key Override","description":"Optional base64/hex/passphrase. If empty, the plugin uses environment key fallback"}},"additionalProperties":false}`)
	return 0
}

//go:wasmexport get
func getSecret() int32 {
	var in getInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}

	path := resolveStoragePath(in.Config.Path)
	key := resolveEncryptionKey(in.Config.Key)

	storeMu.Lock()
	data, err := loadSecrets(path, key)
	storeMu.Unlock()
	if err != nil {
		return writeJSON(getOutput{Error: err.Error()})
	}

	value, ok := data[in.Key]
	if !ok {
		found := false
		return writeJSON(getOutput{Found: &found})
	}
	found := true
	return writeJSON(getOutput{Value: value, Found: &found})
}

//go:wasmexport set
func setSecret() int32 {
	var in setInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}

	path := resolveStoragePath(in.Config.Path)
	key := resolveEncryptionKey(in.Config.Key)

	storeMu.Lock()
	defer storeMu.Unlock()

	data, err := loadSecrets(path, key)
	if err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	data[in.Key] = in.Value
	if err := saveSecrets(path, key, data); err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	return writeJSON(okOutput{OK: true})
}

//go:wasmexport delete
func deleteSecret() int32 {
	var in deleteInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}

	path := resolveStoragePath(in.Config.Path)
	key := resolveEncryptionKey(in.Config.Key)

	storeMu.Lock()
	defer storeMu.Unlock()

	data, err := loadSecrets(path, key)
	if err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	delete(data, in.Key)
	if err := saveSecrets(path, key, data); err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	return writeJSON(okOutput{OK: true})
}

//go:wasmexport list
func listSecrets() int32 {
	var in listInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}

	path := resolveStoragePath(in.Config.Path)
	key := resolveEncryptionKey(in.Config.Key)

	storeMu.Lock()
	data, err := loadSecrets(path, key)
	storeMu.Unlock()
	if err != nil {
		return writeJSON(listOutput{Error: err.Error()})
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		if strings.HasPrefix(k, in.Prefix) {
			keys = append(keys, k)
		}
	}

	return writeJSON(listOutput{Keys: keys})
}

func writeJSON(v any) int32 {
	if err := pdk.OutputJSON(v); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func resolveStoragePath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, ".openlobster", "secrets.json")
	}
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	if strings.HasSuffix(p, string(os.PathSeparator)) {
		return filepath.Join(p, "secrets.json")
	}
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return filepath.Join(p, "secrets.json")
	}
	if filepath.Ext(p) == "" {
		return filepath.Join(p, "secrets.json")
	}
	return p
}

func resolveEncryptionKey(override string) []byte {
	raw := strings.TrimSpace(override)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("OPENLOBSTER_SECRET_KEY"))
	}
	if raw == "" {
		h := sha256.Sum256([]byte("OpenLobster"))
		return h[:]
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b
	}
	if b, err := base64.URLEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
		return b
	}
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func loadSecrets(path string, key []byte) (map[string]string, error) {
	data := make(map[string]string)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return data, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return data, nil
	}
	plain, err := decrypt(key, b)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func saveSecrets(path string, key []byte, data map[string]string) error {
	plain, err := json.Marshal(data)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(key, plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, ciphertext, 0o600)
}

func encrypt(key []byte, plain []byte) ([]byte, error) {
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
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func decrypt(key []byte, data []byte) ([]byte, error) {
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
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func main() {}
