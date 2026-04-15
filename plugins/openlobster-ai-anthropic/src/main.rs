// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Anthropic AI provider plugin (Rust).
//!
//! Delegates chat completions to the anthropic-rs SDK.

use std::collections::{HashMap, HashSet};

use anthropic::{
    types::{
        ContentBlock, Message, MessagesRequestBuilder, Role, StopReason, SystemPrompt,
        Tool, ToolResultContent,
    },
    ClientBuilder,
};
use async_trait::async_trait;
use openlobster_sdk_base::{run, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;

// ---------------------------------------------------------------------------
// Plugin constants
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "openlobster-ai-anthropic";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Anthropic Claude AI provider plugin for OpenLobster";
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
    prompt_tokens: u32,
    completion_tokens: u32,
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "api_key": {
                "type": "string",
                "title": "API Key",
                "description": "Anthropic API key from console.anthropic.com"
            },
            "base_url": {
                "type": "string",
                "title": "Base URL",
                "default": "https://api.anthropic.com",
                "description": "Anthropic API base URL (override for proxies)"
            },
            "model": {
                "type": "string",
                "title": "Model",
                "default": "claude-sonnet-4-5",
                "description": "Default Claude model used when the request omits model"
            }
        },
        "required": ["api_key"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({"supports_audio_input": false, "supports_audio_output": false})
}

// ---------------------------------------------------------------------------
// Tool name encoding (Anthropic does not allow ':' in tool names)
// ---------------------------------------------------------------------------

fn encode_tool_name(name: &str) -> String { name.replace(':', "__") }
fn decode_tool_name(name: &str) -> String { name.replace("__", ":") }

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
    let api_key = HotConfig::get_str(&cfg, "api_key");
    if api_key.is_empty() {
        return CallResponse {
            output: Some(serde_json::to_value(OutputPayload {
                content: String::new(), tool_calls: Vec::new(), stop_reason: String::new(),
                usage: UsagePayload::default(), error: Some("api_key required".to_string()),
            }).unwrap_or_default()),
            error: Some("api_key required".to_string()),
        };
    }

    let base_url = {
        let v = HotConfig::get_str(&cfg, "base_url");
        if v.is_empty() { "https://api.anthropic.com".to_string() } else { v }
    };

    let model = {
        let m = payload.model.as_deref().unwrap_or("").trim().to_string();
        if m.is_empty() {
            let dm = HotConfig::get_str(&cfg, "model");
            if dm.is_empty() { "claude-sonnet-4-5".to_string() } else { dm }
        } else { m }
    };

    let max_tokens: u32 = payload.max_tokens.unwrap_or(500);

    let client = match ClientBuilder::new().api_key(api_key).api_base(base_url).build() {
        Ok(c) => c,
        Err(e) => {
            return CallResponse {
                output: Some(serde_json::to_value(OutputPayload {
                    content: String::new(), tool_calls: Vec::new(), stop_reason: String::new(),
                    usage: UsagePayload::default(),
                    error: Some(format!("failed to build client: {}", e)),
                }).unwrap_or_default()),
                error: Some(e.to_string()),
            };
        }
    };

    let raw_messages = payload.messages.unwrap_or_default();
    let sanitized = sanitize_messages(&raw_messages);

    let mut system_text = String::new();
    let mut messages: Vec<Message> = Vec::with_capacity(sanitized.len());

    for m in &sanitized {
        match m.role.as_str() {
            "system" => {
                if !system_text.is_empty() { system_text.push('\n'); }
                system_text.push_str(&m.content);
            }
            "assistant" => {
                let mut blocks: Vec<ContentBlock> = Vec::new();
                if !m.content.is_empty() {
                    blocks.push(ContentBlock::text(m.content.clone()));
                }
                if let Some(tcs) = &m.tool_calls {
                    for tc in tcs {
                        let input: Value = serde_json::from_str(&tc.function.arguments)
                            .unwrap_or(Value::Object(Default::default()));
                        blocks.push(ContentBlock::ToolUse {
                            id: tc.id.clone(),
                            name: encode_tool_name(&tc.function.name),
                            input,
                        });
                    }
                }
                if blocks.is_empty() { blocks.push(ContentBlock::text(String::new())); }
                messages.push(Message { role: Role::Assistant, content: blocks });
            }
            "tool" => {
                let tool_use_id = m.tool_call_id.clone().unwrap_or_default();
                messages.push(Message {
                    role: Role::User,
                    content: vec![ContentBlock::ToolResult {
                        tool_use_id, is_error: None,
                        content: ToolResultContent::Text(m.content.clone()),
                    }],
                });
            }
            _ => {
                messages.push(Message {
                    role: Role::User,
                    content: vec![ContentBlock::text(m.content.clone())],
                });
            }
        }
    }

    let tools: Vec<Tool> = match payload.tools {
        Some(arr) => arr.into_iter().filter_map(|v| {
            let obj = v.as_object()?;
            let func = obj.get("function")?.as_object()?;
            let name = func.get("name")?.as_str()?;
            let description = func.get("description").and_then(|d| d.as_str()).unwrap_or("").to_string();
            let input_schema = func.get("parameters").cloned()
                .unwrap_or_else(|| serde_json::json!({"type":"object","properties":{}}));
            Some(Tool { name: encode_tool_name(name), description, input_schema })
        }).collect(),
        None => Vec::new(),
    };

    let mut builder = MessagesRequestBuilder::new(model.clone(), messages, max_tokens);
    if !system_text.is_empty() { builder = builder.system(SystemPrompt::Text(system_text)); }
    if !tools.is_empty() { builder = builder.tools(tools); }

    let request = match builder.build() {
        Ok(r) => r,
        Err(e) => {
            return CallResponse {
                output: Some(serde_json::to_value(OutputPayload {
                    content: String::new(), tool_calls: Vec::new(), stop_reason: String::new(),
                    usage: UsagePayload::default(),
                    error: Some(format!("failed to build request: {}", e)),
                }).unwrap_or_default()),
                error: Some(e.to_string()),
            };
        }
    };

    match client.messages(request).await {
        Ok(resp) => {
            let stop_reason = match resp.stop_reason {
                Some(StopReason::EndTurn) | Some(StopReason::StopSequence) | None => "stop",
                Some(StopReason::MaxTokens) => "length",
                Some(StopReason::ToolUse) => "tool_use",
            }.to_string();

            let mut content = String::new();
            let mut tool_calls: Vec<ToolCallMsg> = Vec::new();

            for block in resp.content {
                match block {
                    ContentBlock::Text { text } => content.push_str(&text),
                    ContentBlock::ToolUse { id, name, input } => {
                        tool_calls.push(ToolCallMsg {
                            id,
                            r#type: "function".to_string(),
                            function: ToolCallFunction {
                                name: decode_tool_name(&name),
                                arguments: serde_json::to_string(&input).unwrap_or_default(),
                            },
                        });
                    }
                    _ => {}
                }
            }

            CallResponse::ok(OutputPayload {
                content, tool_calls, stop_reason,
                usage: UsagePayload {
                    prompt_tokens: resp.usage.input_tokens,
                    completion_tokens: resp.usage.output_tokens,
                },
                error: None,
            })
        }
        Err(e) => {
            let msg = format!("anthropic chat request failed: model={}: {}", model, e);
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

struct AnthropicPlugin;

#[async_trait]
impl Plugin for AnthropicPlugin {
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
            "configure"    => CONFIG.configure(input),
            "chat"         => chat(input).await,
            "chat_with_audio" => chat_with_audio(input).await,
            "chat_to_audio"   => chat_to_audio(input).await,
            other          => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(AnthropicPlugin).await;
}
