// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

use async_trait::async_trait;
use openlobster_sdk_base::{
    run, CallResponse, HotConfig, Plugin, PluginInfo,
};
use serde_json::{json, Value};
use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::sync::Mutex;
use once_cell::sync::Lazy;

// ---------------------------------------------------------------------------
// GML Domain Models
// ---------------------------------------------------------------------------

#[derive(Debug, Clone)]
pub struct Node {
    pub id: u64,
    pub user_id: String,
    pub content: String,
    pub label: String,
    pub entity_type: String,
}

#[derive(Debug, Clone)]
pub struct Relation {
    pub source: String,
    pub target: String,
    pub label: String,
}

// ---------------------------------------------------------------------------
// Plugin State
// ---------------------------------------------------------------------------

struct GraphState {
    nodes: Vec<Node>,
    relations: Vec<Relation>,
    next_id: u64,
}

impl GraphState {
    fn new() -> Self {
        Self {
            nodes: Vec::new(),
            relations: Vec::new(),
            next_id: 1,
        }
    }

    fn resolve_path(path: &str) -> String {
        if path.is_empty() {
             if let Ok(base) = std::env::var("OPENLOBSTER_BASE_DIR") {
                 return format!("{}/data/memory.gml", base);
             }
             return "/data/memory.gml".to_string();
        }
        path.to_string()
    }

    fn save(&self, path: &str) {
        let ep = Self::resolve_path(path);
        let mut f = match fs::File::create(&ep) {
            Ok(f) => f,
            Err(e) => {
                eprintln!("[gml:error] Save error creating file {}: {}", ep, e);
                return;
            }
        };

        writeln!(f, "graph [").unwrap();
        writeln!(f, "  directed 1").unwrap();
        for n in &self.nodes {
            writeln!(f, "  node [").unwrap();
            writeln!(f, "    id {}", n.id).unwrap();
            writeln!(f, "    user_id \"{}\"", n.user_id).unwrap();
            writeln!(f, "    content \"{}\"", n.content.replace("\"", "\\\"")).unwrap();
            writeln!(f, "    label \"{}\"", n.label).unwrap();
            writeln!(f, "    entity_type \"{}\"", n.entity_type).unwrap();
            writeln!(f, "  ]").unwrap();
        }
        for r in &self.relations {
            writeln!(f, "  edge [").unwrap();
            writeln!(f, "    source \"{}\"", r.source).unwrap();
            writeln!(f, "    target \"{}\"", r.target).unwrap();
            writeln!(f, "    label \"{}\"", r.label).unwrap();
            writeln!(f, "  ]").unwrap();
        }
        writeln!(f, "]").unwrap();
        eprintln!("[gml:info] SUCCESSFULLY SAVED {} nodes to {}", self.nodes.len(), ep);
    }

    fn load(&mut self, path: &str) {
        let ep = Self::resolve_path(path);
        if !std::path::Path::new(&ep).exists() {
             eprintln!("[gml:info] Load skipped, file does not exist: {}", ep);
             return;
        }

        let file = match fs::File::open(&ep) {
            Ok(f) => f,
            Err(e) => {
                eprintln!("[gml:error] Load error opening file {}: {}", ep, e);
                return;
            }
        };

        let reader = BufReader::new(file);
        let mut nodes = Vec::new();
        let mut relations = Vec::new();
        
        let mut current_block: Option<String> = None;
        let mut temp_node = json!({});
        let mut temp_edge = json!({});

        for line_res in reader.lines() {
            let line = match line_res {
                Ok(l) => l.trim().to_string(),
                Err(_) => continue,
            };

            if line.ends_with('[') {
                let block_type = line.trim_end_matches('[').trim().to_lowercase();
                current_block = Some(block_type);
                continue;
            }

            if line == "]" {
                if let Some(ref bt) = current_block {
                    if bt == "node" {
                        nodes.push(Node {
                            id: temp_node["id"].as_u64().unwrap_or(0),
                            user_id: temp_node["user_id"].as_str().unwrap_or("").to_string(),
                            content: temp_node["content"].as_str().unwrap_or("").to_string(),
                            label: temp_node["label"].as_str().unwrap_or("").to_string(),
                            entity_type: temp_node["entity_type"].as_str().unwrap_or("").to_string(),
                        });
                        temp_node = json!({});
                    } else if bt == "edge" {
                        relations.push(Relation {
                            source: temp_edge["source"].as_str().unwrap_or("").to_string(),
                            target: temp_edge["target"].as_str().unwrap_or("").to_string(),
                            label: temp_edge["label"].as_str().unwrap_or("").to_string(),
                        });
                        temp_edge = json!({});
                    }
                }
                current_block = None;
                continue;
            }

            // Key value parsing
            if let Some(space_idx) = line.find(' ') {
                let key = line[..space_idx].trim().to_lowercase();
                let value_raw = line[space_idx..].trim();
                let value = if value_raw.starts_with('"') && value_raw.ends_with('"') {
                    value_raw[1..value_raw.len()-1].replace("\\\"", "\"")
                } else {
                    value_raw.to_string()
                };

                if let Some(ref bt) = current_block {
                    if bt == "node" {
                        if key == "id" {
                            if let Ok(id) = value.parse::<u64>() {
                                temp_node["id"] = json!(id);
                            }
                        } else {
                            temp_node[key] = json!(value);
                        }
                    } else if bt == "edge" {
                        temp_edge[key] = json!(value);
                    }
                }
            }
        }

        self.nodes = nodes;
        self.relations = relations;
        self.next_id = self.nodes.iter().map(|n| n.id).max().unwrap_or(0) + 1;
        eprintln!("[gml:info] SUCCESSFULLY LOADED {} nodes from {}", self.nodes.len(), ep);
    }
}

static STATE: Lazy<Mutex<GraphState>> = Lazy::new(|| Mutex::new(GraphState::new()));
static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Plugin Lifecycle
// ---------------------------------------------------------------------------

struct GmlPlugin;

#[async_trait]
impl Plugin for GmlPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: "memory:file",
            name: "GML Memory",
            version: "0.1.0",
            description: "GML-based graph memory plugin",
            plugin_type: "memory",
            schema: json!({
                "type": "object",
                "properties": {
                    "data_dir": { "type": "string" },
                    "path": { "type": "string" }
                }
            }),
            properties: json!({}),
            exports: vec!["configure", "store", "retrieve", "query"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure" => {
                let res = CONFIG.configure(input);
                let hot = CONFIG.merge(None);
                let data_dir = hot.get("data_dir").and_then(Value::as_str).unwrap_or("");
                let mut path = hot.get("path").and_then(Value::as_str).unwrap_or("").to_string();
                if path.is_empty() && !data_dir.is_empty() {
                    path = format!("{}/memory.gml", data_dir);
                }
                
                let mut state = STATE.lock().unwrap();
                state.load(&path);
                res
            },
            "store" | "add_memory" => {
                let input_val = input.unwrap_or(json!({}));
                fn_store(&input_val)
            },
            "retrieve" => {
                let input_val = input.unwrap_or(json!({}));
                fn_retrieve(&input_val)
            },
            "query" => {
                let input_val = input.unwrap_or(json!({}));
                fn_query(&input_val)
            },
            _ => CallResponse::err(format!("unknown function: {}", function)),
        }
    }
}

fn fn_store(input: &Value) -> CallResponse {
    let mut state = STATE.lock().unwrap();
    let uid = str_field(input, "user_id");
    let content = str_field(input, "content");
    let label = str_field(input, "label");
    let etype = str_field(input, "entity_type");

    eprintln!("[gml:info] Storing node for user '{}'. Raw: label='{}', type='{}', content_len={}", uid, label, etype, content.len());

    let node = Node {
        id: state.next_id,
        user_id: uid.to_string(),
        content: content.to_string(),
        label: if label.is_empty() { "fact".to_string() } else { label.to_string() },
        entity_type: if etype.is_empty() { "fact".to_string() } else { etype.to_string() },
    };
    state.next_id += 1;
    state.nodes.push(node.clone());

    let hot = CONFIG.merge(None);
    let mut path = input.get("config").and_then(|c| c.get("path")).and_then(Value::as_str).unwrap_or("").to_string();
    if path.is_empty() {
        path = hot.get("path").and_then(Value::as_str).unwrap_or("").to_string();
    }

    eprintln!("[gml:info] Saving to path '{}'. Node ID: {}", path, node.id);
    state.save(&path);
    CallResponse::ok(json!({"success": true, "id": node.id.to_string()}))
}

fn fn_retrieve(input: &Value) -> CallResponse {
    let query = str_field(input, "query").to_lowercase();
    let limit = input.get("limit").and_then(Value::as_u64).unwrap_or(10) as usize;
    let state = STATE.lock().unwrap();
    let results: Vec<Value> = state.nodes.iter()
        .filter(|n| n.content.to_lowercase().contains(&query))
        .take(limit)
        .map(|n| json!({"id": n.id.to_string(), "content": n.content}))
        .collect();
    CallResponse::ok(Value::Array(results))
}

fn fn_query(input: &Value) -> CallResponse {
    let user_id = input.get("user_id").and_then(Value::as_str).unwrap_or("");
    let hot = CONFIG.merge(None);
    let path_val = input.get("config").and_then(|c| c.get("path")).and_then(Value::as_str).unwrap_or("");
    let mut path = path_val.to_string();
    if path.is_empty() {
         path = hot.get("path").and_then(Value::as_str).unwrap_or("").to_string();
    }

    let mut state = STATE.lock().unwrap();
    if state.nodes.is_empty() && !path.is_empty() {
         state.load(&path);
    }

    eprintln!("[gml:info] Querying User Graph for '{}' via path '{}'. Total nodes in state: {}.", user_id, path, state.nodes.len());

    let nodes: Vec<Value> = state.nodes.iter()
        .filter(|n| user_id.is_empty() || n.user_id == user_id)
        .map(|n| json!({
            "id": n.id.to_string(),
            "label": n.label,
            "type": n.entity_type,
            "value": n.content,
        }))
        .collect();

    let edges: Vec<Value> = state.relations.iter()
        .filter(|r| user_id.is_empty() || r.source.contains(user_id) || r.target.contains(user_id))
        .map(|r| json!({"source": r.source, "target": r.target, "label": r.label}))
        .collect();

    eprintln!("[gml:info] Returning {} nodes and {} edges.", nodes.len(), edges.len());
    CallResponse::ok(json!({"nodes": nodes, "edges": edges}))
}

fn str_field<'a>(v: &'a Value, key: &str) -> &'a str {
    v.get(key).and_then(Value::as_str).unwrap_or("")
}

#[tokio::main]
async fn main() {
    run(GmlPlugin).await;
}
