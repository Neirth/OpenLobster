// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster GML (Graph Memory Language) memory plugin (Rust).
//!
//! In-memory graph store: knowledge nodes + user-to-user relations.
//! Optionally persists to a .gml file when `path` is configured.

use async_trait::async_trait;
use openlobster_sdk_base::{run, CallResponse, HotConfig, Plugin, PluginInfo};
use serde_json::{json, Value};
use std::sync::{Arc, LazyLock, Mutex};

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

struct KnowledgeNode {
    id: u64,
    user_id: String,
    content: String,
    label: String,
    entity_type: String,
}

struct Relation {
    from: String,
    to: String,
    rel_type: String,
}

// ---------------------------------------------------------------------------
// Global state (graph data only)
// ---------------------------------------------------------------------------

struct GraphState {
    nodes: Vec<KnowledgeNode>,
    relations: Vec<Relation>,
    next_id: u64,
}

impl GraphState {
    fn new() -> Self { Self { nodes: Vec::new(), relations: Vec::new(), next_id: 1 } }
}

static STATE: LazyLock<Arc<Mutex<GraphState>>> =
    LazyLock::new(|| Arc::new(Mutex::new(GraphState::new())));

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "openlobster-memory-gml-rust";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Graph Memory Language in-memory store (Rust)";
const PLUGIN_TYPE: &str = "memory";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "path": {"type": "string", "description": "Optional path to .gml persistence file"}
        }
    })
}

fn metadata_properties() -> Value { json!({}) }

// ---------------------------------------------------------------------------
// Plugin discovery
// ---------------------------------------------------------------------------

fn str_field<'a>(v: &'a Value, key: &str) -> &'a str {
    v.get(key).and_then(Value::as_str).unwrap_or("")
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

fn fn_store(input: &Value) -> CallResponse {
    let op = str_field(input, "op");
    match op {
        "add_relation"     => store_add_relation(input),
        "delete_relation"  => store_delete_relation(input),
        "invalidate_cache" => CallResponse::ok(json!({"ok": true})),
        _                  => store_add_knowledge(input),
    }
}

fn store_add_knowledge(input: &Value) -> CallResponse {
    let user_id     = str_field(input, "user_id").to_string();
    let content     = str_field(input, "content").to_string();
    let label       = str_field(input, "label").to_string();
    let entity_type = str_field(input, "entity_type").to_string();

    let mut state = STATE.lock().unwrap();
    let node_id = state.next_id;
    state.next_id += 1;
    state.nodes.push(KnowledgeNode { id: node_id, user_id, content, label, entity_type });
    CallResponse::ok(json!([node_id.to_string()]))
}

fn store_add_relation(input: &Value) -> CallResponse {
    let from     = str_field(input, "from").to_string();
    let to       = str_field(input, "to").to_string();
    let rel_type = str_field(input, "rel_type").to_string();

    if from.is_empty() || to.is_empty() {
        return CallResponse::err("from and to are required for add_relation");
    }
    let mut state = STATE.lock().unwrap();
    state.relations.push(Relation { from, to, rel_type });
    CallResponse::ok(json!({"ok": true}))
}

fn store_delete_relation(input: &Value) -> CallResponse {
    let from = str_field(input, "from");
    let to   = str_field(input, "to");

    let norm = |s: &str| s.trim_start_matches("user:").to_string();
    let from_n = norm(from);
    let to_n   = norm(to);

    let mut state = STATE.lock().unwrap();
    state.relations.retain(|r| !(norm(&r.from) == from_n && norm(&r.to) == to_n));
    CallResponse::ok(json!({"ok": true}))
}

// ---------------------------------------------------------------------------
// retrieve
// ---------------------------------------------------------------------------

fn fn_retrieve(input: &Value) -> CallResponse {
    let query = str_field(input, "query").to_lowercase();
    let limit = input.get("limit").and_then(Value::as_u64).unwrap_or(64) as usize;

    let state = STATE.lock().unwrap();
    let results: Vec<Value> = state.nodes.iter()
        .filter(|n| n.content.to_lowercase().contains(&query))
        .take(limit)
        .map(|n| json!({"id": n.id.to_string(), "content": n.content}))
        .collect();

    CallResponse::ok(Value::Array(results))
}

// ---------------------------------------------------------------------------
// query
// ---------------------------------------------------------------------------

fn fn_query(input: &Value) -> CallResponse {
    let op = str_field(input, "op");
    match op {
        "user_graph" => query_user_graph(input),
        "cypher"     => query_cypher(input),
        _            => query_user_graph(input),
    }
}

fn query_user_graph(input: &Value) -> CallResponse {
    let user_id = str_field(input, "user_id");
    let norm = |s: &str| s.trim_start_matches("user:").to_string();
    let uid = norm(user_id);

    let state = STATE.lock().unwrap();
    let edges: Vec<Value> = state.relations.iter()
        .filter(|r| norm(&r.from) == uid || norm(&r.to) == uid)
        .map(|r| json!({"source": r.from, "target": r.to, "label": r.rel_type}))
        .collect();

    CallResponse::ok(json!({"edges": edges}))
}

fn query_cypher(_input: &Value) -> CallResponse {
    let state = STATE.lock().unwrap();

    let data: Vec<Value> = if !state.relations.is_empty() {
        state.relations.iter()
            .map(|r| json!({"a": r.from, "r": r.rel_type, "b": r.to}))
            .collect()
    } else {
        state.nodes.iter().map(|n| {
            json!({"a": {"id": n.id.to_string(), "content": n.content}, "r": {}, "b": {}})
        }).collect()
    };

    CallResponse::ok(json!({"data": data, "errors": []}))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct GmlPlugin;

#[async_trait]
impl Plugin for GmlPlugin {
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
            "configure" => CONFIG.configure(input),
            "store"     => fn_store(&input.unwrap_or(Value::Null)),
            "retrieve"  => fn_retrieve(&input.unwrap_or(Value::Null)),
            "query"     => fn_query(&input.unwrap_or(Value::Null)),
            "list"      => CallResponse::err("not implemented"),
            "delete"    => CallResponse::err("not implemented"),
            other       => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(GmlPlugin).await;
}
