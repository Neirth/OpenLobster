package main

import (
	"context"
	"fmt"

	pdk "github.com/extism/go-pdk"
	neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
	pdk.OutputString(`{"type":"object","properties":{"uri":{"type":"string","title":"Bolt URI","default":"bolt://localhost:7687","description":"Neo4j connection URI"},"username":{"type":"string","title":"Username","default":"neo4j","description":"Neo4j username"},"password":{"type":"string","title":"Password","description":"Neo4j password"},"database":{"type":"string","title":"Database","default":"neo4j","description":"Neo4j database name"}},"required":["uri"]}`)
	return 0
}

// ---------------------------------------------------------------------------
// Config helper
// ---------------------------------------------------------------------------

type neo4jConfig struct {
	URI      string `json:"uri"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func newDriver(cfg neo4jConfig) (neo4j.DriverWithContext, error) {
	if cfg.URI == "" {
		cfg.URI = "bolt://localhost:7687"
	}
	if cfg.Username == "" {
		cfg.Username = "neo4j"
	}
	if cfg.Database == "" {
		cfg.Database = "neo4j"
	}
	return neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
}

func runWrite(ctx context.Context, driver neo4j.DriverWithContext, db, cypher string, params map[string]any) error {
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: db, AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		return res.Collect(ctx)
	})
	return err
}

func runRead(ctx context.Context, driver neo4j.DriverWithContext, db, cypher string, params map[string]any) ([]*neo4j.Record, error) {
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: db, AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)
	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		return res.Collect(ctx)
	})
	if err != nil {
		return nil, err
	}
	records, _ := result.([]*neo4j.Record)
	return records, nil
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

type storeInput struct {
	Op          string      `json:"op"`
	UserID      string      `json:"user_id,omitempty"`
	Content     string      `json:"content,omitempty"`
	Label       string      `json:"label,omitempty"`
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

	ctx := context.Background()
	driver, err := newDriver(input.Config)
	if err != nil {
		pdk.SetError(fmt.Errorf("driver: %w", err))
		return 1
	}
	defer driver.Close(ctx)

	db := input.Config.Database
	if db == "" {
		db = "neo4j"
	}

	var cypher string
	params := map[string]any{}

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

	if err := runWrite(ctx, driver, db, cypher, params); err != nil {
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
// retrieve
// ---------------------------------------------------------------------------

type retrieveInput struct {
	Query  string      `json:"query"`
	Limit  int         `json:"limit,omitempty"`
	Config neo4jConfig `json:"config"`
}

type knowledgeEntry struct {
	ID      string `json:"id"`
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

	ctx := context.Background()
	driver, err := newDriver(input.Config)
	if err != nil {
		pdk.SetError(fmt.Errorf("driver: %w", err))
		return 1
	}
	defer driver.Close(ctx)

	db := input.Config.Database
	if db == "" {
		db = "neo4j"
	}

	cypher := `MATCH (m:Memory) WHERE toLower(m.content) CONTAINS toLower($q) OR toLower(m.label) CONTAINS toLower($q) RETURN m.id AS id, m.content AS content LIMIT $limit`
	records, err := runRead(ctx, driver, db, cypher, map[string]any{"q": input.Query, "limit": limit})
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	results := make([]knowledgeEntry, 0, len(records))
	for _, r := range records {
		id, _ := r.Get("id")
		content, _ := r.Get("content")
		results = append(results, knowledgeEntry{
			ID:      fmt.Sprintf("%v", id),
			Content: fmt.Sprintf("%v", content),
		})
	}
	if err := pdk.OutputJSON(results); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// query
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

	ctx := context.Background()
	driver, err := newDriver(input.Config)
	if err != nil {
		pdk.SetError(fmt.Errorf("driver: %w", err))
		return 1
	}
	defer driver.Close(ctx)

	db := input.Config.Database
	if db == "" {
		db = "neo4j"
	}

	switch input.Op {
	case "user_graph":
		g := graph{}
		nodeRecords, _ := runRead(ctx, driver, db,
			`MATCH (u:User {id:$uid})-[:KNOWS]->(m:Memory) RETURN m.id AS id, m.label AS label, m.type AS type, m.content AS content`,
			map[string]any{"uid": input.UserID})
		edgeRecords, _ := runRead(ctx, driver, db,
			`MATCH (a:Node)-[r:RELATED]->(b:Node) RETURN a.id AS src, b.id AS tgt, r.type AS lbl`,
			map[string]any{})
		for _, r := range nodeRecords {
			id, _ := r.Get("id")
			lbl, _ := r.Get("label")
			tp, _ := r.Get("type")
			val, _ := r.Get("content")
			g.Nodes = append(g.Nodes, graphNode{
				ID:    fmt.Sprintf("%v", id),
				Label: fmt.Sprintf("%v", lbl),
				Type:  fmt.Sprintf("%v", tp),
				Value: fmt.Sprintf("%v", val),
			})
		}
		for _, r := range edgeRecords {
			src, _ := r.Get("src")
			tgt, _ := r.Get("tgt")
			lbl, _ := r.Get("lbl")
			g.Edges = append(g.Edges, graphEdge{
				Source: fmt.Sprintf("%v", src),
				Target: fmt.Sprintf("%v", tgt),
				Label:  fmt.Sprintf("%v", lbl),
			})
		}
		if err := pdk.OutputJSON(g); err != nil {
			pdk.SetError(err)
			return 1
		}
	case "cypher":
		records, err := runRead(ctx, driver, db, input.Cypher, map[string]any{})
		if err != nil {
			pdk.SetError(err)
			return 1
		}
		rows := make([]map[string]interface{}, 0, len(records))
		for _, r := range records {
			row := make(map[string]interface{})
			for _, key := range r.Keys {
				v, _ := r.Get(key)
				row[key] = v
			}
			rows = append(rows, row)
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
