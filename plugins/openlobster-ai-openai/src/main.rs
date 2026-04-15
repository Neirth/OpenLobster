// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster OpenAI AI provider plugin (Rust).
//!
//! Delegates chat completions to the async-openai SDK.

use std::collections::{HashMap, HashSet};

use async_openai::{
    config::OpenAIConfig,
    types::{
        ChatCompletionMessageToolCall, ChatCompletionRequestAssistantMessageArgs,
        ChatCompletionRequestMessage, ChatCompletionRequestSystemMessageArgs,
        ChatCompletionRequestToolMessageArgs, ChatCompletionRequestUserMessageArgs,
        ChatCompletionTool, ChatCompletionToolArgs, ChatCompletionToolType,
        CreateChatCompletionRequestArgs, FunctionCall, FunctionObjectArgs,
    },
    Client,
};
use async_trait::async_trait;
use openlobster_sdk_base::{run, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;

// ---------------------------------------------------------------------------
// Plugin constants
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "openlobster-ai-openai";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "OpenAI AI provider plugin for OpenLobster";
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
    #[serde(default)] max_tokens: Option<u16>,
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
            "api_key": {
                "type": "string",
                "title": "API Key",
                "description": "Provider API key used for authentication"
            },
            "model": {
                "type": "string",
                "title": "Model",
                "default": "gpt-4o",
                "description": "Default model when a request does not specify one"
            },
            "endpoint": {
                "type": "string",
                "title": "Endpoint",
                "description": "Select the provider endpoint by name",
                "default": "OpenAI",
                "enum": ["OpenAI", "OpenRouter", "Docker Model Runner", "OpenCode Zen",
                         "Groq", "Perplexity", "Mistral", "xAI", "Custom"]
            },
            "base_url": {
                "type": "string",
                "title": "Base URL (Custom)",
                "description": "Required when endpoint is Custom"
            }
        },
        "required": ["api_key"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({"supports_audio_input": true, "supports_audio_output": false})
}

// ---------------------------------------------------------------------------
// Message sanitization
// ---------------------------------------------------------------------------

fn sanitize_messages(messages: &[ChatMsg]) -> Vec<ChatMsg> {
    let mut valid_ids: HashSet<String> = HashSet::new();
    let mut out = Vec::with_capacity(messages.len());
    for m in messages {
        if m.role == "tool" {
            let tcid = m.tool_call_id.as_deref().unwrap_or("").trim();
            if tcid.is_empty() || !valid_ids.contains(tcid) { continue; }
        }
        if let Some(tcs) = &m.tool_calls {
            for tc in tcs {
                let id = tc.id.trim();
                if !id.is_empty() { valid_ids.insert(id.to_string()); }
            }
        }
        out.push(m.clone());
    }
    out
}

// ---------------------------------------------------------------------------
// Tool name encoding (mirrors Go plugin: ':' <-> '__')
// ---------------------------------------------------------------------------

fn encode_tool_name(name: &str) -> String { name.replace(':', "__") }
fn decode_tool_name(name: &str) -> String { name.replace("__", ":") }

// ---------------------------------------------------------------------------
// Endpoint resolution
// ---------------------------------------------------------------------------

fn resolve_endpoint_base_url(endpoint: &str) -> Result<String, String> {
    let name = endpoint.trim().to_lowercase();
    if name.is_empty() || name == "openai" { return Ok(String::new()); }
    match name.as_str() {
        "openrouter"                          => Ok("https://openrouter.ai/api/v1".into()),
        "docker model runner" | "docker-model-runner" => Ok("http://localhost:12434/engines/v1".into()),
        "opencode zen" | "opencode-zen" | "opencode" => Ok("https://opencode.ai/zen/v1".into()),
        "groq"                                => Ok("https://api.groq.com/openai/v1".into()),
        "deepseek"                            => Ok("https://api.deepseek.com/v1".into()),
        "perplexity"                          => Ok("https://api.perplexity.ai".into()),
        "mistral"                             => Ok("https://api.mistral.ai/v1".into()),
        "xai" | "x.ai"                        => Ok("https://api.x.ai/v1".into()),
        "custom"                              => Err("base_url required when endpoint is Custom".into()),
        _                                     => Err(format!("unsupported endpoint {:?}", endpoint)),
    }
}

// ---------------------------------------------------------------------------
// Chat export
// ---------------------------------------------------------------------------

async fn chat_inner(input: Option<Value>) -> anyhow::Result<CallResponse> {
    let payload: InputPayload = input
        .and_then(|v| serde_json::from_value(v).ok())
        .unwrap_or_default();

    let cfg = CONFIG.merge(payload.config);

    let api_key = HotConfig::get_str(&cfg, "api_key");
    if api_key.is_empty() { anyhow::bail!("api_key required"); }

    let base_url = {
        let direct = HotConfig::get_str(&cfg, "base_url");
        if !direct.is_empty() {
            direct
        } else {
            let endpoint = HotConfig::get_str(&cfg, "endpoint");
            resolve_endpoint_base_url(&endpoint).map_err(|e| anyhow::anyhow!(e))?
        }
    };

    let mut oai_config = OpenAIConfig::new().with_api_key(api_key);
    if !base_url.is_empty() { oai_config = oai_config.with_api_base(base_url); }
    let client = Client::with_config(oai_config);

    let model = {
        let m = payload.model.as_deref().unwrap_or("").trim().to_string();
        if m.is_empty() {
            let dm = HotConfig::get_str(&cfg, "model");
            if dm.is_empty() { "gpt-4o".to_string() } else { dm }
        } else { m }
    };

    let raw_messages = payload.messages.unwrap_or_default();
    let sanitized = sanitize_messages(&raw_messages);

    let mut messages: Vec<ChatCompletionRequestMessage> = Vec::with_capacity(sanitized.len());
    for m in &sanitized {
        let msg: ChatCompletionRequestMessage = match m.role.as_str() {
            "system" => ChatCompletionRequestSystemMessageArgs::default()
                .content(m.content.clone()).build()?.into(),
            "assistant" => {
                let mut builder = ChatCompletionRequestAssistantMessageArgs::default();
                if !m.content.is_empty() { builder.content(m.content.clone()); }
                if let Some(tcs) = &m.tool_calls {
                    let calls: Vec<_> = tcs.iter().map(|tc| ChatCompletionMessageToolCall {
                        id: tc.id.clone(),
                        r#type: ChatCompletionToolType::Function,
                        function: FunctionCall {
                            name: encode_tool_name(&tc.function.name),
                            arguments: tc.function.arguments.clone(),
                        },
                    }).collect();
                    builder.tool_calls(calls);
                }
                builder.build()?.into()
            }
            "tool" => {
                let tc_id = m.tool_call_id.as_deref().unwrap_or("").to_string();
                ChatCompletionRequestToolMessageArgs::default()
                    .tool_call_id(tc_id).content(m.content.clone()).build()?.into()
            }
            _ => ChatCompletionRequestUserMessageArgs::default()
                .content(m.content.clone()).build()?.into(),
        };
        messages.push(msg);
    }

    let mut tools: Vec<ChatCompletionTool> = Vec::new();
    if let Some(input_tools) = payload.tools {
        for t in input_tools {
            if t.get("type").and_then(|v| v.as_str()) != Some("function") { continue; }
            if let Some(func) = t.get("function") {
                let name = func.get("name").and_then(|v| v.as_str()).unwrap_or("").to_string();
                if name.is_empty() { continue; }
                let mut func_builder = FunctionObjectArgs::default();
                func_builder.name(encode_tool_name(&name));
                if let Some(desc) = func.get("description").and_then(|v| v.as_str()) {
                    func_builder.description(desc.to_string());
                }
                if let Some(params) = func.get("parameters").cloned() {
                    func_builder.parameters(params);
                }
                let tool = ChatCompletionToolArgs::default()
                    .r#type(ChatCompletionToolType::Function)
                    .function(func_builder.build()?)
                    .build()?;
                tools.push(tool);
            }
        }
    }

    let mut req_builder = CreateChatCompletionRequestArgs::default();
    req_builder.model(model);
    req_builder.messages(messages);
    if !tools.is_empty() { req_builder.tools(tools); }
    if let Some(max_tokens) = payload.max_tokens { req_builder.max_tokens(max_tokens); }

    let response = client.chat().create(req_builder.build()?).await?;

    let mut out = OutputPayload {
        content: String::new(), tool_calls: Vec::new(),
        stop_reason: "stop".to_string(), usage: UsagePayload::default(), error: None,
    };

    if let Some(choice) = response.choices.first() {
        out.content = choice.message.content.clone().unwrap_or_default();
        out.stop_reason = choice.finish_reason.as_ref()
            .and_then(|r| serde_json::to_value(r).ok())
            .and_then(|v| v.as_str().map(|s| s.to_string()))
            .map(|s| if s == "tool_calls" { "tool_use".to_string() } else { s })
            .unwrap_or_else(|| "stop".to_string());
        if let Some(tcs) = &choice.message.tool_calls {
            for tc in tcs {
                out.tool_calls.push(ToolCallMsg {
                    id: tc.id.clone(),
                    r#type: "function".to_string(),
                    function: ToolCallFunction {
                        name: decode_tool_name(&tc.function.name),
                        arguments: tc.function.arguments.clone(),
                    },
                });
            }
        }
    }

    if let Some(usage) = &response.usage {
        out.usage.prompt_tokens = usage.prompt_tokens as u64;
        out.usage.completion_tokens = usage.completion_tokens as u64;
    }

    Ok(CallResponse::ok(out))
}

async fn chat(input: Option<Value>) -> CallResponse {
    match chat_inner(input).await {
        Ok(resp) => resp,
        Err(e) => {
            let msg = format!("openai chat request failed: {}", e);
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

struct OpenAiPlugin;

#[async_trait]
impl Plugin for OpenAiPlugin {
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
            other          => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(OpenAiPlugin).await;
}
