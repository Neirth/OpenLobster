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
    #[serde(default)] attachments: Option<Vec<Attachment>>,
}

#[derive(Deserialize, Debug, Default)]
struct Attachment {
    #[serde(rename = "type")] r#type: String,
    filename: String,
    mime_type: String,
    #[serde(default)] data: String, // Base64
}

#[derive(Deserialize, Debug, Default)]
struct AudioContent {
    data: String, // Base64
    format: Option<String>,
    duration: Option<u64>,
    sample_rate: Option<u32>,
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
// Config resolution
// ---------------------------------------------------------------------------

fn resolve(input: &Value, key: &str, fallback: &str) -> String {
    if let Some(v) = input.get("config").and_then(|c| c.get(key))
        .and_then(Value::as_str).filter(|s| !s.is_empty())
    {
        return v.to_string();
    }
    if let Some(v) = input.get(key).and_then(Value::as_str).filter(|s| !s.is_empty()) {
        return v.to_string();
    }
    let hot = CONFIG.merge(None);
    let v = HotConfig::get_str(&hot, key);
    if !v.is_empty() { return v; }
    fallback.to_string()
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
    let input = input.unwrap_or(Value::Null);
    let payload_res: Result<SendInput, _> = serde_json::from_value(input.clone());
    if let Err(e) = payload_res {
        return CallResponse::err(format!("invalid input: {}", e));
    }
    let payload = payload_res.unwrap();
    let cfg     = CONFIG.merge(payload.config);
    
    let bot_token = HotConfig::get_str(&cfg, "bot_token");
    if bot_token.is_empty() {
        return CallResponse::err("telegram bot_token required");
    }

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata
        }
    })));

    let chat_id = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve chat_id".to_string())) },
    };

    let chat_id = chat_id.trim();
    if chat_id.is_empty() {
        return CallResponse::err("telegram send: missing destination");
    }

    let bot = Bot::new(&bot_token);
    let chat_id_num: i64 = match chat_id.parse() {
        Ok(id) => id,
        Err(_) => return CallResponse::err("telegram: invalid chat_id - must be numeric"),
    };

    let user_id = teloxide::types::Recipient::Id(ChatId(chat_id_num));

    // Multimedia
    if let Some(atts) = &payload.message.attachments {
        if !atts.is_empty() {
             for att in atts {
                if let Ok(bytes) = BASE64.decode(&att.data) {
                    let input_file = teloxide::types::InputFile::memory(bytes).file_name(att.filename.clone());
                    let escaped_content = escape_markdown_v2(&payload.message.content);
                    let _ = match att.r#type.as_str() {
                        "image" => bot.send_photo(user_id.clone(), input_file)
                            .caption(escaped_content)
                            .parse_mode(teloxide::types::ParseMode::MarkdownV2)
                            .await.map(|_| ()),
                        _       => bot.send_document(user_id.clone(), input_file)
                            .caption(escaped_content)
                            .parse_mode(teloxide::types::ParseMode::MarkdownV2)
                            .await.map(|_| ()),
                    };
                }
             }
             return CallResponse::ok(serde_json::json!({"ok": true}));
        }
    }

    // Default text
    let escaped_content = escape_markdown_v2(&payload.message.content);
    emit_log("info", &format!("Telegram: sending message (escaped_len={}) preview: {}", escaped_content.len(), if escaped_content.len() > 100 { format!("{}...", &escaped_content[..100]) } else { escaped_content.clone() }));

    match bot.send_message(user_id, escaped_content)
        .parse_mode(teloxide::types::ParseMode::MarkdownV2)
        .await {
        Ok(_)  => CallResponse::ok(serde_json::json!({"ok": true})),
        Err(e) => CallResponse::err(format!("telegram send text failed: {}", e)),
    }
}

/// Escapes characters for Telegram MarkdownV2 while attempting to preserve common markdown entities.
fn escape_markdown_v2(text: &str) -> String {
    let mut escaped = String::with_capacity(text.len() * 2);
    let mut in_code = false;
    let mut in_pre = false;
    
    // Pre-process common MD to Telegram MD
    let mut processed = text.to_string();
    processed = processed.replace("**", "BOLD_TAG_MARKER");
    processed = processed.replace("~~", "STRIKE_TAG_MARKER");
    processed = processed.replace("<u>", "UNDER_TAG_MARKER").replace("</u>", "UNDER_TAG_MARKER");
    
    let marker_bold = "BOLD_TAG_MARKER".chars().collect::<Vec<char>>();
    let marker_strike = "STRIKE_TAG_MARKER".chars().collect::<Vec<char>>();
    let marker_under = "UNDER_TAG_MARKER".chars().collect::<Vec<char>>();

    let chars_vec: Vec<char> = processed.chars().collect();
    let mut i = 0;
    while i < chars_vec.len() {
        let c = chars_vec[i];
        
        if !in_code && !in_pre {
            if chars_vec[i..].starts_with(&marker_bold) {
                escaped.push('*');
                i += marker_bold.len();
                continue;
            }
            if chars_vec[i..].starts_with(&marker_strike) {
                escaped.push('~');
                i += marker_strike.len();
                continue;
            }
            if chars_vec[i..].starts_with(&marker_under) {
                escaped.push('_');
                escaped.push('_');
                i += marker_under.len();
                continue;
            }
        }

        match c {
            '`' => {
                if i + 2 < chars_vec.len() && chars_vec[i+1] == '`' && chars_vec[i+2] == '`' {
                    in_pre = !in_pre;
                    escaped.push_str("```");
                    i += 3;
                    continue;
                } else {
                    in_code = !in_code;
                    escaped.push('`');
                }
            }
            _ if in_code || in_pre => {
                // Inside code blocks, only ` and \ must be escaped
                if c == '`' || c == '\\' {
                    escaped.push('\\');
                }
                escaped.push(c);
            }
            // MarkdownV2 reserved characters
            '\\' | '_' | '*' | '[' | ']' | '(' | ')' | '~' | '`' | '>' | '#' | '+' | '-' | '=' | '|' | '{' | '}' | '.' | '!' => {
                // If it's a marker we want to keep (like * for bold converted from **), don't escape it
                // BUT wait, * IS bold in MarkdownV2. So if we pushed '*' at line 360, it's fine.
                // If we find a RAW '*' or '_' here, we MUST escape it because it's not our marker.
                escaped.push('\\');
                escaped.push(c);
            }
            _ => escaped.push(c),
        }
        i += 1;
    }
    escaped
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

    // Run polling loop
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
                        let mut audio_json = serde_json::Value::Null;
                        let mut file_to_download: Option<(String, String, String)> = None; // (file_id, filename, mime)

                        emit_log("debug", &format!("DEBUG: Inbound message kind/content: {:?}", msg.kind));

                        if let Some(photo) = msg.photo() {
                                let best_photo = photo.iter().max_by_key(|p| p.file.size).unwrap();
                                file_to_download = Some((best_photo.file.id.to_string(), "photo.jpg".to_string(), "image/jpeg".to_string()));
                            }
                            if let Some(doc) = msg.document() {
                                file_to_download = Some((doc.file.id.to_string(), doc.file_name.clone().unwrap_or_else(|| "file".to_string()), doc.mime_type.as_ref().map(|m| m.to_string()).unwrap_or_else(|| "application/octet-stream".to_string())));
                            }
                            if let Some(voice) = msg.voice() {
                                file_to_download = Some((voice.file.id.to_string(), "voice.ogg".to_string(), voice.mime_type.as_ref().map(|m| m.to_string()).unwrap_or_else(|| "audio/ogg".to_string())));
                            }
                            if let Some(audio) = msg.audio() {
                                file_to_download = Some((audio.file.id.to_string(), audio.file_name.clone().unwrap_or_else(|| "audio.mp3".to_string()), audio.mime_type.as_ref().map(|m| m.to_string()).unwrap_or_else(|| "audio/mpeg".to_string())));
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
                                        let att_type = if mime.starts_with("audio/") { "audio" } else if mime.starts_with("image/") { "image" } else { "binary" };
                                        
                                        if att_type == "audio" && audio_json.is_null() {
                                            audio_json = serde_json::json!({
                                                "data": b64.clone(),
                                                "format": mime.split('/').last().unwrap_or("ogg").to_string(),
                                            });
                                        }

                                        attachments_json.push(serde_json::json!({
                                            "type": att_type,
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
                            "audio": audio_json,
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
                          "send", "send_voice", "start", "configure", "typing", "speaking"],
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
            "send_voice"         => fn_send_voice(input).await,
            "typing"             => typing(input).await,
            "speaking"           => speaking(input).await,
            "start"              => start(input).await,
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

async fn speaking(input: Option<Value>) -> CallResponse {
    let payload: TypingInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("speaking requires input"),
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
        return CallResponse::err("telegram speaking: missing destination");
    }

    let chat_id_num: i64 = match chat_id.parse() {
        Ok(id) => id,
        Err(_) => return CallResponse::err("telegram: invalid chat_id - must be numeric"),
    };

    tokio::spawn(async move {
        let bot = Bot::new(&bot_token);
        let user_id = teloxide::types::Recipient::Id(ChatId(chat_id_num));
        let start_time = tokio::time::Instant::now();
        let duration_ms = if duration == 0 { 10000 } else { duration };

        while start_time.elapsed().as_millis() < duration_ms as u128 {
            let _ = bot.send_chat_action(user_id.clone(), teloxide::types::ChatAction::UploadVoice).await;
            sleep(Duration::from_secs(4)).await;
        }
    });

    CallResponse::ok(serde_json::json!({"ok": true}))
}

#[tokio::main]
async fn main() {
    run(TelegramPlugin).await;
}

async fn fn_send_voice(input: Option<Value>) -> CallResponse {
    let input = match input {
        Some(v) => v,
        None    => return CallResponse::err("input required"),
    };

    let cfg = input.get("config").cloned().unwrap_or(Value::Null);
    let bot_token = resolve(&input, "bot_token", "");
    if bot_token.is_empty() {
        return CallResponse::err("telegram bot_token required for send_voice");
    }
    let payload: SendInput = match serde_json::from_value(input) {
        Ok(p)  => p,
        Err(e) => return CallResponse::err(format!("invalid input: {}", e)),
    };

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   payload.message.channel_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata
        }
    })));

    let chat_id = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve chat_id".to_string())) },
    };

    let chat_id = chat_id.trim();
    if chat_id.is_empty() {
        return CallResponse::err("telegram send_voice: missing destination");
    }

    let bot = Bot::new(&bot_token);
    let chat_id_num: i64 = match chat_id.parse() {
        Ok(id) => id,
        Err(_) => return CallResponse::err("telegram send_voice: invalid chat_id - must be numeric"),
    };

    let user_id = teloxide::types::Recipient::Id(ChatId(chat_id_num));

    if let Some(audio) = &payload.message.audio {
        emit_log("info", &format!("Telegram send_voice: received audio format={:?} size={}", audio.format, audio.data.len()));
        if let Ok(mut bytes) = BASE64.decode(&audio.data) {
            emit_log("info", &format!("Telegram send_voice: decoded base64, size={} bytes", bytes.len()));
            // Core Unified Format: PCM. We wrap it in WAV for Telegram.
            if audio.format.as_deref() == Some("pcm") || audio.format.is_none() {
                let sample_rate = audio.sample_rate.unwrap_or(16000);
                emit_log("info", &format!("Telegram send_voice: wrapping PCM ({}Hz) as WAV...", sample_rate));
                bytes = wrap_pcm_as_wav(bytes, sample_rate);
                emit_log("info", &format!("Telegram send_voice: WAV wrapper complete, final size={} bytes", bytes.len()));
            }

            let input_file = teloxide::types::InputFile::memory(bytes.clone());
            
            emit_log("info", "Telegram send_voice: calling bot.send_voice API...");
            // Try send_voice for bubble. Fallback to audio if platform rejects it.
            let escaped_content = escape_markdown_v2(&payload.message.content);
            let action = bot.send_voice(user_id.clone(), input_file)
                .caption(escaped_content.clone())
                .parse_mode(teloxide::types::ParseMode::MarkdownV2)
                .await;

            match action {
                Ok(msg_sent)  => {
                    emit_log("info", &format!("Telegram: Sent voice bubble MsgID={}", msg_sent.id));
                    return CallResponse::ok(serde_json::json!({"ok": true}));
                },
                Err(e) => {
                    emit_log("warn", &format!("Telegram voice bubble failed (trying audio fallback): {}", e));
                    let input_file_retry = teloxide::types::InputFile::memory(bytes);
                    let _ = bot.send_audio(user_id.clone(), input_file_retry)
                        .caption(escaped_content)
                        .parse_mode(teloxide::types::ParseMode::MarkdownV2)
                        .await;
                    return CallResponse::ok(serde_json::json!({"ok": true, "fallback": "audio"}));
                }
            }
        }
    }
    
    CallResponse::err("no audio provided for send_voice")
}

/// Prepends a standard 44-byte WAV header to raw PCM (S16LE) data.
fn wrap_pcm_as_wav(pcm_data: Vec<u8>, sample_rate: u32) -> Vec<u8> {
    let data_len = pcm_data.len() as u32;
    let mut header = Vec::with_capacity(44 + pcm_data.len());
    
    // RIFF header
    header.extend_from_slice(b"RIFF");
    header.extend_from_slice(&(36 + data_len).to_le_bytes()); 
    header.extend_from_slice(b"WAVE");
    
    // Format chunk
    header.extend_from_slice(b"fmt ");
    header.extend_from_slice(&16u32.to_le_bytes()); 
    header.extend_from_slice(&1u16.to_le_bytes());  // PCM
    header.extend_from_slice(&1u16.to_le_bytes());  // Mono
    header.extend_from_slice(&sample_rate.to_le_bytes());
    header.extend_from_slice(&(sample_rate * 2).to_le_bytes()); 
    header.extend_from_slice(&2u16.to_le_bytes());  
    header.extend_from_slice(&16u16.to_le_bytes()); 
    
    // Data chunk
    header.extend_from_slice(b"data");
    header.extend_from_slice(&data_len.to_le_bytes());
    header.extend_from_slice(&pcm_data);
    
    header
}
