// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Telegram messaging plugin (Rust).
//!
//! Delegates messaging operations to the teloxide SDK.

use std::collections::HashMap;

use async_trait::async_trait;
use openlobster_sdk_base::{run, emit_log, emit_message, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use teloxide::prelude::*;
use teloxide::net::Download;
use futures::StreamExt;
use teloxide::Bot;
use tokio::time::{sleep, Duration};

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

const PLUGIN_ID: &str = "telegram";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Telegram messaging plugin for OpenLobster via bot API";
const PLUGIN_TYPE: &str = "messaging";

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "bot_token": {
                "type": "string",
                "format": "password",
                "title": "Bot Token",
                "description": "Telegram bot token from @BotFather",
                "placeholder": "123456789:ABCdefG..."
            }
        },
        "required": ["bot_token"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({
        "inbound_mode": "polling",
        "HasVoiceMessage": true, "HasCallStream": false,
        "HasTextStream": true, "HasMediaSupport": true
    })
}

// ---------------------------------------------------------------------------
// Messaging discovery
// ---------------------------------------------------------------------------

fn inbound_mode() -> CallResponse { CallResponse::ok(serde_json::json!("polling")) }

// ---------------------------------------------------------------------------
// handle_webhook
// ---------------------------------------------------------------------------

fn handle_webhook(input: Option<Value>) -> CallResponse {
    let payload = match input {
        Some(v) => v,
        None    => return CallResponse::err("handle_webhook requires payload"),
    };

    if let Some(message) = payload.get("message") {
        let chat_id = message.get("chat").and_then(|c| c.get("id"));
        let text    = message.get("text");
        let from    = message.get("from").and_then(|f| f.get("first_name"));

        if chat_id.is_some() {
            return CallResponse::ok(serde_json::json!({
                "channel_id":  chat_id,
                "content":     text,
                "sender_name": from,
                "metadata":    payload
            }));
        }
    }

    CallResponse::ok(serde_json::json!({"ok": true}))
}

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

    let cfg = CONFIG.merge(payload.config);

    if let Some(channel_id) = &payload.message.channel_id {
        let trimmed = channel_id.trim();
        if !trimmed.is_empty() && !trimmed.eq_ignore_ascii_case("telegram") {
            return CallResponse::ok(serde_json::json!(trimmed.to_string()));
        }
    }

    if let Some(recipient_id) = &payload.message.recipient_id {
        let trimmed = recipient_id.trim();
        if !trimmed.is_empty() {
            return CallResponse::ok(serde_json::json!(trimmed.to_string()));
        }
    }

    if let Some(sender_id) = &payload.message.sender_id {
        let trimmed = sender_id.trim();
        if !trimmed.is_empty() {
            return CallResponse::ok(serde_json::json!(trimmed.to_string()));
        }
    }

    if let Some(metadata) = &payload.message.metadata {
        for key in &["platform_channel_id", "platform_user_id", "chat_id", "recipient_id"] {
            if let Some(v) = metadata.get(&key.to_string()) {
                if let serde_json::Value::String(s) = v {
                    let trimmed = s.trim();
                    if !trimmed.is_empty() {
                        return CallResponse::ok(serde_json::json!(trimmed.to_string()));
                    }
                }
            }
        }
    }

    let default_recipient = HotConfig::get_str(&cfg, "default_recipient_id");
    if !default_recipient.is_empty() {
        return CallResponse::ok(serde_json::json!(default_recipient));
    }

    CallResponse {
        output: Some(serde_json::json!("")),
        error: Some("telegram resolve_channel_id: missing destination".to_string()),
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

    let cfg       = CONFIG.merge(payload.config);
    let bot_token = HotConfig::get_str(&cfg, "bot_token");

    if bot_token.is_empty() {
        return CallResponse::err("telegram bot_token required");
    }

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":  payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":   payload.message.sender_id,
            "metadata":    payload.message.metadata
        }
    })));

    let chat_id = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve chat_id".to_string())) },
    };

    if chat_id.is_empty() {
        return CallResponse::err("telegram send: missing destination");
    }

    let bot = Bot::new(&bot_token);
    let chat_id_num: i64 = match chat_id.parse() {
        Ok(id) => id,
        Err(_) => return CallResponse::err("telegram: invalid chat_id - must be numeric"),
    };

    let user_id = teloxide::types::UserId(chat_id_num as u64);

    // Audio support
    if let Some(audio) = &payload.message.audio {
        if let Ok(bytes) = BASE64.decode(&audio.data) {
            let input_file = teloxide::types::InputFile::memory(bytes);
                    let action = if audio.format.as_deref() == Some("ogg") || audio.platform_format.as_deref() == Some("voice") {
                        bot.send_voice(user_id.clone(), input_file).caption(&payload.message.content).await
                    } else {
                        bot.send_audio(user_id.clone(), input_file).caption(&payload.message.content).await
                    };
                    
                    match action {
                        Ok(_)  => return CallResponse::ok(serde_json::json!({"ok": true})),
                        Err(e) => return CallResponse::err(format!("telegram audio send failed: {}", e)),
                    }
        }
    }

    match bot.send_message(user_id, &payload.message.content).await {
        Ok(_)  => CallResponse::ok(serde_json::json!({"ok": true})),
        Err(e) => CallResponse::err(format!("telegram send failed: {}", e)),
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

    let cfg       = CONFIG.merge(payload.config);
    let bot_token = HotConfig::get_str(&cfg, "bot_token");
    let duration  = payload.duration_ms;

    if bot_token.is_empty() {
        return CallResponse::err("telegram bot_token required");
    }

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata,
        }
    })));

    let chat_id = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve chat_id".to_string())) },
    };

    if chat_id.is_empty() {
        return CallResponse::err("telegram typing: missing destination");
    }

    let chat_id_num: i64 = match chat_id.parse() {
        Ok(id) => id,
        Err(_) => return CallResponse::err("telegram: invalid chat_id - must be numeric"),
    };

    // Autonomous background loop to keep the typing indicator alive
    tokio::spawn(async move {
        let bot = Bot::new(&bot_token);
        let user_id = teloxide::types::Recipient::Id(ChatId(chat_id_num));
        let start_time = tokio::time::Instant::now();
        let duration_ms = if duration == 0 { 10000 } else { duration };

        while start_time.elapsed().as_millis() < duration_ms as u128 {
            let _ = bot.send_chat_action(user_id.clone(), teloxide::types::ChatAction::Typing).await;
            // Telegram typing usually lasts ~5s, so refresh every 4s
            sleep(Duration::from_secs(4)).await;
        }
    });

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

    let cfg       = CONFIG.merge(payload.config);
    let bot_token = HotConfig::get_str(&cfg, "bot_token");

    if bot_token.is_empty() {
        return CallResponse::err("telegram bot_token required");
    }

    let bot = Bot::new(&bot_token);

    match bot.get_me().await {
        Ok(info)  => emit_log("info", &format!("Telegram bot {} (@{}) started in polling mode", 
            info.user.first_name, info.user.username.unwrap_or_default())),
        Err(e) => return CallResponse::err(format!("telegram bot_token invalid: {}", e)),
    }

    // Spawn polling loop in background
    tokio::spawn(async move {
        let mut offset: Option<i32> = None;
        emit_log("info", "Starting Telegram polling loop...");

        loop {
            let mut request = bot.get_updates();
            if let Some(o) = offset {
                request = request.offset(o);
            }

            match request.timeout(10).send().await {
                Ok(updates) => {
                    for update in updates {
                        // Diagnostic RAW logging
                        emit_log("debug", &format!("[RAW_UPDATE] {:?}", update));

                        // Update offset
                        offset = Some(update.id.0 as i32 + 1);

                        use teloxide::types::UpdateKind;
                        if let UpdateKind::Message(msg) = update.kind {
                            let chat_id = msg.chat.id.0;
                            let text = msg.text().or(msg.caption()).map(|s| s.to_string()).unwrap_or_default();
                            let from = msg.from.as_ref();
                            let sender_name = from.map(|u| u.first_name.clone()).unwrap_or_else(|| "Unknown".to_string());
                            let sender_id = from.map(|u| u.id.0.to_string()).unwrap_or_default();

                            // Media extraction
                            let mut attachments_json = Vec::new();
                            let mut file_to_download: Option<(String, String, String)> = None; // (file_id, filename, mime)

                            if let Some(photo) = msg.photo() {
                                    let best_photo = photo.iter().max_by_key(|p| p.file.size).unwrap();
                                    file_to_download = Some((best_photo.file.id.to_string(), "photo.jpg".to_string(), "image/jpeg".to_string()));
                                }
                                if let Some(doc) = msg.document() {
                                    file_to_download = Some((doc.file.id.to_string(), doc.file_name.clone().unwrap_or_else(|| "file".to_string()), doc.mime_type.as_ref().map(|m| m.to_string()).unwrap_or_else(|| "application/octet-stream".to_string())));
                                }

                                if let Some((file_id_str, filename, mime)) = file_to_download {
                                    match bot.get_file(teloxide::types::FileId(file_id_str)).await {
                                    Ok(file) => {
                                        emit_log("info", &format!("Downloading Telegram media ({} via SDK)...", filename));
                                        
                                        let mut stream = bot.download_file_stream(&file.path);
                                        let mut buffer = Vec::new();
                                        while let Some(chunk_res) = stream.next().await {
                                            match chunk_res {
                                                Ok(chunk) => buffer.extend_from_slice(&chunk),
                                                Err(e) => {
                                                    emit_log("error", &format!("Failed to stream Telegram file: {}", e));
                                                    break;
                                                }
                                            }
                                        }

                                        if !buffer.is_empty() {
                                            let b64 = BASE64.encode(&buffer);
                                            attachments_json.push(serde_json::json!({
                                                "type": "binary",
                                                "filename": filename,
                                                "size": buffer.len(),
                                                "mime_type": mime,
                                                "data": b64
                                            }));
                                        }
                                    }
                                    Err(e) => emit_log("error", &format!("Failed to get Telegram file path: {}", e)),
                                }
                            }

                            // Emit message to core
                            emit_message(&serde_json::json!({
                                "channel_id": chat_id.to_string(),
                                "sender_id":  sender_id,
                                "sender_name": sender_name,
                                "content": text,
                                "attachments": attachments_json,
                                "metadata": {
                                    "platform": "telegram",
                                    "message_id": msg.id.0,
                                    "chat_id": chat_id
                                }
                            }));

                            emit_log("info", &format!("Message Inbound: from={} content='{}' (Media: {})", sender_id, text, attachments_json.len()));
                        }
                    }
                }
                Err(e) => {
                    emit_log("error", &format!("Telegram polling error (retrying in 2s): {}", e));
                    sleep(Duration::from_secs(2)).await;
                }
            }
        }
    });

    // Give the gateway time to stabilize
    sleep(Duration::from_secs(3)).await;

    CallResponse::ok(serde_json::json!({
        "status": "gateway_mode",
        "details": "Polling loop started in background"
    }))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct TelegramPlugin;

#[async_trait]
impl Plugin for TelegramPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "Telegram",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["inbound_mode", "capabilities", "resolve_channel_id",
                          "send", "start", "configure", "typing"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure"          => CONFIG.configure(input),
            "inbound_mode"       => inbound_mode(),
            "capabilities"       => capabilities(),
            "handle_webhook"     => handle_webhook(input),
            "resolve_channel_id" => resolve_channel_id(input),
            "send"               => send(input).await,
            "typing"             => typing(input).await,
            "start"              => start(input).await,
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(TelegramPlugin).await;
}
