package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	inputBuf  []byte
	resultBuf []byte
	mu        sync.RWMutex
	store     map[string]string
	storePath string
)

//go:wasmexport openlobster_alloc_input
func allocInput(size uint32) uint32 {
	inputBuf = make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&inputBuf[0])))
}

//go:wasmexport openlobster_result_ptr
func resultPtr() uint32 {
	if len(resultBuf) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

//go:wasmexport openlobster_result_len
func resultLen() uint32 {
	return uint32(len(resultBuf))
}

func writeResult(v interface{}) int32 {
	b, err := json.Marshal(v)
	if err != nil {
		resultBuf = []byte(`{"error":"marshal failed"}`)
		return 1
	}
	resultBuf = b
	return 0
}

func writeStringResult(s string) int64 {
	resultBuf = []byte(s)
	ptr := uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
	return int64(ptr)<<32 | int64(len(resultBuf))
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_get_name
func getName() int64 { return writeStringResult("openlobster-memory-gml") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("File-based (JSON) memory storage for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("memory") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"path":{"type":"string","title":"Storage Path","default":"~/.openlobster/memory.json"}}}`)
}

// ---------------------------------------------------------------------------
// Persistence helpers
// ---------------------------------------------------------------------------

func init() {
	home, _ := os.UserHomeDir()
	storePath = filepath.Join(home, ".openlobster", "memory.json")
	store = make(map[string]string)
	_ = loadStore()
}

func loadStore() error {
	b, err := os.ReadFile(storePath)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	return json.Unmarshal(b, &store)
}

func saveStore() error {
	mu.RLock()
	b, err := json.Marshal(store)
	mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(storePath, b, 0o644)
}

// ---------------------------------------------------------------------------
// Memory operations
// ---------------------------------------------------------------------------

type StoreInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// optional: metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Config   struct {
		Path string `json:"path,omitempty"`
	} `json:"config"`
}

type RetrieveInput struct {
	Key    string `json:"key"`
	Config struct {
		Path string `json:"path,omitempty"`
	} `json:"config"`
}

type QueryInput struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Config struct {
		Path string `json:"path,omitempty"`
	} `json:"config"`
}

type MemoryEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

//go:wasmexport openlobster_store
func storeEntry() int32 {
	var input StoreInput
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	if input.Config.Path != "" {
		storePath = input.Config.Path
		_ = loadStore()
	}
	mu.Lock()
	store[input.Key] = input.Value
	mu.Unlock()
	if err := saveStore(); err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	return writeResult(map[string]bool{"ok": true})
}

//go:wasmexport openlobster_retrieve
func retrieve() int64 {
	var input RetrieveInput
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		return writeStringResult(`{"error":"invalid input"}`)
	}
	if input.Config.Path != "" {
		storePath = input.Config.Path
		_ = loadStore()
	}
	mu.RLock()
	val, ok := store[input.Key]
	mu.RUnlock()
	if !ok {
		return writeStringResult(`{"value":null,"found":false}`)
	}
	b, _ := json.Marshal(map[string]interface{}{"value": val, "found": true})
	return writeStringResult(string(b))
}

//go:wasmexport openlobster_query
func query() int64 {
	var input QueryInput
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		return writeStringResult(`{"error":"invalid input"}`)
	}
	if input.Config.Path != "" {
		storePath = input.Config.Path
		_ = loadStore()
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	queryLower := strings.ToLower(input.Query)
	mu.RLock()
	var results []MemoryEntry
	for k, v := range store {
		if queryLower == "" || strings.Contains(strings.ToLower(k), queryLower) || strings.Contains(strings.ToLower(v), queryLower) {
			results = append(results, MemoryEntry{Key: k, Value: v})
			if len(results) >= limit {
				break
			}
		}
	}
	mu.RUnlock()
	b, _ := json.Marshal(map[string]interface{}{"results": results})
	return writeStringResult(string(b))
}

// configure allows updating the storage path at runtime
//go:wasmexport openlobster_configure
func configure() int32 {
	var input struct {
		Config struct {
			Path string `json:"path,omitempty"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	if input.Config.Path != "" {
		storePath = input.Config.Path
		_ = loadStore()
	}
	return writeResult(map[string]bool{"ok": true})
}

// Prevent unused import lint error
var _ = time.Now

func main() {}
