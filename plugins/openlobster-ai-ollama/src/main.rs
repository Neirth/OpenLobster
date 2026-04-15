// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Ollama AI provider plugin (Rust).
//!
//! Delegates chat completions to the ollama-rs SDK.

use std::collections::{HashMap, HashSet};

use async_trait::async_trait;
use ollama_rs::generation::chat::request::ChatMessageRequest;
use ollama_rs::generation::chat::ChatMessage;
use ollama_rs::Ollama;
use openlobster_sdk_base::{run, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;

// ---------------------------------------------------------------------------
// Plugin constants
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "openlobster-ai-ollama";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Ollama local AI provider plugin for OpenLobster";
const PLUGIN_TYPE: &str = "ai";

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Business types
// ---------------------------------------------------------------------------

#[derive(Deserialize, Debug, Default)]
#[allow(dead_code)]
struct InputPayload {
    #[serde(default)] model: Option<String>,
    #[serde(default)] messages: Option<Vec<ChatMsg>>,
    #[serde(default)] tools: Option<Vec<Value>>,
    #[serde(default)] max_tokens: Option<u32>,
    #[serde(default)] config: Option<HashMap<String, Value>>,
}

#[derive(Deserialize, Serialize, Debug, Clone)]
struct ChatMsg {
    #[serde(default)] role: String,
    #[serde(default)] content: String,
    #[serde(default, skip_serializing_if = "Option::is_none")] tool_calls: Option<Vec<ToolCallMsg>>,
    #[serde(default, skip_serializing_if = "Option::is_none")] tool_call_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")] tool_name: Option<String>,
}

#[derive(Deserialize, Serialize, Debug, Clone)]
struct ToolCallMsg {
    #[serde(default)] id: String,
    #[serde(default)] r#type: String,
    #[serde(default)] function: ToolCallFunction,
}

#[derive(Deserialize, Serialize, Debug, Clone, Default)]
struct ToolCallFunction {
    #[serde(default)] name: String,
    #[serde(default)] arguments: String,
}

#[derive(Serialize, Debug)]
struct OutputPayload {
    content: String,
    #[serde(skip_serializing_if = "Vec::is_empty")] tool_calls: Vec<ToolCallMsg>,
    stop_reason: String,
    usage: UsagePayload,
    #[serde(skip_serializing_if = "Option::is_none")] error: Option<String>,
}

#[derive(Serialize, Debug, Default)]
struct UsagePayload {
    prompt_tokens: u64,
    completion_tokens: u64,
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "base_url": {
                "type": "string",
                "title": "Base URL",
                "default": "http://localhost:11434",
                "description": "Ollama endpoint (local or remote)"
            },
            "default_model": {
                "type": "string",
                "title": "Default Model",
                "default": "llama3.2",
                "description": "Model used when the request does not specify one"
            },
            "api_key": {
                "type": "string",
                "title": "API Key",
                "description": "Optional Bearer token for protected or cloud Ollama endpoints"
            }
        },
        "required": []
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({"supports_audio_input": false, "supports_audio_output": false})
}

// ---------------------------------------------------------------------------
// Message sanitization
// ---------------------------------------------------------------------------

fn sanitize_messages(messages: &[ChatMsg]) -> Vec<ChatMsg> {
    let mut valid_ids: HashSet<String> = HashSet::new();
    for m in messages {
        if m.role == "assistant" {
            if let Some(tcs) = &m.tool_calls {
                for tc in tcs {
                    let id = tc.id.trim();
                    if !id.is_empty() { valid_ids.insert(id.to_string()); }
                }
            }
        }
    }
    let mut seen: HashSet<String> = HashSet::new();
    let mut out = Vec::with_capacity(messages.len());
    for m in messages {
        if m.role == "tool" {
            let tcid = m.tool_call_id.as_deref().unwrap_or("").trim();
            if tcid.is_empty() || !valid_ids.contains(tcid) || seen.contains(tcid) {
                continue;
            }
            seen.insert(tcid.to_string());
        }
        out.push(m.clone());
    }
    out
}

// ---------------------------------------------------------------------------
// Chat export
// ---------------------------------------------------------------------------

async fn chat(input: Option<Value>) -> CallResponse {
    let payload: InputPayload = input
        .and_then(|v| serde_json::from_value(v).ok())
        .unwrap_or_default();

    let cfg = CONFIG.merge(payload.config);

    let base_url = {
        let v = HotConfig::get_str(&cfg, "base_url");
        if v.is_empty() { "http://localhost:11434".to_string() } else { v }
    };

    let model = {
        let m = payload.model.as_deref().unwrap_or("").trim().to_string();
        if m.is_empty() {
            let dm = HotConfig::get_str(&cfg, "default_model");
            if dm.is_empty() { "llama3.2".to_string() } else { dm }
        } else { m }
    };

    let parsed = match url::Url::parse(&base_url) {
        Ok(u) => u,
        Err(e) => {
            return CallResponse {
                output: Some(serde_json::json!({"error": format!("invalid base_url: {}", e)})),
                error: Some(format!("invalid base_url: {}", e)),
            };
        }
    };
    let host = format!("{}://{}", parsed.scheme(), parsed.host_str().unwrap_or("localhost"));
    let port = parsed.port().unwrap_or(11434);

    let client = Ollama::new(host, port);

    let raw_messages = payload.messages.unwrap_or_default();
    let sanitized = sanitize_messages(&raw_messages);

    let mut ollama_messages: Vec<ChatMessage> = Vec::with_capacity(sanitized.len());
    for m in &sanitized {
        let msg = match m.role.as_str() {
            "system" => ChatMessage::system(m.content.clone()),
            "tool"   => ChatMessage::tool(m.content.clone()),
            "assistant" => {
                let mut msg = ChatMessage::assistant(m.content.clone());
                if let Some(tcs) = &m.tool_calls {
                    for tc in tcs {
                        let args: Value = serde_json::from_str(&tc.function.arguments)
                            .unwrap_or(Value::Object(Default::default()));
                        msg.tool_calls.push(ollama_rs::generation::tools::ToolCall {
                            function: ollama_rs::generation::tools::ToolCallFunction {
                                name: tc.function.name.clone(),
                                arguments: args,
                            },
                        });
                    }
                }
                msg
            }
            _ => ChatMessage::user(m.content.clone()),
        };
        ollama_messages.push(msg);
    }

    let payload_tools: Vec<ollama_rs::generation::tools::ToolInfo> = match payload.tools {
        Some(arr) => arr.into_iter().filter_map(|v| {
            let mut tool_json = v.clone();
            if let Some(t_obj) = tool_json.as_object_mut() {
                let type_val = t_obj.get("type").and_then(|t| t.as_str()).unwrap_or("function");
                let normalized_type = if type_val.eq_ignore_ascii_case("function") { "Function" } else { type_val };
                t_obj.insert("type".to_string(), serde_json::json!(normalized_type));
                if let Some(func_obj) = t_obj.get_mut("function").and_then(|f| f.as_object_mut()) {
                    if !func_obj.contains_key("parameters") {
                        func_obj.insert("parameters".to_string(), serde_json::json!({"type":"object","properties":{}}));
                    }
                    if !func_obj.contains_key("description") {
                        func_obj.insert("description".to_string(), serde_json::json!(""));
                    }
                }
            }
            serde_json::from_value(tool_json).ok()
        }).collect(),
        None => Vec::new(),
    };

    let request = ChatMessageRequest::new(model.clone(), ollama_messages).tools(payload_tools);

    match client.send_chat_messages(request).await {
        Ok(resp) => {
            let content = resp.message.content.clone();
            let (prompt_tokens, completion_tokens) = resp.final_data.as_ref()
                .map(|fd| (fd.prompt_eval_count, fd.eval_count))
                .unwrap_or((0, 0));

            let mut out_tool_calls: Vec<ToolCallMsg> = Vec::new();
            for (i, tc) in resp.message.tool_calls.into_iter().enumerate() {
                out_tool_calls.push(ToolCallMsg {
                    id: format!("call_{}", i),
                    r#type: "function".to_string(),
                    function: ToolCallFunction {
                        name: tc.function.name,
                        arguments: serde_json::to_string(&tc.function.arguments).unwrap_or_default(),
                    },
                });
            }

            let stop_reason = if !out_tool_calls.is_empty() {
                "tool_use".to_string()
            } else {
                "stop".to_string()
            };

            CallResponse::ok(OutputPayload {
                content, tool_calls: out_tool_calls, stop_reason,
                usage: UsagePayload { prompt_tokens, completion_tokens },
                error: None,
            })
        }
        Err(e) => {
            let msg = format!("ollama chat request failed: model={} base_url={}: {}", model, base_url, e);
            CallResponse {
                output: Some(serde_json::to_value(OutputPayload {
                    content: String::new(), tool_calls: Vec::new(), stop_reason: String::new(),
                    usage: UsagePayload::default(), error: Some(msg),
                }).unwrap_or_default()),
                error: Some(e.to_string()),
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct OllamaPlugin;

#[async_trait]
impl Plugin for OllamaPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: PLUGIN_ID,
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["chat", "configure"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure" => CONFIG.configure(input),
            "chat"      => chat(input).await,
            other       => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(OllamaPlugin).await;
}
