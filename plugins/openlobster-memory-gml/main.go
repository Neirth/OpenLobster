package main

import (
"encoding/json"
"os"
"path/filepath"
"strings"
"sync"
"time"

pdk "github.com/extism/go-pdk"
)

var (
mu        sync.RWMutex
store     map[string]string
storePath string
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-memory-gml"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("File-based (JSON) memory storage for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("memory"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"path":{"type":"string","title":"Storage Path","default":"~/.openlobster/memory.json"}}}`)
return 0
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
// store — handles add_knowledge, add_relation, delete_relation,
// invalidate_cache, set_user_property, edit_node, delete_node,
// update_user_label (all non-query write operations)
// ---------------------------------------------------------------------------

type storeInput struct {
Op          string      `json:"op"`
UserID      string      `json:"user_id,omitempty"`
Content     string      `json:"content,omitempty"`
Label       string      `json:"label,omitempty"`
Relation    string      `json:"relation,omitempty"`
EntityType  string      `json:"entity_type,omitempty"`
Key         string      `json:"key,omitempty"`
Value       string      `json:"value,omitempty"`
From        string      `json:"from,omitempty"`
To          string      `json:"to,omitempty"`
RelType     string      `json:"rel_type,omitempty"`
NodeID      string      `json:"node_id,omitempty"`
NewValue    string      `json:"new_value,omitempty"`
DisplayName string      `json:"display_name,omitempty"`
Config      storeConfig `json:"config"`
}

type storeConfig struct {
Path string `json:"path,omitempty"`
}

//go:wasmexport store
func storeEntry() int32 {
var input storeInput
if err := pdk.InputJSON(&input); err != nil {
pdk.SetError(err)
return 1
}
if input.Config.Path != "" && input.Config.Path != storePath {
storePath = input.Config.Path
_ = loadStore()
}

mu.Lock()
switch input.Op {
case "add_relation":
k := "rel:" + input.From + ":" + input.To
store[k] = input.RelType
case "delete_relation":
delete(store, "rel:"+input.From+":"+input.To)
case "invalidate_cache":
// No-op for file-based store
case "set_user_property":
store["prop:"+input.UserID+":"+input.Key] = input.Value
case "edit_node":
store[input.NodeID] = input.NewValue
case "delete_node":
delete(store, input.NodeID)
case "update_user_label":
store["label:"+input.UserID] = input.DisplayName
default:
// Default: store knowledge content
key := input.Key
if key == "" {
key = "know:" + input.UserID + ":" + time.Now().Format(time.RFC3339Nano)
}
store[key] = input.Content
}
mu.Unlock()

if err := saveStore(); err != nil {
pdk.SetError(err)
return 1
}
if err := pdk.OutputJSON(map[string]bool{"ok": true}); err != nil {
pdk.SetError(err)
return 1
}
return 0
}

// ---------------------------------------------------------------------------
// retrieve — SearchSimilar
// ---------------------------------------------------------------------------

type retrieveInput struct {
Query  string      `json:"query"`
Limit  int         `json:"limit,omitempty"`
Config storeConfig `json:"config"`
}

type knowledgeEntry struct {
ID      string `json:"id"`
UserID  string `json:"user_id"`
Content string `json:"content"`
}

//go:wasmexport retrieve
func retrieve() int32 {
var input retrieveInput
if err := pdk.InputJSON(&input); err != nil {
pdk.SetError(err)
return 1
}
if input.Config.Path != "" && input.Config.Path != storePath {
storePath = input.Config.Path
_ = loadStore()
}
limit := input.Limit
if limit <= 0 {
limit = 20
}
q := strings.ToLower(input.Query)
mu.RLock()
var results []knowledgeEntry
for k, v := range store {
if q == "" || strings.Contains(strings.ToLower(k), q) || strings.Contains(strings.ToLower(v), q) {
results = append(results, knowledgeEntry{ID: k, Content: v})
if len(results) >= limit {
break
}
}
}
mu.RUnlock()
if err := pdk.OutputJSON(results); err != nil {
pdk.SetError(err)
return 1
}
return 0
}

// ---------------------------------------------------------------------------
// query — GetUserGraph, cypher
// ---------------------------------------------------------------------------

type queryInput struct {
Op     string      `json:"op"`
UserID string      `json:"user_id,omitempty"`
Cypher string      `json:"cypher,omitempty"`
Config storeConfig `json:"config"`
}

type graphNode struct {
ID    string `json:"id"`
Label string `json:"label"`
Type  string `json:"type"`
Value string `json:"value"`
}

type graphEdge struct {
Source string `json:"source"`
Target string `json:"target"`
Label  string `json:"label"`
}

type graph struct {
Nodes []graphNode `json:"nodes"`
Edges []graphEdge `json:"edges"`
}

//go:wasmexport query
func queryMem() int32 {
var input queryInput
if err := pdk.InputJSON(&input); err != nil {
pdk.SetError(err)
return 1
}
if input.Config.Path != "" && input.Config.Path != storePath {
storePath = input.Config.Path
_ = loadStore()
}

mu.RLock()
defer mu.RUnlock()

switch input.Op {
case "user_graph":
g := graph{}
prefix := "know:" + input.UserID + ":"
relPrefix := "rel:" + input.UserID + ":"
for k, v := range store {
if strings.HasPrefix(k, prefix) {
g.Nodes = append(g.Nodes, graphNode{ID: k, Label: v, Type: "fact", Value: v})
} else if strings.HasPrefix(k, relPrefix) {
parts := strings.SplitN(k[len("rel:"):], ":", 2)
if len(parts) == 2 {
g.Edges = append(g.Edges, graphEdge{Source: parts[0], Target: parts[1], Label: v})
}
}
}
if err := pdk.OutputJSON(g); err != nil {
pdk.SetError(err)
return 1
}
default:
// For cypher or unrecognised ops return empty graph result
if err := pdk.OutputJSON(map[string]interface{}{"data": []interface{}{}, "errors": []interface{}{}}); err != nil {
pdk.SetError(err)
return 1
}
}
return 0
}

func main() {}
