// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster WhatsApp Business messaging plugin (Rust).
//!
//! Delegates messaging operations to the WhatsApp Business Cloud API via whatsapp-business-rs.

use std::collections::HashMap;

use async_trait::async_trait;
use openlobster_sdk_base::{run, emit_log, emit_message, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use whatsapp_business_rs::{Client, Draft};

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

const PLUGIN_ID: &str = "openlobster-messages-whatsapp";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "WhatsApp Business messaging plugin for OpenLobster";
const PLUGIN_TYPE: &str = "messaging";

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "phone_number_id": {
                "type": "string",
                "title": "Phone Number ID",
                "description": "WhatsApp Business Phone Number ID from Meta Developer Console"
            },
            "access_token": {
                "type": "string",
                "title": "Access Token",
                "description": "WhatsApp Business API access token from Meta Developer Console"
            },
            "app_secret": {
                "type": "string",
                "title": "App Secret",
                "description": "Meta app secret for webhook verification"
            },
            "webhook_verify_token": {
                "type": "string",
                "title": "Webhook Verify Token",
                "description": "Token used to verify webhook endpoint"
            },
            "api_version": {
                "type": "string",
                "title": "API Version",
                "default": "v18.0",
                "description": "WhatsApp Business API version"
            }
        },
        "required": ["phone_number_id", "access_token"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({
        "HasVoiceMessage": true, "HasCallStream": false,
        "HasTextStream": true, "HasMediaSupport": true
    })
}

fn get_metadata() -> CallResponse {
    CallResponse::ok(serde_json::json!({
        "id": PLUGIN_ID, "name": PLUGIN_ID, "version": PLUGIN_VERSION,
        "description": PLUGIN_DESC, "type": PLUGIN_TYPE,
        "schema": metadata_schema(), "properties": metadata_properties()
    }))
}

// ---------------------------------------------------------------------------
// Phone normalization
// ---------------------------------------------------------------------------

fn normalize_whatsapp_phone(phone: &str) -> String {
    phone.chars().filter(|c| c.is_ascii_digit()).collect()
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
        has_voice_message: true, has_call_stream: false,
        has_text_stream: true, has_media_support: true,
    };
    CallResponse::ok(serde_json::to_value(&caps).unwrap_or(Value::Null))
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
            return CallResponse::ok(serde_json::json!(normalize_whatsapp_phone(trimmed)));
        }
    }

    if let Some(channel_id) = &payload.message.channel_id {
        let trimmed = channel_id.trim();
        if !trimmed.is_empty() && !trimmed.eq_ignore_ascii_case("whatsapp") {
            return CallResponse::ok(serde_json::json!(normalize_whatsapp_phone(trimmed)));
        }
    }

    if let Some(sender_id) = &payload.message.sender_id {
        let trimmed = sender_id.trim();
        if !trimmed.is_empty() {
            return CallResponse::ok(serde_json::json!(normalize_whatsapp_phone(trimmed)));
        }
    }

    if let Some(metadata) = &payload.message.metadata {
        for key in &["platform_user_id", "phone_number", "from"] {
            if let Some(v) = metadata.get(*key) {
                if let Some(s) = v.as_str() {
                    let trimmed = s.trim();
                    if !trimmed.is_empty() {
                        return CallResponse::ok(serde_json::json!(normalize_whatsapp_phone(trimmed)));
                    }
                }
            }
        }
    }

    CallResponse {
        output: Some(serde_json::json!("")),
        error: Some("whatsapp resolve_channel_id: missing destination".to_string()),
    }
}

// ---------------------------------------------------------------------------
// handle_webhook
// ---------------------------------------------------------------------------

async fn handle_webhook(input: Option<Value>) -> CallResponse {
    let payload = match input {
        Some(v) => v,
        None    => return CallResponse::err("handle_webhook requires payload"),
    };

    let cfg = CONFIG.merge(None);
    let access_token = HotConfig::get_str(&cfg, "access_token");
    let api_version = HotConfig::get_str(&cfg, "api_version");
    let api_version = if api_version.is_empty() { "v18.0" } else { &api_version };

    // WhatsApp Webhook structure check
    if let Some(entry) = payload.get("entry").and_then(|e| e.as_array()).and_then(|a| a.first()) {
        if let Some(change) = entry.get("changes").and_then(|c| c.as_array()).and_then(|a| a.first()) {
            if let Some(value) = change.get("value") {
                if let Some(messages) = value.get("messages").and_then(|m| m.as_array()) {
                    if let Some(msg) = messages.first() {
                        let from = msg.get("from").and_then(|f| f.as_str()).unwrap_or_default();
                        let msg_type = msg.get("type").and_then(|t| t.as_str()).unwrap_or("text");
                        
                        let mut text_content = String::new();
                        let mut attachments = Vec::new();
                        let mut media_id = None;
                        let mut media_filename = "file".to_string();
                        let mut media_mime = "application/octet-stream".to_string();

                        match msg_type {
                            "text" => {
                                text_content = msg.get("text").and_then(|t| t.get("body")).and_then(|b| b.as_str()).unwrap_or_default().to_string();
                            }
                            "image" => {
                                let image = msg.get("image");
                                text_content = image.and_then(|i| i.get("caption")).and_then(|c| c.as_str()).unwrap_or_default().to_string();
                                media_id = image.and_then(|i| i.get("id")).and_then(|i| i.as_str());
                                media_filename = "photo.jpg".to_string();
                                media_mime = image.and_then(|i| i.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("image/jpeg").to_string();
                            }
                            "document" => {
                                let doc = msg.get("document");
                                text_content = doc.and_then(|d| d.get("caption")).and_then(|c| c.as_str()).unwrap_or_default().to_string();
                                media_id = doc.and_then(|d| d.get("id")).and_then(|i| i.as_str());
                                media_filename = doc.and_then(|d| d.get("filename")).and_then(|f| f.as_str()).unwrap_or("document").to_string();
                                media_mime = doc.and_then(|d| d.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("application/octet-stream").to_string();
                            }
                            "video" => {
                                let video = msg.get("video");
                                text_content = video.and_then(|v| v.get("caption")).and_then(|c| c.as_str()).unwrap_or_default().to_string();
                                media_id = video.and_then(|v| v.get("id")).and_then(|i| i.as_str());
                                media_mime = video.and_then(|v| v.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("video/mp4").to_string();
                            }
                            "audio" | "voice" => {
                                let audio = msg.get("audio").or(msg.get("voice"));
                                media_id = audio.and_then(|a| a.get("id")).and_then(|i| i.as_str());
                                media_mime = audio.and_then(|a| a.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("audio/ogg").to_string();
                            }
                            _ => {}
                        }

                        let mut audio_content_standard = None;
                        if audio_content.is_none() && (msg_type == "audio" || msg_type == "voice") {
                            // In a real scenario, we'd wait for download to finish to get the b64
                            // but for now we'll populate it if the download succeeds below.
                        }

                        // Download media if detected
                        if let Some(id) = media_id {
                            if !access_token.is_empty() {
                                if let Ok(bytes) = download_whatsapp_media(id, api_version, &access_token).await {
                                    let b64 = BASE64.encode(&bytes);
                                    if msg_type == "audio" || msg_type == "voice" {
                                        audio_content_standard = Some(serde_json::json!({
                                            "data": b64.clone(),
                                            "format": media_mime.split('/').last().unwrap_or("ogg").to_string(),
                                            "platform_format": msg_type
                                        }));
                                    }

                                    attachments.push(serde_json::json!({
                                        "type": "binary",
                                        "filename": media_filename,
                                        "size": bytes.len(),
                                        "mime_type": media_mime,
                                        "data": b64
                                    }));
                                }
                            }
                        }

                        return CallResponse::ok(serde_json::json!({
                            "channel_id":  from,
                            "sender_id":   from,
                            "content":     text_content,
                            "attachments": attachments,
                            "audio":       audio_content_standard,
                            "metadata":    payload
                        }));
                    }
                }
            }
        }
    }

    CallResponse::ok(serde_json::json!({"ok": true}))
}

// ---------------------------------------------------------------------------
// Helper: download_whatsapp_media
// ---------------------------------------------------------------------------

async fn download_whatsapp_media(media_id: &str, api_version: &str, access_token: &str) -> Result<Vec<u8>, String> {
    let client = reqwest::Client::new();
    let metadata_url = format!("https://graph.facebook.com/{}/{}", api_version, media_id);
    
    let resp = client.get(&metadata_url)
        .header("Authorization", format!("Bearer {}", access_token))
        .send().await
        .map_err(|e| format!("WhatsApp media metadata fetch failed: {}", e))?;

    let meta_json: Value = resp.json().await.map_err(|e| format!("Failed to parse WhatsApp media metadata: {}", e))?;
    
    let download_url = meta_json.get("url").and_then(|u| u.as_str())
        .ok_or_else(|| "WhatsApp media metadata missing URL".to_string())?;

    let file_resp = client.get(download_url)
        .header("Authorization", format!("Bearer {}", access_token))
        .send().await
        .map_err(|e| format!("WhatsApp file download failed: {}", e))?;

    if !file_resp.status().is_success() {
        return Err(format!("WhatsApp file download HTTP error: {}", file_resp.status()));
    }

    let bytes = file_resp.bytes().await.map_err(|e| format!("WhatsApp file read error: {}", e))?;
    Ok(bytes.to_vec())
}

// ---------------------------------------------------------------------------
// send
// ---------------------------------------------------------------------------

async fn send(input: Option<Value>) -> CallResponse {
    let payload: SendInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("send requires input"),
    };

    let cfg             = CONFIG.merge(payload.config);
    let phone_number_id = HotConfig::get_str(&cfg, "phone_number_id");
    let access_token    = HotConfig::get_str(&cfg, "access_token");

    if phone_number_id.is_empty() { return CallResponse::err("whatsapp phone_number_id required"); }
    if access_token.is_empty()    { return CallResponse::err("whatsapp access_token required"); }

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata
        }
    })));

    let recipient = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve recipient".to_string())) },
    };

    if recipient.is_empty() { return CallResponse::err("whatsapp send: missing recipient"); }

    let wa_client = match Client::new(&access_token).await {
        Ok(c)  => c,
        Err(e) => return CallResponse::err(format!("whatsapp client init failed: {}", e)),
    };

    // Audio support
    if let Some(audio) = &payload.message.audio {
        // WhatsApp Business API usually requires a URL for media. 
        // If we only have bytes, we might need a temporary upload, but the current 
        // SDK 'Draft::audio' expects a URL. We'll check if the core provided one.
        if let Some(url) = payload.message.media_url.clone() {
            emit_log("info", &format!("WhatsApp: Sending audio note via URL: {}", url));
            match wa_client.message(&phone_number_id).send(&recipient, Draft::audio(url)).await {
                Ok(_)  => return CallResponse::ok(serde_json::json!({"ok": true})),
                Err(e) => return CallResponse::err(format!("whatsapp audio send failed: {}", e)),
            }
        } else {
            emit_log("warn", "WhatsApp: Audio received but no media_url provided (byte upload not yet implemented for WhatsApp), falling back to text.");
        }
    }

    let draft = Draft::text(&payload.message.content);

    match wa_client.message(&phone_number_id).send(&recipient, draft).await {
        Ok(_response) => CallResponse::ok(serde_json::json!({"ok": true, "recipient": recipient})),
        Err(e)        => CallResponse::err(format!("whatsapp send failed: {}", e)),
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
            "metadata":     payload.message.metadata,
        }
    })));

    let recipient = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve recipient".to_string())) },
    };

    emit_log("debug", &format!("WhatsApp typing requested for {} during {}ms (Cloud API semi-supported via read-receipts but skipping for now)", recipient, duration));
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

    let cfg             = CONFIG.merge(payload.config);
    let phone_number_id = HotConfig::get_str(&cfg, "phone_number_id");
    let access_token    = HotConfig::get_str(&cfg, "access_token");

    if phone_number_id.is_empty() { return CallResponse::err("whatsapp phone_number_id required"); }
    if access_token.is_empty()    { return CallResponse::err("whatsapp access_token required"); }

    match Client::new(&access_token).await {
        Ok(wa_client) => {
            let _ = wa_client;
            CallResponse::ok(serde_json::json!({
                "status": "webhook_mode",
                "note": "WhatsApp uses webhook-based inbound - configure webhook URL in Meta Developer Console"
            }))
        }
        Err(e) => CallResponse::err(format!("whatsapp credentials invalid: {}", e)),
    }
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct WhatsAppPlugin;

#[async_trait]
impl Plugin for WhatsAppPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID, name: PLUGIN_ID, version: PLUGIN_VERSION,
            description: PLUGIN_DESC, plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(), properties: metadata_properties(),
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
    run(WhatsAppPlugin).await;
}
