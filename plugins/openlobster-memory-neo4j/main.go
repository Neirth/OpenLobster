package main

import (
"bytes"
"encoding/json"
"fmt"
"io"
"net/http"
"strings"

pdk "github.com/extism/go-pdk"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-memory-neo4j"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("Neo4j graph memory storage for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("memory"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"url":{"type":"string","title":"Neo4j HTTP URL","default":"http://localhost:7474"},"username":{"type":"string","title":"Username","default":"neo4j"},"password":{"type":"string","title":"Password"}},"required":["url"]}`)
return 0
}

// ---------------------------------------------------------------------------
// Neo4j HTTP helper
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
Columns []string `json:"columns"`
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
// store
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
Config      neo4jConfig `json:"config"`
}

//go:wasmexport store
func storeEntry() int32 {
var input storeInput
if err := pdk.InputJSON(&input); err != nil {
pdk.SetError(err)
return 1
}
var cypher string
params := map[string]interface{}{}

switch input.Op {
case "add_relation":
cypher = `MERGE (a:Node {id:$from}) MERGE (b:Node {id:$to}) MERGE (a)-[r:RELATED {type:$rel}]->(b)`
params["from"] = input.From
params["to"] = input.To
params["rel"] = input.RelType
case "delete_relation":
cypher = `MATCH (a:Node {id:$from})-[r]->(b:Node {id:$to}) DELETE r`
params["from"] = input.From
params["to"] = input.To
case "invalidate_cache":
if err := pdk.OutputJSON(map[string]bool{"ok": true}); err != nil {
pdk.SetError(err)
return 1
}
return 0
case "set_user_property":
cypher = `MERGE (u:User {id:$uid}) SET u[$key] = $val`
params["uid"] = input.UserID
params["key"] = input.Key
params["val"] = input.Value
case "edit_node":
cypher = `MATCH (n:Memory {id:$nid}) SET n.value = $val`
params["nid"] = input.NodeID
params["val"] = input.NewValue
case "delete_node":
cypher = `MATCH (n:Memory {id:$nid}) DETACH DELETE n`
params["nid"] = input.NodeID
case "update_user_label":
cypher = `MERGE (u:User {id:$uid}) SET u.label = $lbl`
params["uid"] = input.UserID
params["lbl"] = input.DisplayName
default:
cypher = `MERGE (u:User {id:$uid}) MERGE (m:Memory {id:$kid}) SET m.content=$content, m.label=$label, m.type=$etype MERGE (u)-[:KNOWS]->(m)`
params["uid"] = input.UserID
params["kid"] = input.UserID + ":" + input.Label
params["content"] = input.Content
params["label"] = input.Label
params["etype"] = input.EntityType
}

if _, err := neo4jQuery(input.Config, cypher, params); err != nil {
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
Config neo4jConfig `json:"config"`
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
limit := input.Limit
if limit <= 0 {
limit = 20
}
cypher := `MATCH (m:Memory) WHERE toLower(m.content) CONTAINS toLower($q) OR toLower(m.label) CONTAINS toLower($q) RETURN m.id AS id, m.content AS content LIMIT $limit`
params := map[string]interface{}{"q": input.Query, "limit": limit}
rows, err := neo4jQuery(input.Config, cypher, params)
if err != nil {
pdk.SetError(err)
return 1
}
results := make([]knowledgeEntry, 0, len(rows))
for _, r := range rows {
id, _ := r["id"].(string)
content, _ := r["content"].(string)
results = append(results, knowledgeEntry{ID: id, Content: content})
}
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
Config neo4jConfig `json:"config"`
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
switch input.Op {
case "user_graph":
nodeRows, _ := neo4jQuery(input.Config,
`MATCH (u:User {id:$uid})-[:KNOWS]->(m:Memory) RETURN m.id AS id, m.label AS label, m.type AS type, m.content AS content`,
map[string]interface{}{"uid": input.UserID})
edgeRows, _ := neo4jQuery(input.Config,
`MATCH (a:Node)-[r:RELATED]->(b:Node) RETURN a.id AS src, b.id AS tgt, r.type AS lbl`,
map[string]interface{}{})
g := graph{}
for _, r := range nodeRows {
id, _ := r["id"].(string)
lbl, _ := r["label"].(string)
tp, _ := r["type"].(string)
val, _ := r["content"].(string)
g.Nodes = append(g.Nodes, graphNode{ID: id, Label: lbl, Type: tp, Value: val})
}
for _, r := range edgeRows {
src, _ := r["src"].(string)
tgt, _ := r["tgt"].(string)
lbl, _ := r["lbl"].(string)
g.Edges = append(g.Edges, graphEdge{Source: src, Target: tgt, Label: lbl})
}
if err := pdk.OutputJSON(g); err != nil {
pdk.SetError(err)
return 1
}
case "cypher":
rows, err := neo4jQuery(input.Config, input.Cypher, map[string]interface{}{})
if err != nil {
pdk.SetError(err)
return 1
}
if err := pdk.OutputJSON(map[string]interface{}{"data": rows, "errors": []interface{}{}}); err != nil {
pdk.SetError(err)
return 1
}
default:
if err := pdk.OutputJSON(map[string]interface{}{"data": []interface{}{}, "errors": []interface{}{}}); err != nil {
pdk.SetError(err)
return 1
}
}
return 0
}

func main() {}
