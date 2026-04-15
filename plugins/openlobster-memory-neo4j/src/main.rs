// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Neo4j memory plugin (Rust).
//!
//! Uses neo4rs 0.8 (Bolt 5.x, Neo4j 5 compatible).

use async_trait::async_trait;
use neo4rs::{query, Graph};
use openlobster_sdk_base::{run, CallResponse, HotConfig, Plugin, PluginInfo};
use serde_json::{json, Value};
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

fn get_connection() -> (String, String, String) {
    let hot = CONFIG.merge(None);
    let uri  = HotConfig::get_str(&hot, "uri");
    let user = HotConfig::get_str(&hot, "username");
    let pass = HotConfig::get_str(&hot, "password");
    (
        if uri.is_empty()  { "bolt://localhost:7687".to_string() } else { uri },
        if user.is_empty() { "neo4j".to_string() } else { user },
        pass,
    )
}

async fn connect(uri: &str, user: &str, pass: &str) -> Result<Graph, String> {
    Graph::new(uri, user, pass).await.map_err(|e| format!("neo4j connect: {}", e))
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "openlobster-memory-neo4j-rust";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Neo4j memory plugin (Rust, neo4rs 0.8)";
const PLUGIN_TYPE: &str = "memory";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "uri":      {"type": "string"},
            "username": {"type": "string"},
            "password": {"type": "string"}
        }
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
    CONFIG.configure(input);
    let (uri, user, pass) = get_connection();
    match connect(&uri, &user, &pass).await {
        Ok(_)  => CallResponse::ok(json!({"ok": true})),
        Err(e) => CallResponse::err(e),
    }
}

// ---------------------------------------------------------------------------
// store
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
    let (uri, user, pass) = get_connection();
    let uid  = str_field(input, "user_id").to_string();
    let cont = str_field(input, "content").to_string();
    let lbl  = str_field(input, "label").to_string();
    let et   = str_field(input, "entity_type").to_string();
    let nid  = next_node_id();

    let g = match connect(&uri, &user, &pass).await {
        Ok(g)  => g,
        Err(e) => return CallResponse::err(e),
    };
    match g.run(query(
        "MERGE (u:User {user_id:$uid}) \
         CREATE (u)-[:HAS_FACT]->(n:Memory { \
             node_id:$nid, content:$content, label:$label, \
             entity_type:$etype, created_at:timestamp() })"
    )
    .param("uid",     uid.as_str())
    .param("nid",     nid.as_str())
    .param("content", cont.as_str())
    .param("label",   lbl.as_str())
    .param("etype",   et.as_str()))
    .await {
        Ok(()) => CallResponse::ok(json!([nid])),
        Err(e) => CallResponse::err(e.to_string()),
    }
}

async fn store_add_relation(input: &Value) -> CallResponse {
    let (uri, user, pass) = get_connection();
    let from     = str_field(input, "from").to_string();
    let to       = str_field(input, "to").to_string();
    let rel_type = sanitize_rel(str_field(input, "rel_type"));
    if from.is_empty() || to.is_empty() { return CallResponse::err("from/to required"); }

    let g = match connect(&uri, &user, &pass).await {
        Ok(g)  => g,
        Err(e) => return CallResponse::err(e),
    };
    let cypher = format!("MERGE (a:User {{user_id:$from}}) MERGE (b:User {{user_id:$to}}) CREATE (a)-[:{rel_type}]->(b)");
    match g.run(query(&cypher).param("from", from.as_str()).param("to", to.as_str())).await {
        Ok(()) => CallResponse::ok(json!({"ok": true})),
        Err(e) => CallResponse::err(e.to_string()),
    }
}

async fn store_delete_relation(input: &Value) -> CallResponse {
    let (uri, user, pass) = get_connection();
    let from = str_field(input, "from").trim_start_matches("user:").to_string();
    let to   = str_field(input, "to").trim_start_matches("user:").to_string();

    let g = match connect(&uri, &user, &pass).await {
        Ok(g)  => g,
        Err(e) => return CallResponse::err(e),
    };
    match g.run(query("MATCH (a:User {user_id:$from})-[r]->(b:User {user_id:$to}) DELETE r")
        .param("from", from.as_str()).param("to", to.as_str())).await {
        Ok(()) => CallResponse::ok(json!({"ok": true})),
        Err(e) => CallResponse::err(e.to_string()),
    }
}

// ---------------------------------------------------------------------------
// retrieve
// ---------------------------------------------------------------------------

async fn fn_retrieve(input: &Value) -> CallResponse {
    let (uri, user, pass) = get_connection();
    let qtext = str_field(input, "query").to_string();
    let limit = input.get("limit").and_then(Value::as_i64).unwrap_or(64);

    let g = match connect(&uri, &user, &pass).await {
        Ok(g)  => g,
        Err(e) => return CallResponse::err(e),
    };
    let mut rows = match g.execute(
        query("MATCH (n:Memory) WHERE toLower(n.content) CONTAINS toLower($q) \
               RETURN n.node_id AS nid, n.content AS content LIMIT $lim")
            .param("q", qtext.as_str()).param("lim", limit),
    ).await {
        Ok(r)  => r,
        Err(e) => return CallResponse::err(e.to_string()),
    };

    let mut items: Vec<Value> = Vec::new();
    while let Ok(Some(row)) = rows.next().await {
        let nid  = row.get::<String>("nid").unwrap_or_default();
        let cont = row.get::<String>("content").unwrap_or_default();
        items.push(json!({"id": nid, "content": cont}));
    }
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
    let (uri, user, pass) = get_connection();
    let uid = str_field(input, "user_id").to_string();

    let g = match connect(&uri, &user, &pass).await {
        Ok(g)  => g,
        Err(e) => return CallResponse::err(e),
    };
    let cypher = "MATCH (u:User {user_id:$uid})-[r]->(n) \
        RETURN type(r) AS rel_type, \
        CASE WHEN n:User THEN n.user_id ELSE '' END AS tgt_uid";
    let mut rows = match g.execute(query(cypher).param("uid", uid.as_str())).await {
        Ok(r)  => r,
        Err(e) => return CallResponse::err(e.to_string()),
    };
    let mut edges: Vec<Value> = Vec::new();
    while let Ok(Some(row)) = rows.next().await {
        let rel = row.get::<String>("rel_type").unwrap_or_default();
        let tgt = row.get::<String>("tgt_uid").unwrap_or_default();
        edges.push(json!({"source": uid, "target": tgt, "label": rel}));
    }
    CallResponse::ok(json!({"edges": edges}))
}

async fn query_cypher(input: &Value) -> CallResponse {
    let (uri, user, pass) = get_connection();
    let cq = str_field(input, "cypher").to_string();
    if cq.is_empty() { return CallResponse::err("cypher required"); }

    let g = match connect(&uri, &user, &pass).await {
        Ok(g)  => g,
        Err(e) => return CallResponse::err(e),
    };
    let mut rows = match g.execute(query(&cq)).await {
        Ok(r)  => r,
        Err(e) => return CallResponse::err(e.to_string()),
    };
    let mut data: Vec<Value> = Vec::new();
    while let Ok(Some(_)) = rows.next().await { data.push(json!({"row": true})); }
    CallResponse::ok(json!({"data": data, "errors": []}))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct Neo4jPlugin;

#[async_trait]
impl Plugin for Neo4jPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID, name: PLUGIN_ID, version: PLUGIN_VERSION,
            description: PLUGIN_DESC, plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(), properties: metadata_properties(),
            exports: vec!["configure", "store", "retrieve", "query", "list", "delete"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure" => fn_configure(input).await,
            "store"     => fn_store(&input.unwrap_or(Value::Null)).await,
            "retrieve"  => fn_retrieve(&input.unwrap_or(Value::Null)).await,
            "query"     => fn_query(&input.unwrap_or(Value::Null)).await,
            "list"      => CallResponse::err("not implemented".to_string()),
            "delete"    => CallResponse::err("not implemented".to_string()),
            other       => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(Neo4jPlugin).await;
}
