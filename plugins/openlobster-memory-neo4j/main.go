package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unsafe"

	_ "github.com/stealthrocket/net/wasip1"
)

var (
	inputBuf  []byte
	resultBuf []byte
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
func getName() int64 { return writeStringResult("openlobster-memory-neo4j") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Neo4j graph memory storage for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("memory") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"url":{"type":"string","title":"Neo4j HTTP URL","default":"http://localhost:7474"},"username":{"type":"string","title":"Username","default":"neo4j"},"password":{"type":"string","title":"Password"}},"required":["url"]}`)
}

// ---------------------------------------------------------------------------
// Neo4j HTTP helpers
// ---------------------------------------------------------------------------

type neo4jConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func neo4jQuery(cfg neo4jConfig, cypher string, params map[string]interface{}) ([]map[string]interface{}, error) {
	body := map[string]interface{}{
		"statements": []map[string]interface{}{
			{"statement": cypher, "parameters": params},
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(cfg.URL, "/") + "/db/data/transaction/commit"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			Columns []string        `json:"columns"`
			Data    []struct {
				Row []interface{} `json:"row"`
			} `json:"data"`
		} `json:"results"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("neo4j error: %s", result.Errors[0].Message)
	}

	var rows []map[string]interface{}
	if len(result.Results) > 0 {
		res := result.Results[0]
		for _, d := range res.Data {
			row := make(map[string]interface{})
			for i, col := range res.Columns {
				if i < len(d.Row) {
					row[col] = d.Row[i]
				}
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// Memory operations
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_store
func storeEntry() int32 {
	var input struct {
		Key   string                 `json:"key"`
		Value string                 `json:"value"`
		Meta  map[string]interface{} `json:"metadata,omitempty"`
		Config neo4jConfig           `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	cypher := `MERGE (m:Memory {key: $key}) SET m.value = $value, m.updated = timestamp()`
	params := map[string]interface{}{"key": input.Key, "value": input.Value}
	if _, err := neo4jQuery(input.Config, cypher, params); err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	return writeResult(map[string]bool{"ok": true})
}

//go:wasmexport openlobster_retrieve
func retrieve() int64 {
	var input struct {
		Key    string      `json:"key"`
		Config neo4jConfig `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		return writeStringResult(`{"error":"invalid input"}`)
	}
	cypher := `MATCH (m:Memory {key: $key}) RETURN m.value AS value`
	params := map[string]interface{}{"key": input.Key}
	rows, err := neo4jQuery(input.Config, cypher, params)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return writeStringResult(string(b))
	}
	if len(rows) == 0 {
		return writeStringResult(`{"value":null,"found":false}`)
	}
	b, _ := json.Marshal(map[string]interface{}{"value": rows[0]["value"], "found": true})
	return writeStringResult(string(b))
}

//go:wasmexport openlobster_query
func queryMem() int64 {
	var input struct {
		Query  string      `json:"query"`
		Limit  int         `json:"limit,omitempty"`
		Config neo4jConfig `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		return writeStringResult(`{"error":"invalid input"}`)
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	cypher := `MATCH (m:Memory) WHERE toLower(m.key) CONTAINS toLower($q) OR toLower(m.value) CONTAINS toLower($q) RETURN m.key AS key, m.value AS value LIMIT $limit`
	params := map[string]interface{}{"q": input.Query, "limit": limit}
	rows, err := neo4jQuery(input.Config, cypher, params)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return writeStringResult(string(b))
	}
	b, _ := json.Marshal(map[string]interface{}{"results": rows})
	return writeStringResult(string(b))
}

//go:wasmexport openlobster_configure
func configure() int32 {
	return writeResult(map[string]bool{"ok": true})
}

func main() {}
