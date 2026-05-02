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
            version: "0.1.1",
            description: "GML-based graph memory plugin",
            plugin_type: "memory",
            schema: json!({
                "type": "object",
                "properties": {
                    "data_dir": {
                        "type": "string",
                        "title": "Data Directory",
                        "description": "Base directory for GML files (optional)"
                    },
                    "path": {
                        "type": "string",
                        "title": "GML File Path",
                        "description": "Specific path to the .gml file (overrides Data Directory, optional)"
                    }
                }
            }),
            properties: json!({}),
            exports: vec!["configure", "store", "retrieve", "query", "add_relation", "delete"],
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
            "add_relation" => {
                let input_val = input.unwrap_or(json!({}));
                fn_add_relation(&input_val)
            },
            "delete" => {
                let input_val = input.unwrap_or(json!({}));
                fn_delete(&input_val)
            },
            _ => CallResponse::err(format!("unknown function: {}", function)),
        }
    }
}

fn fn_store(input: &Value) -> CallResponse {
    let op = str_field(input, "op");
    if op == "add_relation" {
        return fn_add_relation(input);
    }
    if op == "delete_relation" {
        return fn_delete(input);
    }
    if op == "delete" {
        return fn_delete(input);
    }
    
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
    let op = input.get("op").and_then(Value::as_str).unwrap_or("");
    
    // Handle cypher query operation
    if op == "cypher" {
        let cypher = input.get("cypher").and_then(Value::as_str).unwrap_or("");
        if cypher.is_empty() {
            eprintln!("[gml:error] cypher query requires 'cypher' parameter");
            return CallResponse::err("cypher query requires 'cypher' parameter");
        }
        
        // Very basic GML cypher-like support
        // Parse patterns like "MATCH (a)-[r]->(b) RETURN a, r, b"
        let mut state = STATE.lock().unwrap();
        let mut results: Vec<Value> = Vec::new();
        
        // For now, just return all nodes as "data" for any MATCH
        if cypher.contains("MATCH") {
            for node in &state.nodes {
                results.push(json!({
                    "a": {"id": node.id.to_string(), "label": node.label}
                }));
            }
            // Also return edges
            for rel in &state.relations {
                results.push(json!({
                    "r": {"source": rel.source, "target": rel.target, "label": rel.label}
                }));
            }
        }
        
        eprintln!("[gml:info] cypher query executed, returning {} results", results.len());
        return CallResponse::ok(json!({"data": results, "errors": []}));
    }
    
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

    eprintln!("[gml:info] Querying User Graph for '{}' via path '{}'. Total nodes: {}, relations: {}.", 
        user_id, path, state.nodes.len(), state.relations.len());

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

fn fn_add_relation(input: &Value) -> CallResponse {
    let from_raw = str_field(input, "from");
    let to_raw = str_field(input, "to");
    let from = from_raw.trim_start_matches("user:").to_string();
    let to = to_raw.trim_start_matches("user:").to_string();

    if from.is_empty() || to.is_empty() {
        eprintln!("[gml:error] add_relation requires non-empty from and to");
        return CallResponse::err("add_relation: from and to are required and must be non-empty");
    }

    let rel_type = str_field(input, "rel_type");
    if rel_type.is_empty() {
        eprintln!("[gml:error] add_relation requires non-empty rel_type");
        return CallResponse::err("add_relation: rel_type is required");
    }

    eprintln!("[gml:info] add_relation user:{} -[{}]-> user:{}", from, rel_type, to);

    let mut state = STATE.lock().unwrap();

    let from_normalized = from.trim_start_matches("user:").to_string();
    let to_normalized = to.trim_start_matches("user:").to_string();
    let from_id_str = from_normalized.clone();
    let to_id_str = to_normalized.clone();
    let node_ids: Vec<String> = state.nodes.iter().map(|n| n.user_id.clone()).collect();
    let node_id_nums: Vec<String> = state.nodes.iter().map(|n| n.id.to_string()).collect();

    let from_exists = node_ids.contains(&from_id_str) || node_id_nums.contains(&from_id_str);
    let to_exists = node_ids.contains(&to_id_str) || node_id_nums.contains(&to_id_str);

    let mut new_next_id = state.next_id;

    if !from_exists {
        eprintln!("[gml:debug] auto-creating source node for add_relation: {}", from);
        state.nodes.push(Node {
            id: new_next_id,
            user_id: from_normalized.clone(),
            content: String::new(),
            label: "user".to_string(),
            entity_type: "user".to_string(),
        });
        new_next_id += 1;
    }

    if !to_exists {
        eprintln!("[gml:debug] auto-creating target node for add_relation: {}", to);
        state.nodes.push(Node {
            id: new_next_id,
            user_id: to_normalized.clone(),
            content: String::new(),
            label: "user".to_string(),
            entity_type: "user".to_string(),
        });
        new_next_id += 1;
    }

    state.next_id = new_next_id;

    let rel = Relation {
        source: format!("user:{}", from_normalized),
        target: format!("user:{}", to_normalized),
        label: rel_type.to_string(),
    };
    state.relations.push(rel);

    let hot = CONFIG.merge(None);
    let mut path = input.get("config").and_then(|c| c.get("path")).and_then(Value::as_str).unwrap_or("").to_string();
    if path.is_empty() {
        path = hot.get("path").and_then(Value::as_str).unwrap_or("").to_string();
    }

    state.save(&path);
    eprintln!("[gml:info] add_relation OK");
    CallResponse::ok(json!({"ok": true}))
}

fn fn_delete(input: &Value) -> CallResponse {
    let target_type = str_field(input, "target_type");
    let target_id = str_field(input, "target_id").to_string();
    let from = str_field(input, "from").to_string();
    let to = str_field(input, "to").to_string();
    
    let delete_target = if !target_id.is_empty() { 
        target_id.clone() 
    } else if !from.is_empty() && !to.is_empty() {
        format!("{}->{}", from, to)
    } else {
        "".to_string()
    };

    if delete_target.is_empty() {
        eprintln!("[gml:error] delete requires non-empty target_id or from/to");
        return CallResponse::err("delete: target_id is required");
    }

    eprintln!("[gml:info] delete target_type={} target={}", target_type, delete_target);

    let mut state = STATE.lock().unwrap();

    let mut removed = false;
    if target_type.is_empty() || target_type == "node" {
        let target_normalized = delete_target.trim_start_matches("user:");
        let original_len = state.nodes.len();
        state.nodes.retain(|n| n.id.to_string() != delete_target && n.user_id != target_normalized);
        if state.nodes.len() < original_len {
            removed = true;
            eprintln!("[gml:debug] deleted node target={}", delete_target);
        }
        state.relations.retain(|r| !r.source.contains(&delete_target) && !r.target.contains(&delete_target));
        if !from.is_empty() && !to.is_empty() {
            let from_norm = from.trim_start_matches("user:");
            let to_norm = to.trim_start_matches("user:");
            let original_rels = state.relations.len();
            state.relations.retain(|r| {
                !(r.source.contains(from_norm) && r.target.contains(to_norm))
            });
            if state.relations.len() < original_rels {
                removed = true;
                eprintln!("[gml:debug] deleted relation from={} to={}", from, to);
            }
        }
    } else if target_type == "relation" {
        if let Ok(rel_id) = delete_target.parse::<usize>() {
            if rel_id < state.relations.len() {
                state.relations.remove(rel_id);
                removed = true;
                eprintln!("[gml:debug] deleted relation index={}", rel_id);
            }
        }
    }

    let hot = CONFIG.merge(None);
    let mut path = input.get("config").and_then(|c| c.get("path")).and_then(Value::as_str).unwrap_or("").to_string();
    if path.is_empty() {
        path = hot.get("path").and_then(Value::as_str).unwrap_or("").to_string();
    }

    state.save(&path);
    if removed {
        eprintln!("[gml:info] delete OK");
        CallResponse::ok(json!({"ok": true}))
    } else {
        eprintln!("[gml:warn] delete target not found: {}", target_id);
        CallResponse::err(format!("delete: target {} not found", target_id))
    }
}

fn str_field<'a>(v: &'a Value, key: &str) -> &'a str {
    v.get(key).and_then(Value::as_str).unwrap_or("")
}

#[tokio::main]
async fn main() {
    run(GmlPlugin).await;
}
