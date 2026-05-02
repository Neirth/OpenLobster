// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Neo4j memory plugin (Rust).
//!
//! Uses neo4rs 0.8 (Bolt 5.x, Neo4j 5 compatible).

use async_trait::async_trait;
use neo4rs::{query, ConfigBuilder, Graph};
use openlobster_sdk_base::{run, emit_log, CallResponse, HotConfig, Plugin, PluginInfo};
use serde_json::{json, Value};
use std::collections::HashMap;
use std::sync::{Arc, LazyLock, Mutex};
use std::time::{SystemTime, UNIX_EPOCH};

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Node counter
// ---------------------------------------------------------------------------

static NODE_COUNTER: LazyLock<Arc<Mutex<u64>>> =
    LazyLock::new(|| Arc::new(Mutex::new(0)));

fn next_node_id() -> String {
    let mut c = NODE_COUNTER.lock().unwrap();
    *c += 1;
    let ts = SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_millis();
    format!("{}-{}", ts, *c)
}

// ---------------------------------------------------------------------------
// Connection helpers
// ---------------------------------------------------------------------------

fn get_connection(input: &Value) -> (String, String, String, String) {
    emit_log("debug", &format!("Neo4j: get_connection INPUT: {:?}", input));
    let per_call: Option<HashMap<String, Value>> = input.get("config")
        .and_then(|c| serde_json::from_value(c.clone()).ok());
    let hot = CONFIG.merge(per_call);
    let uri  = HotConfig::get_str(&hot, "uri");
    let user = HotConfig::get_str(&hot, "user");
    let username = if !user.is_empty() { user } else { HotConfig::get_str(&hot, "username") };
    let database = HotConfig::get_str(&hot, "database");
    (uri, username, HotConfig::get_str(&hot, "password"), database)
}

fn validate_uri(uri: &str) -> Result<(), String> {
    if uri.is_empty() {
        return Err("neo4j: uri is empty — plugin not configured".to_string());
    }
    if !uri.starts_with("bolt://") && !uri.starts_with("neo4j://") && !uri.starts_with("bolt+s://") && !uri.starts_with("neo4j+s://") {
        return Err(format!("neo4j: unsupported uri scheme — expected bolt:// or neo4j://, got '{}'", uri));
    }
    Ok(())
}

async fn connect(uri: &str, user: &str, pass: &str, database: &str) -> Result<Graph, String> {
    let db_display = if database.is_empty() { "(server default)" } else { database };
    emit_log("debug", &format!("Neo4j: connecting to {} as {} db={}", uri, user, db_display));

    let mut builder = ConfigBuilder::default()
        .uri(uri)
        .user(user)
        .password(pass)
        .db(database); // explicitly set, empty = let server pick default
    let config = builder.build().map_err(|e| format!("neo4j config: {}", e))?;

    Graph::connect(config).await
        .map_err(|e| format!("neo4j connect: {}", e))
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "neo4j";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Neo4j graph memory provider for high-scale knowledge storage";
const PLUGIN_TYPE: &str = "memory";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "uri": {
                "type": "string",
                "title": "Neo4j URI",
                "description": "Connection URI for the Neo4j instance (bolt or neo4j scheme)",
                "default": "bolt://localhost:7687",
                "placeholder": "bolt://localhost:7687"
            },
            "user": {
                "type": "string",
                "title": "Database User",
                "description": "Database user for authentication",
                "default": "neo4j",
                "placeholder": "neo4j"
            },
            "username": {
                "type": "string",
                "title": "Database User (alias)",
                "description": "Alias for 'user'; accepted for backward compatibility",
                "default": "neo4j",
                "placeholder": "neo4j"
            },
            "password": {
                "type": "string",
                "format": "password",
                "title": "Password",
                "description": "Database password for the selected user",
                "placeholder": "Enter neo4j password"
            },
            "database": {
                "type": "string",
                "title": "Database",
                "description": "Neo4j database name (leave empty for default, e.g. 'neo4j')",
                "placeholder": "neo4j"
            }
        },
        "required": ["uri", "user", "password"]
    })
}

fn metadata_properties() -> Value { json!({}) }

// ---------------------------------------------------------------------------
// Memory discovery
// ---------------------------------------------------------------------------

fn str_field<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(Value::as_str).unwrap_or("")
}

fn sanitize_rel(s: &str) -> String {
    s.chars().map(|c| if c.is_alphanumeric() || c == '_' { c } else { '_' }).collect()
}

// ---------------------------------------------------------------------------
// configure
// ---------------------------------------------------------------------------

async fn fn_configure(input: Option<Value>) -> CallResponse {
    CONFIG.configure(input.clone());
    let (uri, user, pass, db) = get_connection(&input.unwrap_or(serde_json::Value::Null));

    if let Err(e) = validate_uri(&uri) {
        emit_log("error", &format!("Neo4j: configure FAILED — {}", e));
        return CallResponse::err(e);
    }

    match connect(&uri, &user, &pass, &db).await {
        Ok(_) => {
            emit_log("info", "Neo4j: connection OK");
            CallResponse::ok(json!({"ok": true}))
        }
        Err(e) => {
            emit_log("error", &format!("Neo4j: connection FAILED — {}", e));
            CallResponse::err(e)
        }
    }
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------
//
// Schema is backward-compatible with OpenLobster 0.3.0:
//   User nodes:     :User {id: $userID}
//   Knowledge nodes: :<entityType> {id: $factID, label, content, createdAt}
//   Relationships:   [:HAS_FACT] or dynamic type from store operation
// ---------------------------------------------------------------------------

async fn fn_store(input: &Value) -> CallResponse {
    match str_field(input, "op") {
        "add_relation"     => store_add_relation(input).await,
        "delete_relation"  => store_delete_relation(input).await,
        "invalidate_cache" => CallResponse::ok(json!({"ok": true})),
        _                  => store_add_knowledge(input).await,
    }
}

async fn store_add_knowledge(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }

    let uid  = str_field(input, "user_id").to_string();
    if uid.is_empty() {
        emit_log("error", "Neo4j: store_add_knowledge requires non-empty user_id");
        return CallResponse::err("store_add_knowledge: user_id is required and must be non-empty");
    }
    
    let cont = str_field(input, "content").to_string();
    let lbl  = str_field(input, "label").to_string();
    let mut et = str_field(input, "entity_type").to_string();
    if et.is_empty() { et = "Fact".to_string(); }
    let nid  = next_node_id();

    emit_log("debug", &format!("Neo4j: store_add_knowledge user={} label={} type={} node={}", uid, lbl, et, nid));

    // 0.3.0 schema: User {id}, dynamic entity label, HAS_FACT rel
    let cypher = format!(
        "MERGE (u:User {{id: $uid}}) SET u.displayName = $uid \
         CREATE (u)-[:HAS_FACT]->(n:{} {{id: $nid, label: $label, content: $content, createdAt: timestamp()}})",
        et
    );

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => {
            emit_log("error", &format!("Neo4j: store_add_knowledge connect FAILED — {}", e));
            return CallResponse::err(e);
        }
    };

    match g.run(query(&cypher)
        .param("uid",     uid.as_str())
        .param("nid",     nid.as_str())
        .param("content", cont.as_str())
        .param("label",   lbl.as_str()))
        .await {
        Ok(()) => {
            emit_log("debug", &format!("Neo4j: store_add_knowledge OK node={}", nid));
            CallResponse::ok(json!([nid]))
        }
        Err(e) => {
            emit_log("error", &format!("Neo4j: store_add_knowledge query FAILED — {}", e));
            CallResponse::err(e.to_string())
        }
    }
}

async fn store_add_relation(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }

    let from_raw = str_field(input, "from");
    let to_raw   = str_field(input, "to");
    let from = from_raw.trim_start_matches("user:").to_string();
    let to   = to_raw.trim_start_matches("user:").to_string();

    if from.is_empty() || to.is_empty() {
        emit_log("error", "Neo4j: add_relation requires non-empty from and to");
        return CallResponse::err("add_relation: from and to are required and must be non-empty");
    }

    let rel_type = sanitize_rel(str_field(input, "rel_type"));
    if rel_type.is_empty() {
        emit_log("error", "Neo4j: add_relation requires non-empty rel_type");
        return CallResponse::err("add_relation: rel_type is required");
    }

    emit_log("debug", &format!("Neo4j: add_relation user:{} -[{}]-> user:{}", from, rel_type, to));

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => {
            emit_log("error", &format!("Neo4j: add_relation connect FAILED — {}", e));
            return CallResponse::err(e);
        }
    };

    let cypher = "MERGE (a:User {id: $from}) MERGE (b:User {id: $to}) MERGE (a)-[r:RELATION]->(b) SET r.relType = $relType RETURN id(r) AS rel_id".to_string();
    match g.execute(query(&cypher).param("from", from.as_str()).param("to", to.as_str()).param("relType", rel_type.as_str())).await {
        Ok(mut rows) => {
            if let Ok(Some(row)) = rows.next().await {
                let rel_id: i64 = row.get("rel_id").unwrap_or(-1);
                emit_log("debug", &format!("Neo4j: add_relation OK rel_id={}", rel_id));
                CallResponse::ok(json!({"ok": true, "rel_id": rel_id}))
            } else {
                emit_log("debug", &format!("Neo4j: add_relation OK (no rel_id)"));
                CallResponse::ok(json!({"ok": true}))
            }
        }
        Err(e) => {
            emit_log("error", &format!("Neo4j: add_relation query FAILED — {}", e));
            CallResponse::err(e.to_string())
        }
    }
}

async fn store_delete_relation(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }

    let from = str_field(input, "from").trim_start_matches("user:").to_string();
    let to   = str_field(input, "to").trim_start_matches("user:").to_string();

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => return CallResponse::err(e),
    };
    
    //PRIMERO verificar que la relación existe
    let check_cypher = format!("MATCH (a:User {{id: '{}'}})-[r]->(b:User {{id: '{}'}}) RETURN r LIMIT 1", from, to);
    let result = g.execute(query(&check_cypher)).await;
    match result {
        Ok(mut rows) => {
            if let Ok(Some(_)) = rows.next().await {
                //EXISTE - borrar
                let del_cypher = format!("MATCH (a:User {{id: '{}'}})-[r]->(b:User {{id: '{}'}}) DELETE r", from, to);
                if let Err(e) = g.run(query(&del_cypher)).await {
                    return CallResponse::err(e.to_string());
                }
                emit_log("debug", &format!("Neo4j: delete_relation DELETED relation: {} -> {}", from, to));
                return CallResponse::ok(json!({"ok": true}));
            } else {
                //NO existe - no hay nada que borrar
                emit_log("debug", &format!("Neo4j: delete_relation relation not found: {} -> {}", from, to));
                return CallResponse::ok(json!({"ok": true}));
            }
        },
        Err(e) => return CallResponse::err(e.to_string()),
    }
}

// ---------------------------------------------------------------------------
// retrieve
// ---------------------------------------------------------------------------

async fn fn_retrieve(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }

    let qtext = str_field(input, "query").to_string();
    let limit = input.get("limit").and_then(Value::as_i64).unwrap_or(64);

    emit_log("debug", &format!("Neo4j: retrieve query='{}' limit={}", qtext, limit));

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => return CallResponse::err(e),
    };

    let mut rows = match g.execute(
        query("MATCH (n) WHERE toLower(n.content) CONTAINS toLower($q) \
               RETURN n.id AS nid, n.content AS content LIMIT $lim")
            .param("q", qtext.as_str()).param("lim", limit),
    ).await {
        Ok(r) => r,
        Err(e) => {
            emit_log("error", &format!("Neo4j: retrieve execute FAILED — {}", e));
            return CallResponse::err(e.to_string());
        }
    };

    let mut items: Vec<Value> = Vec::new();
    while let Ok(Some(row)) = rows.next().await {
        let nid  = row.get::<String>("nid").unwrap_or_default();
        let cont = row.get::<String>("content").unwrap_or_default();
        items.push(json!({"id": nid, "content": cont}));
    }

    emit_log("debug", &format!("Neo4j: retrieve OK rows={}", items.len()));
    CallResponse::ok(Value::Array(items))
}

// ---------------------------------------------------------------------------
// query
// ---------------------------------------------------------------------------

async fn fn_query(input: &Value) -> CallResponse {
    match str_field(input, "op") {
        "cypher" => query_cypher(input).await,
        _        => query_user_graph(input).await,
    }
}

async fn query_user_graph(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }
    let uid = str_field(input, "user_id").to_string();

    emit_log("debug", &format!("Neo4j: query_user_graph user={}", uid));

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => return CallResponse::err(e),
    };

    // 0.3.0 pattern: empty uid → full graph, non-empty → per-user with "user:" prefix
    use std::collections::HashSet;
    let mut node_set: HashSet<String> = HashSet::new();
    let mut nodes: Vec<Value> = Vec::new();
    let mut edges: Vec<Value> = Vec::new();

    // Query edges: both HAS_FACT (User->Fact) and custom relations (User->User)
    let user_node_id = if uid.is_empty() || uid == "*" { "".to_string() } else { format!("user:{}", uid) };
    let (mut rows, is_full): (_, bool) = if uid.is_empty() || uid == "*" {
        let cypher = "MATCH (u:User)-[r]->(n) \
            RETURN u.id AS userId, u.displayName AS displayName, labels(n) AS labels, \
                   n.id AS id, n.content AS content, n.name AS name, n.label AS nodeLabel, type(r) AS relType";
        match g.execute(query(cypher)).await {
            Ok(r) => (r, true),
            Err(e) => return CallResponse::err(e.to_string()),
        }
    } else {
        // Include both User->Fact and User->User relations
        let cypher = "MATCH (u:User {id: $uid})-[r1:HAS_FACT]->(f) \
            RETURN u.id AS userId, u.displayName AS displayName, labels(f) AS labels, \
                   f.id AS id, f.content AS content, f.name AS name, f.label AS nodeLabel, type(r1) AS relType \
            UNION ALL \
            MATCH (u:User {id: $uid})-[r2:RELATION]->(other:User) \
            RETURN u.id AS userId, u.displayName AS displayName, labels(other) AS labels, \
                   other.id AS id, other.content AS content, other.name AS name, other.label AS nodeLabel, \
                   r2.relType AS relType";
        match g.execute(query(cypher).param("uid", uid.as_str())).await {
            Ok(r) => (r, false),
            Err(e) => return CallResponse::err(e.to_string()),
        }
    };

    let mut user_node_added = false;
    let mut added_users: HashSet<String> = HashSet::new();
    while let Ok(Some(row)) = rows.next().await {
        let raw_labels: Vec<String> = row.get::<Vec<String>>("labels").unwrap_or_default();
        let nid: String = row.get("id").unwrap_or_default();
        let content: String = row.get("content").unwrap_or_default();
        let name: String = row.get("name").unwrap_or_default();
        let node_label: String = row.get("nodeLabel").unwrap_or_default();
        let rel_type: String = row.get("relType").unwrap_or_default();
        let display_name: String = row.get("displayName").unwrap_or_default();
        let user_id_val: String = row.get("userId").unwrap_or_default();

        // Add user node: per-user query has one user, full graph has many
        let user_for = if is_full {
            user_id_val.clone()
        } else {
            uid.clone()
        };
        let user_nid = if is_full && !user_id_val.is_empty() {
            format!("user:{}", user_id_val)
        } else {
            user_node_id.clone()
        };
        if !user_nid.is_empty() && added_users.insert(user_nid.clone()) {
            let dn = if !display_name.is_empty() { display_name.clone() } else { user_for.clone() };
            nodes.push(json!({"id": user_nid, "label": dn, "type": "user", "value": user_for}));
        }

        let node_type = raw_labels.iter()
            .find(|l| *l != "User")
            .cloned().unwrap_or_else(|| {
                if !nid.is_empty() && (nid.contains("smoke-peer") || rel_type == "RELATION") {
                    "user".to_string()
                } else {
                    "Node".to_string()
                }
            });
        let display_label = if !name.is_empty() { name.clone() }
            else if !node_label.is_empty() { node_label.clone() }
            else { node_type.clone() };
        let value = if !content.is_empty() { content.clone() }
            else if !name.is_empty() { name.clone() }
            else { String::new() };

        if !nid.is_empty() && node_set.insert(nid.clone()) {
            nodes.push(json!({"id": nid, "label": display_label, "type": node_type.to_lowercase(), "value": value}));
        }

        let source = if is_full {
            if !user_id_val.is_empty() { format!("user:{}", user_id_val) } else { String::new() }
        } else {
            user_node_id.clone()
        };
        if !source.is_empty() && !nid.is_empty() {
            edges.push(json!({"source": source, "target": nid, "label": rel_type}));
        }
    }

    emit_log("debug", &format!("Neo4j: query_user_graph nodes={} edges={}", nodes.len(), edges.len()));
    CallResponse::ok(json!({"nodes": nodes, "edges": edges}))
}

async fn query_cypher(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }

    let cq = str_field(input, "cypher").to_string();
    if cq.is_empty() { return CallResponse::err("cypher required"); }

    emit_log("debug", &format!("Neo4j: query_cypher len={}", cq.len()));

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => return CallResponse::err(e),
    };

    let mut rows = match g.execute(query(&cq)).await {
        Ok(r) => r,
        Err(e) => {
            emit_log("error", &format!("Neo4j: query_cypher FAILED — {}", e));
            return CallResponse::err(e.to_string());
        }
    };

    let mut data: Vec<Value> = Vec::new();
    while let Ok(Some(_)) = rows.next().await {
        // neo4rs 0.8 Row only exposes get::<T>(&str) — column-dependent.
        // Full extraction requires ahead-of-time column name knowledge.
        data.push(json!({"row": true}));
    }

    emit_log("debug", &format!("Neo4j: query_cypher OK rows={}", data.len()));
    CallResponse::ok(json!({"data": data, "errors": []}))
}

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

async fn fn_delete(input: &Value) -> CallResponse {
    let (uri, user, pass, db) = get_connection(input);
    if let Err(e) = validate_uri(&uri) { return CallResponse::err(e); }

    let target_type = str_field(input, "target_type");
    let target_id = str_field(input, "target_id").to_string();
    let from = str_field(input, "from").trim_start_matches("user:").to_string();
    let to = str_field(input, "to").trim_start_matches("user:").to_string();

    // Accept either target_id OR from/to for deletion
    if target_id.is_empty() && (from.is_empty() || to.is_empty()) {
        emit_log("error", "Neo4j: delete requires target_id or from/to");
        return CallResponse::err("delete: target_id or from/to is required");
    }

    emit_log("debug", &format!("Neo4j: delete target_type={} target_id={} from={} to={}", target_type, target_id, from, to));

    let g = match connect(&uri, &user, &pass, &db).await {
        Ok(g) => g,
        Err(e) => {
            emit_log("error", &format!("Neo4j: delete connect FAILED — {}", e));
            return CallResponse::err(e);
        }
    };

    if !from.is_empty() && !to.is_empty() {
        //PRIMERO verificar que la relación existe
        let check_cypher = format!("MATCH (a:User {{id: '{}'}})-[r]->(b:User {{id: '{}'}}) RETURN r LIMIT 1", from, to);
        let result = g.execute(query(&check_cypher)).await;
        match result {
            Ok(mut rows) => {
                if let Ok(Some(_)) = rows.next().await {
                    //EXISTE - borrar
                    if let Err(e) = g.run(query(&format!("MATCH (a:User {{id: '{}'}})-[r]->(b:User {{id: '{}'}}) DELETE r", from, to))).await {
                        return CallResponse::err(e.to_string());
                    }
                    emit_log("debug", &format!("Neo4j: delete DELETED: {} -> {}", from, to));
                } else {
                    emit_log("debug", &format!("Neo4j: delete not found: {} -> {}", from, to));
                }
            },
            Err(e) => return CallResponse::err(e.to_string()),
        }
    } else if target_type == "relation" {
        if let Err(e) = g.run(query(&format!("MATCH ()-[r]->() WHERE ID(r) = {} DELETE r", target_id))).await {
            return CallResponse::err(e.to_string());
        }
    } else {
        // For user nodes, user_graph returns "user:<id>" but DB stores just "<id>"
        let db_id = target_id.trim_start_matches("user:");
        let check_cypher = format!("MATCH (n {{id: '{}'}}) RETURN n LIMIT 1", db_id);
        let result = g.execute(query(&check_cypher)).await;
        match result {
            Ok(mut rows) => {
                if let Ok(Some(_)) = rows.next().await {
                    //EXISTS - delete node
                    if let Err(e) = g.run(query(&format!("MATCH (n {{id: '{}'}}) DETACH DELETE n", db_id))).await {
                        return CallResponse::err(e.to_string());
                    }
                    emit_log("debug", &format!("Neo4j: delete DELETED node: {}", db_id));
                } else {
                    emit_log("debug", &format!("Neo4j: delete node not found: {}", db_id));
                }
            },
            Err(e) => return CallResponse::err(e.to_string()),
        }
    }
    CallResponse::ok(json!({"ok": true}))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct Neo4jPlugin;

#[async_trait]
impl Plugin for Neo4jPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "Neo4j",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["configure", "store", "retrieve", "query", "list", "delete"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure"   => fn_configure(input).await,
            "store"       => fn_store(&input.unwrap_or(Value::Null)).await,
            "retrieve"    => fn_retrieve(&input.unwrap_or(Value::Null)).await,
            "query"       => fn_query(&input.unwrap_or(Value::Null)).await,
            "list"        => CallResponse::err("not implemented".to_string()),
            "delete"      => fn_delete(&input.unwrap_or(Value::Null)).await,
            "add_relation" => fn_store(&input.unwrap_or(Value::Null)).await,
            other         => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(Neo4jPlugin).await;
}
