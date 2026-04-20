// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster GML (Graph Memory Language) memory plugin (Rust).
//!
//! In-memory graph store: knowledge nodes + user-to-user relations.
//! Optionally persists to a .gml file when `path` is configured.

use async_trait::async_trait;
use openlobster_sdk_base::{run, CallResponse, HotConfig, Plugin, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::fs;
use std::io::{Read, Write};
use std::sync::{Arc, LazyLock, Mutex};

mod gml_serde;

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "lowercase")]
struct KnowledgeNode {
    id: u64,
    user_id: String,
    content: String,
    label: String,
    entity_type: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
#[serde(rename_all = "lowercase")]
struct Relation {
    from: String,
    to: String,
    label: String,
}

#[derive(Serialize, Deserialize, Debug)]
#[serde(rename_all = "lowercase")]
struct Graph {
    #[serde(default = "default_directed")]
    directed: i32,
    node: Vec<KnowledgeNode>,
    edge: Vec<Relation>,
}

fn default_directed() -> i32 { 1 }

// ---------------------------------------------------------------------------
// Global state (graph data only)
// ---------------------------------------------------------------------------

struct GraphState {
    nodes: Vec<KnowledgeNode>,
    relations: Vec<Relation>,
    next_id: u64,
}

impl GraphState {
    fn new() -> Self {
        Self { nodes: Vec::new(), relations: Vec::new(), next_id: 1 }
    }

    fn clear(&mut self) {
        self.nodes.clear();
        self.relations.clear();
        self.next_id = 1;
    }

    fn resolve_path(path: &str) -> String {
        if !path.is_empty() {
            return path.to_string();
        }
        // Default fallback to ~/.openlobster/data/memory.gml
        if let Ok(home) = std::env::var("HOME") {
            let dir = format!("{}/.openlobster/data", home);
            let _ = fs::create_dir_all(&dir);
            format!("{}/memory.gml", dir)
        } else {
            "memory.gml".to_string() // Absolute fallback to current dir
        }
    }

    fn save(&self, path: &str) {
        let effective_path = Self::resolve_path(path);
        
        let graph = Graph {
            directed: 1,
            node: self.nodes.clone(),
            edge: self.relations.clone(),
        };

        if let Ok(gml_string) = gml_serde::to_string(&graph) {
            if let Ok(mut file) = fs::File::create(effective_path) {
                let _ = file.write_all(gml_string.as_bytes());
            }
        }
    }

    fn load(&mut self, path: &str) {
        let effective_path = Self::resolve_path(path);
        let mut content = String::new();
        if let Ok(mut file) = fs::File::open(effective_path) {
            if file.read_to_string(&mut content).is_err() { return; }
        } else {
            return;
        }

        if content.is_empty() { return; }

        if let Ok(graph) = gml_serde::from_str::<Graph>(&content) {
            self.clear();
            self.nodes = graph.node;
            self.relations = graph.edge;
            for n in &self.nodes {
                if n.id >= self.next_id { self.next_id = n.id + 1; }
            }
        }
    }
}

static STATE: LazyLock<Arc<Mutex<GraphState>>> =
    LazyLock::new(|| Arc::new(Mutex::new(GraphState::new())));

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "gml";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Lightweight Graph Memory Layer (GML) for persistent knowledge storage";
const PLUGIN_TYPE: &str = "memory";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "title": "GML Storage Path",
                "description": "Local path for graph persistence. Defaults to ~/.openlobster/data/memory.gml",
                "placeholder": "~/.openlobster/data/memory.gml"
            }
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
    state.relations.push(Relation {
        from,
        to,
        label: rel_type,
    });
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
        .map(|r| json!({"source": r.from, "target": r.to, "label": r.label}))
        .collect();

    CallResponse::ok(json!({"edges": edges}))
}

fn query_cypher(_input: &Value) -> CallResponse {
    let state = STATE.lock().unwrap();

    let data: Vec<Value> = if !state.relations.is_empty() {
        state.relations.iter()
            .map(|r| json!({"a": r.from, "r": r.label, "b": r.to}))
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
            id: PLUGIN_ID,
            name: "GML",
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
            "configure" => {
                let res = CONFIG.configure(input.clone());
                let hot = CONFIG.merge(None);
                let path = HotConfig::get_str(&hot, "path");
                if !path.is_empty() {
                  let mut state = STATE.lock().unwrap();
                  state.load(&path);
                }
                res
            },
            "store"     => {
              let res = fn_store(&input.unwrap_or(Value::Null));
              let hot = CONFIG.merge(None);
              let path = HotConfig::get_str(&hot, "path");
              if !path.is_empty() {
                let state = STATE.lock().unwrap();
                state.save(&path);
              }
              res
            },
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
