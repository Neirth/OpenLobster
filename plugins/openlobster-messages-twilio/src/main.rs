// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Twilio messaging plugin (Rust).
//!
//! Delegates messaging operations to the Twilio REST API via reqwest.

use std::collections::HashMap;

use async_trait::async_trait;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use openlobster_sdk_base::{run, emit_log, emit_message, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Plugin business types
// ---------------------------------------------------------------------------

#[derive(Serialize, Debug)]
struct CapabilitiesOutput {
    #[serde(rename = "HasVoiceMessage")] has_voice_message: bool,
    #[serde(rename = "HasCallStream")]   has_call_stream: bool,
    #[serde(rename = "HasTextStream")]   has_text_stream: bool,
    #[serde(rename = "HasMediaSupport")] has_media_support: bool,
}

#[derive(Deserialize, Debug, Default)]
struct SendInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
    message: SendMessage,
}

#[derive(Deserialize, Debug, Default)]
struct SendMessage {
    #[serde(default)] channel_id: Option<String>,
    #[serde(default)] recipient_id: Option<String>,
    #[serde(default)] sender_id: Option<String>,
    #[serde(default)] metadata: Option<HashMap<String, Value>>,
    #[serde(default)] content: String,
    #[serde(default)] media_url: Option<String>,
    #[serde(default)] audio: Option<AudioContent>,
}

#[derive(Deserialize, Debug, Default)]
struct AudioContent {
    data: String, // Base64
    format: Option<String>,
    duration: Option<u64>,
    platform_format: Option<String>,
    url: Option<String>,
}

#[derive(Deserialize, Debug, Default)]
struct ResolveChannelIdInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
    message: ResolveChannelIdMessage,
}

#[derive(Deserialize, Debug, Default)]
struct ResolveChannelIdMessage {
    #[serde(rename = "channel_id",   default)] channel_id: Option<String>,
    #[serde(rename = "recipient_id", default)] recipient_id: Option<String>,
    #[serde(rename = "sender_id",    default)] sender_id: Option<String>,
    #[serde(rename = "metadata",     default)] metadata: Option<HashMap<String, Value>>,
}

#[derive(Deserialize, Debug, Default)]
struct StartInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
}

#[derive(Deserialize, Debug, Default)]
struct TypingInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
    message: SendMessage,
    #[serde(default)] duration_ms: u64,
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "twilio";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Twilio SMS/MMS messaging plugin for OpenLobster";
const PLUGIN_TYPE: &str = "messaging";

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "account_sid": {
                "type": "string",
                "title": "Account SID",
                "description": "Your unique Twilio Account SID from the Console",
                "placeholder": "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
            },
            "auth_token": {
                "type": "string",
                "format": "password",
                "title": "Auth Token",
                "description": "The secret Auth Token associated with your Account SID",
                "placeholder": "Enter your Auth Token"
            },
            "from_number": {
                "type": "string",
                "title": "From Number",
                "description": "A purchased Twilio phone number (in E.164 format)",
                "placeholder": "+15550001234"
            },
            "messaging_service_sid": {
                "type": "string",
                "title": "Messaging Service SID (Optional)",
                "description": "Optional CID for Twilio Messaging Services features",
                "placeholder": "MGxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
            }
        },
        "required": ["account_sid", "auth_token", "from_number"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({
        "inbound_mode": "webhook",
        "HasVoiceMessage": true, "HasCallStream": true,
        "HasTextStream": true, "HasMediaSupport": true
    })
}

fn get_metadata() -> CallResponse {
    CallResponse::ok(serde_json::json!({
        "id": PLUGIN_ID, "name": "Twilio", "version": PLUGIN_VERSION,
        "description": PLUGIN_DESC, "type": PLUGIN_TYPE,
        "schema": metadata_schema(), "properties": metadata_properties()
    }))
}

// ---------------------------------------------------------------------------
// Phone normalization
// ---------------------------------------------------------------------------

fn normalize_phone(phone: &str) -> String {
    let digits: String = phone.chars().filter(|c| c.is_ascii_digit()).collect();
    if digits.len() == 10 {
        format!("+1{}", digits)
    } else if digits.len() == 11 && digits.starts_with('1') {
        format!("+{}", digits)
    } else if !digits.starts_with('+') && !phone.starts_with('+') {
        format!("+{}", digits)
    } else {
        digits
    }
}

// ---------------------------------------------------------------------------
// inbound_mode
// ---------------------------------------------------------------------------

fn inbound_mode() -> CallResponse { CallResponse::ok(serde_json::json!("webhook")) }

// ---------------------------------------------------------------------------
// capabilities
// ---------------------------------------------------------------------------

fn capabilities() -> CallResponse {
    let caps = CapabilitiesOutput {
        has_voice_message: true, has_call_stream: true,
        has_text_stream: true, has_media_support: true,
    };
    CallResponse::ok(serde_json::to_value(&caps).unwrap_or(Value::Null))
}

// ---------------------------------------------------------------------------
// handle_webhook
// ---------------------------------------------------------------------------

async fn handle_webhook(input: Option<Value>) -> CallResponse {
    let payload = match input {
        Some(v) => v,
        None    => return CallResponse::err("handle_webhook requires payload"),
    };

    // The core passes a 'request' object with a 'body' string (which is form-encoded for Twilio)
    let body_str = payload.get("request").and_then(|r| r.get("body")).and_then(|b| b.as_str()).unwrap_or_default();
    
    // Parse form data: From, Body, NumMedia, MediaUrl0, etc.
    let params: HashMap<String, String> = serde_urlencoded::from_str(body_str).unwrap_or_default();
    
    let from = params.get("From").cloned().unwrap_or_default();
    let text = params.get("Body").cloned().unwrap_or_default();
    let num_media: i32 = params.get("NumMedia").and_then(|n| n.parse().ok()).unwrap_or(0);

    let mut attachments = Vec::new();
    let mut audio_content = None;

    if num_media > 0 {
        let client = reqwest::Client::new();
        for i in 0..num_media {
            let url_key = format!("MediaUrl{}", i);
            let mime_key = format!("MediaContentType{}", i);
            
            if let Some(url) = params.get(&url_key) {
                emit_log("info", &format!("Downloading Twilio media {}: {}", i, url));
                match client.get(url).send().await {
                    Ok(resp) => {
                        if resp.status().is_success() {
                            let mime = params.get(&mime_key).cloned().unwrap_or_else(|| "application/octet-stream".to_string());
                            match resp.bytes().await {
                                Ok(bytes) => {
                                    let b64 = BASE64.encode(&bytes);
                                    
                                    // Identify if this is the primary audio content
                                    if audio_content.is_none() && mime.starts_with("audio/") {
                                        audio_content = Some(serde_json::json!({
                                            "data": b64.clone(),
                                            "format": mime.split('/').last().unwrap_or("ogg").to_string(),
                                            "platform_format": "mms",
                                            "url": url
                                        }));
                                    }

                                    attachments.push(serde_json::json!({
                                        "type": "binary",
                                        "filename": format!("media_{}", i),
                                        "size": bytes.len(),
                                        "mime_type": mime,
                                        "data": b64
                                    }));
                                }
                                Err(e) => emit_log("error", &format!("Failed to read Twilio media bytes: {}", e)),
                            }
                        }
                    }
                    Err(e) => emit_log("error", &format!("Failed to download Twilio media: {}", e)),
                }
            }
        }
    }

    if !from.is_empty() {
        return CallResponse::ok(serde_json::json!({
            "channel_id":  from,
            "sender_id":   from,
            "content":     text,
            "attachments": attachments,
            "audio":       audio_content,
            "metadata":    params
        }));
    }

    CallResponse::ok(serde_json::json!({"ok": true}))
}

// ---------------------------------------------------------------------------
// resolve_channel_id
// ---------------------------------------------------------------------------

fn resolve_channel_id(input: Option<Value>) -> CallResponse {
    let payload: ResolveChannelIdInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => ResolveChannelIdInput::default(),
    };

    if let Some(recipient_id) = &payload.message.recipient_id {
        let trimmed = recipient_id.trim();
        if !trimmed.is_empty() {
            return CallResponse::ok(serde_json::json!(normalize_phone(trimmed)));
        }
    }

    if let Some(channel_id) = &payload.message.channel_id {
        let trimmed = channel_id.trim();
        if !trimmed.is_empty() && !trimmed.eq_ignore_ascii_case("twilio") {
            return CallResponse::ok(serde_json::json!(normalize_phone(trimmed)));
        }
    }

    if let Some(sender_id) = &payload.message.sender_id {
        let trimmed = sender_id.trim();
        if !trimmed.is_empty() {
            return CallResponse::ok(serde_json::json!(normalize_phone(trimmed)));
        }
    }

    if let Some(metadata) = &payload.message.metadata {
        for key in &["platform_user_id", "phone_number", "from", "to"] {
            if let Some(v) = metadata.get(*key) {
                if let Some(s) = v.as_str() {
                    let trimmed = s.trim();
                    if !trimmed.is_empty() {
                        return CallResponse::ok(serde_json::json!(normalize_phone(trimmed)));
                    }
                }
            }
        }
    }

    CallResponse {
        output: Some(serde_json::json!("")),
        error: Some("twilio resolve_channel_id: missing destination".to_string()),
    }
}

// ---------------------------------------------------------------------------
// send
// ---------------------------------------------------------------------------

async fn send(input: Option<Value>) -> CallResponse {
    let payload: SendInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("send requires input"),
    };

    let cfg         = CONFIG.merge(payload.config);
    let account_sid = HotConfig::get_str(&cfg, "account_sid");
    let auth_token  = HotConfig::get_str(&cfg, "auth_token");
    let from_number = HotConfig::get_str(&cfg, "from_number");

    if account_sid.is_empty() { return CallResponse::err("twilio account_sid required"); }
    if auth_token.is_empty()  { return CallResponse::err("twilio auth_token required"); }
    if from_number.is_empty() { return CallResponse::err("twilio from_number required"); }

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata
        }
    })));

    let to = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve recipient".to_string())) },
    };

    if to.is_empty() { return CallResponse::err("twilio send: missing recipient"); }

    let client = reqwest::Client::new();
    let url    = format!("https://api.twilio.com/2010-04-01/Accounts/{}/Messages.json", account_sid);

    let mut form = vec![
        ("To".to_string(),   normalize_phone(&to)),
        ("From".to_string(), from_number.clone()),
        ("Body".to_string(), payload.message.content.clone()),
    ];

    if let Some(media_url) = &payload.message.media_url {
        form.push(("MediaUrl".to_string(), media_url.clone()));
    } else if let Some(audio) = &payload.message.audio {
        if let Some(url) = &audio.url {
            form.push(("MediaUrl".to_string(), url.clone()));
        } else {
            emit_log("warn", "Twilio: Audio note received but no URL provided (Twilio requires MediaUrl for MMS), falling back to text.");
        }
    }

    if let Some(sid_val) = cfg.get("messaging_service_sid") {
        if let Some(sid) = sid_val.as_str() {
            if !sid.is_empty() { form.push(("MessagingServiceSid".to_string(), sid.to_string())); }
        }
    }

    let encoded = serde_urlencoded::to_string(&form).unwrap_or_default();

    match client
        .post(&url)
        .header("Authorization", format!("Basic {}", BASE64.encode(format!("{}:{}", account_sid, auth_token))))
        .header("Content-Type", "application/x-www-form-urlencoded")
        .body(encoded)
        .send().await
    {
        Ok(resp) => {
            if resp.status().is_success() {
                match resp.json::<Value>().await {
                    Ok(json) => CallResponse::ok(json),
                    Err(_)   => CallResponse::ok(serde_json::json!({"ok": true})),
                }
            } else {
                let err_text = resp.text().await.unwrap_or_default();
                CallResponse::err(format!("twilio API error: {}", err_text))
            }
        }
        Err(e) => CallResponse::err(format!("twilio request failed: {}", e)),
    }
}
// ---------------------------------------------------------------------------
// typing
// ---------------------------------------------------------------------------

async fn typing(input: Option<Value>) -> CallResponse {
    let payload: TypingInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("typing requires input"),
    };

    let cfg = CONFIG.merge(payload.config);
    let duration = payload.duration_ms;

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata
        }
    })));

    let to = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve recipient".to_string())) },
    };

    if to.is_empty() { return CallResponse::err("twilio typing: missing destination"); }

    emit_log("debug", &format!("Twilio typing requested (no-op) for {} during {}ms", to, duration));
    CallResponse::ok(serde_json::json!({"ok": true}))
}

// ---------------------------------------------------------------------------
// start
// ---------------------------------------------------------------------------

async fn start(input: Option<Value>) -> CallResponse {
    let payload: StartInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("start requires input"),
    };

    let cfg         = CONFIG.merge(payload.config);
    let account_sid = HotConfig::get_str(&cfg, "account_sid");
    let auth_token  = HotConfig::get_str(&cfg, "auth_token");

    if account_sid.is_empty() { return CallResponse::err("twilio account_sid required"); }
    if auth_token.is_empty()  { return CallResponse::err("twilio auth_token required"); }

    let client = reqwest::Client::new();
    let url    = format!("https://api.twilio.com/2010-04-01/Accounts/{}.json", account_sid);

    match client
        .get(&url)
        .header("Authorization", format!("Basic {}", BASE64.encode(format!("{}:{}", account_sid, auth_token))))
        .send().await
    {
        Ok(resp) => {
            if resp.status().is_success() {
                CallResponse::ok(serde_json::json!({
                    "status": "webhook_mode",
                    "note": "Twilio uses webhook-based inbound - configure webhook URL in Twilio Console"
                }))
            } else {
                CallResponse::err("twilio credentials invalid")
            }
        }
        Err(e) => CallResponse::err(format!("twilio request failed: {}", e)),
    }
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct TwilioPlugin;

#[async_trait]
impl Plugin for TwilioPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "Twilio",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["inbound_mode", "capabilities", "resolve_channel_id",
                          "send", "start", "configure", "get_metadata", "handle_webhook", "typing"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure"          => CONFIG.configure(input),
            "get_metadata"       => get_metadata(),
            "inbound_mode"       => inbound_mode(),
            "capabilities"       => capabilities(),
            "resolve_channel_id" => resolve_channel_id(input),
            "send"               => send(input).await,
            "typing"             => typing(input).await,
            "start"              => start(input).await,
            "handle_webhook"     => handle_webhook(input).await,
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(TwilioPlugin).await;
}
