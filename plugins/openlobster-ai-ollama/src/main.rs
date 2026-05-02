// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Ollama AI provider plugin (Rust).
//!
//! Delegates chat completions to the ollama-rs SDK.

use std::collections::{HashMap, HashSet};

use async_trait::async_trait;
use ollama_rs::generation::chat::request::ChatMessageRequest;
use ollama_rs::generation::chat::ChatMessage;
use ollama_rs::generation::images::Image;
use ollama_rs::Ollama;
use openlobster_sdk_base::{run, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;

// ---------------------------------------------------------------------------
// Plugin constants
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "ollama";
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
    #[serde(default, skip_serializing_if = "Option::is_none")] blocks: Option<Vec<ContentBlock>>,
    #[serde(default, skip_serializing_if = "Option::is_none")] tool_calls: Option<Vec<ToolCallMsg>>,
    #[serde(default, skip_serializing_if = "Option::is_none")] tool_call_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")] tool_name: Option<String>,
}

#[derive(Deserialize, Serialize, Debug, Clone)]
struct ContentBlock {
    r#type: String,
    #[serde(default)] text: Option<String>,
    #[serde(default)] data: Option<String>,
    #[serde(default)] mime_type: Option<String>,
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
            "endpoint": {
                "type": "string",
                "title": "Endpoint",
                "description": "Ollama API endpoint (e.g., http://localhost:11434)",
                "default": "http://localhost:11434",
                "placeholder": "http://localhost:11434"
            },
            "default_model": {
                "type": "string",
                "title": "Default Model",
                "description": "The local Ollama model to use when the request omits one",
                "default": "llama3.2",
                "placeholder": "llama3.2"
            },
            "api_key": {
                "type": "string",
                "title": "API Key (Optional)",
                "description": "Optional Bearer token for protected or cloud-hosted Ollama instances",
                "placeholder": "Enter key if required"
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
    let payload: InputPayload = input.as_ref()
        .and_then(|v| serde_json::from_value(v.clone()).ok())
        .unwrap_or_default();

    let cfg = CONFIG.merge(payload.config);

    let endpoint = {
        let v = HotConfig::get_str(&cfg, "endpoint");
        if v.is_empty() { "http://localhost:11434".to_string() } else { v }
    };

    let model = {
        let m = payload.model.as_deref().unwrap_or("").trim().to_string();
        if m.is_empty() {
            let dm = HotConfig::get_str(&cfg, "default_model");
            if dm.is_empty() { "llama3.2".to_string() } else { dm }
        } else { m }
    };

    let parsed = match url::Url::parse(&endpoint) {
        Ok(u) => u,
        Err(e) => {
            return CallResponse {
                output: Some(serde_json::json!({"error": format!("invalid base_url: {}", e)})),
                error: Some(format!("invalid base_url: {}", e)),
            };
        }
    };
    let host = format!("{}://{}", parsed.scheme(), parsed.host_str().unwrap_or("localhost"));
    let port = parsed.port().unwrap_or_else(|| {
        if parsed.scheme() == "https" { 443 } else { 11434 }
    });

    let api_key = HotConfig::get_str(&cfg, "api_key");
    let client = if !api_key.is_empty() {
        use reqwest::header::{HeaderMap, HeaderValue, AUTHORIZATION};
        let mut headers = HeaderMap::new();
        if let Ok(mut auth_val) = HeaderValue::from_str(&format!("Bearer {}", api_key)) {
            auth_val.set_sensitive(true);
            headers.insert(AUTHORIZATION, auth_val);
        }
        let req_client = reqwest::Client::builder()
            .default_headers(headers)
            .build()
            .unwrap_or_default();
        Ollama::new_with_client(host, port, req_client)
    } else {
        Ollama::new(host, port)
    };

    let raw_messages = payload.messages.unwrap_or_default();
    let sanitized = sanitize_messages(&raw_messages);

    let mut ollama_messages: Vec<ChatMessage> = Vec::with_capacity(sanitized.len());
    let last_idx = if sanitized.is_empty() { 0 } else { sanitized.len() - 1 };

    for (idx, m) in sanitized.iter().enumerate() {
        let mut images: Vec<Image> = Vec::new();
        
        // ONLY send images for the LATEST message to avoid saturating cloud proxies
        if idx == last_idx {
            if let Some(blocks) = &m.blocks {
                for b in blocks {
                    if b.r#type == "image" {
                        if let Some(data) = &b.data {
                             let clean_data = data.chars().filter(|c| !c.is_whitespace()).collect::<String>();
                             images.push(Image::from_base64(clean_data));
                        }
                    }
                }
            }
        }

        let mut msg = match m.role.as_str() {
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

        if !images.is_empty() {
            msg.images = Some(images);
        }
        ollama_messages.push(msg);
    }

    // MANUAL REQWEST IMPLEMENTATION: Bypassing ollama-rs to ensure ultra-minimalist JSON.
    let endpoint = input.as_ref()
        .and_then(|v| v.get("config"))
        .and_then(|c| c.get("endpoint"))
        .and_then(|v| v.as_str())
        .unwrap_or("http://localhost:11434");
        
    let api_key = input.as_ref()
        .and_then(|v| v.get("config"))
        .and_then(|c| c.get("api_key"))
        .and_then(|v| v.as_str())
        .unwrap_or("");
    
    let client = reqwest::Client::new();
    let url = format!("{}/api/chat", endpoint.trim_end_matches('/'));
    
    let request_payload = serde_json::json!({
        "model": model,
        "messages": ollama_messages,
        "stream": false
    });

    openlobster_sdk_base::emit_log("debug", &format!("Ollama: Sending manual POST to {} (len={})", url, request_payload.to_string().len()));
    
    let mut req_builder = client.post(&url).json(&request_payload);
    if !api_key.is_empty() {
        req_builder = req_builder.header("Authorization", format!("Bearer {}", api_key));
    }

    match req_builder.send().await {
        Ok(resp) => {
            let status = resp.status();
            let body = resp.text().await.unwrap_or_default();
            
            if status.is_success() {
                let resp_val: Value = serde_json::from_str(&body).unwrap_or(Value::Null);
                let content = resp_val.get("message").and_then(|m| m.get("content")).and_then(|c| c.as_str()).unwrap_or("").to_string();
                
                let mut out_tool_calls: Vec<ToolCallMsg> = Vec::new();
                if let Some(tcs) = resp_val.get("message").and_then(|m| m.get("tool_calls")).and_then(|a| a.as_array()) {
                    for (i, tc) in tcs.iter().enumerate() {
                        if let (Some(name), Some(args)) = (tc.get("function").and_then(|f| f.get("name")).and_then(|n| n.as_str()),
                                                         tc.get("function").and_then(|f| f.get("arguments"))) {
                            out_tool_calls.push(ToolCallMsg {
                                id: format!("call_{}", i),
                                r#type: "function".to_string(),
                                function: ToolCallFunction {
                                    name: name.to_string(),
                                    arguments: args.to_string(),
                                },
                            });
                        }
                    }
                }

                let prompt_tokens = resp_val.get("prompt_eval_count").and_then(|v| v.as_u64()).unwrap_or(0);
                let completion_tokens = resp_val.get("eval_count").and_then(|v| v.as_u64()).unwrap_or(0);

                let stop_reason = if !out_tool_calls.is_empty() { "tool_use".to_string() } else { "stop".to_string() };

                openlobster_sdk_base::emit_log("debug", &format!("Ollama: Manual Success content_len={} tools={}", content.len(), out_tool_calls.len()));

                CallResponse::ok(OutputPayload {
                    content, tool_calls: out_tool_calls, stop_reason,
                    usage: UsagePayload { prompt_tokens, completion_tokens },
                    error: None,
                })
            } else {
                openlobster_sdk_base::emit_log("error", &format!("Ollama: Manual request failed status={} body={}", status, body));
                CallResponse::err(format!("Ollama status {}: {}", status, body))
            }
        },
        Err(e) => {
            openlobster_sdk_base::emit_log("error", &format!("Ollama connection error: {}", e));
            CallResponse::err(format!("Ollama connection error: {}", e))
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
            name: "Ollama",
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
