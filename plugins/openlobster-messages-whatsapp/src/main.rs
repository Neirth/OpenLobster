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
use whatsapp_business_rs::message::{Media, MediaType, MediaSource, AudioExtension};
use opus_rs::{OpusEncoder, Application};
use ogg::PacketWriter;

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

const PLUGIN_ID: &str = "whatsapp";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "WhatsApp Business Cloud API messaging plugin for OpenLobster";
const PLUGIN_TYPE: &str = "messaging";

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "phone_number_id": {
                "type": "string",
                "title": "Phone Number ID",
                "description": "WhatsApp Business Phone Number ID from Meta Developer Console",
                "placeholder": "e.g., 102938475657483"
            },
            "access_token": {
                "type": "string",
                "format": "password",
                "title": "Access Token",
                "description": "WhatsApp Business API access token from Meta Developer Console",
                "placeholder": "EAAG..."
            },
            "app_secret": {
                "type": "string",
                "format": "password",
                "title": "App Secret",
                "description": "Meta app secret for webhook signature verification",
                "placeholder": "Enter your app secret"
            },
            "webhook_verify_token": {
                "type": "string",
                "format": "password",
                "title": "Webhook Verify Token",
                "description": "Token to verify your webhook endpoint in the Meta dashboard",
                "placeholder": "e.g., my_secure_token_123"
            },
            "api_version": {
                "type": "string",
                "title": "API Version",
                "description": "WhatsApp Business API version (e.g., v20.0)",
                "default": "v18.0",
                "placeholder": "v18.0"
            }
        },
        "required": ["phone_number_id", "access_token"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({
        "inbound_mode": "webhook",
        "HasVoiceMessage": true, "HasCallStream": false,
        "HasTextStream": true, "HasMediaSupport": true
    })
}

fn get_metadata() -> CallResponse {
    CallResponse::ok(serde_json::json!({
        "id": PLUGIN_ID, "name": "WhatsApp", "version": PLUGIN_VERSION,
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

    // Note: Media downloading on WhatsApp still requires an HTTP client. 
    // Since we are removing reqwest, we'll log it as a limitation for now 
    // or use the SDK if it adds download support in future.
    // For now, we process the metadata and emit the message.

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
                        let mut media_mime = "application/octet-stream".to_string();

                        match msg_type {
                            "text" => {
                                text_content = msg.get("text").and_then(|t| t.get("body")).and_then(|b| b.as_str()).unwrap_or_default().to_string();
                            }
                            "image" => {
                                let image = msg.get("image");
                                text_content = image.and_then(|i| i.get("caption")).and_then(|c| c.as_str()).unwrap_or_default().to_string();
                                media_id = image.and_then(|i| i.get("id")).and_then(|i| i.as_str());
                                media_mime = image.and_then(|i| i.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("image/jpeg").to_string();
                            }
                            "document" => {
                                let doc = msg.get("document");
                                text_content = doc.and_then(|d| d.get("caption")).and_then(|c| c.as_str()).unwrap_or_default().to_string();
                                media_id = doc.and_then(|d| d.get("id")).and_then(|i| i.as_str());
                                media_mime = doc.and_then(|d| d.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("application/octet-stream").to_string();
                            }
                            "audio" | "voice" => {
                                let audio = msg.get("audio").or(msg.get("voice"));
                                media_id = audio.and_then(|a| a.get("id")).and_then(|i| i.as_str());
                                media_mime = audio.and_then(|a| a.get("mime_type")).and_then(|m| m.as_str()).unwrap_or("audio/ogg").to_string();
                            }
                            _ => {}
                        }

                        // Emit message to core (media bytes skipped to avoid reqwest)
                        if let Some(id) = media_id {
                            attachments.push(serde_json::json!({
                                "type": "id_reference",
                                "id": id,
                                "mime_type": media_mime
                            }));
                        }

                        return CallResponse::ok(serde_json::json!({
                            "channel_id":  from,
                            "sender_id":   from,
                            "content":     text_content,
                            "attachments": attachments,
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
    if let Some(_audio) = &payload.message.audio {
        if let Some(url) = payload.message.media_url.clone() {
            emit_log("info", &format!("WhatsApp: Sending audio note via URL: {}", url));
            let media = Media::new(MediaSource::Link(url.into()), MediaType::Audio(AudioExtension::Ogg));
            match wa_client.message(&phone_number_id).send(&recipient, Draft::media(media)).await {
                Ok(_)  => return CallResponse::ok(serde_json::json!({"ok": true})),
                Err(e) => return CallResponse::err(format!("whatsapp audio send failed: {}", e)),
            }
        }
    }

    let formatted_content = convert_markdown_to_whatsapp(&payload.message.content);
    let draft = Draft::text(&formatted_content);

    match wa_client.message(&phone_number_id).send(&recipient, draft).await {
        Ok(_response) => CallResponse::ok(serde_json::json!({ "ok": true, "recipient": recipient })),
        Err(e)        => CallResponse::err(format!("whatsapp send failed: {}", e)),
    }
}

fn convert_markdown_to_whatsapp(text: &str) -> String {
    use regex::Regex;
    // WhatsApp Dialect: *bold*, _italic_, ~strike~
    // Standard MD: **bold**, *italic*
    
    // 1. **bold** -> *bold*
    let re_bold = Regex::new(r"\*\*([^\*]+)\*\*").unwrap();
    let text = re_bold.replace_all(text, "*$1*");
    
    // 2. *italic* -> _italic_
    // Need to be careful with existing _ and headers.
    let re_italic = Regex::new(r"([^\*])\*([^\*]+)\*([^\*])").unwrap();
    let text = re_italic.replace_all(&text, "$1_$2_$3");
    
    // Handle start/end
    let re_italic_start = Regex::new(r"^\*([^\*]+)\*").unwrap();
    let text = re_italic_start.replace_all(&text, "_$1_");
    let re_italic_end = Regex::new(r"\*([^\*]+)\*$").unwrap();
    let text = re_italic_end.replace_all(&text, "_$1_");
    
    text.to_string()
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

    emit_log("debug", &format!("WhatsApp typing requested for {} during {}ms", recipient, duration));
    CallResponse::ok(serde_json::json!({"ok": true}))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct WhatsAppPlugin;

#[async_trait]
impl Plugin for WhatsAppPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "WhatsApp",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["inbound_mode", "capabilities", "resolve_channel_id",
                          "send", "send_voice", "configure", "get_metadata", "handle_webhook", "typing", "speaking"],
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
            "send_voice"         => fn_send_voice(input).await,
            "typing"             => typing(input).await,
            "speaking"           => speaking(input).await,
            "handle_webhook"     => handle_webhook(input).await,
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

async fn speaking(input: Option<Value>) -> CallResponse {
    let payload: TypingInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("speaking requires input"),
    };
    emit_log("debug", &format!("WhatsApp speaking indicator (recording_audio) requested but not supported via public API – skipping. Duration: {}ms", payload.duration_ms));
    CallResponse::ok(serde_json::json!({"ok": true}))
}

#[tokio::main]
async fn main() {
    run(WhatsAppPlugin).await;
}

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

async fn fn_send_voice(input: Option<Value>) -> CallResponse {
    let input = match input {
        Some(v) => v,
        None    => return CallResponse::err("input required"),
    };

    let phone_number_id = resolve(&input, "phone_number_id", "");
    let access_token    = resolve(&input, "access_token", "");

    if phone_number_id.is_empty() || access_token.is_empty() {
        return CallResponse::err("whatsapp credentials required for send_voice");
    }

    let payload: SendInput = match serde_json::from_value(input.clone()) {
        Ok(p)  => p,
        Err(e) => return CallResponse::err(format!("invalid input: {}", e)),
    };

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &payload.config,
        "message": {
            "channel_id":   payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":    payload.message.sender_id,
            "metadata":     payload.message.metadata
        }
    })));

    let recipient = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse::err("failed to resolve WhatsApp recipient"),
    };

    let wa_client = match Client::new(&access_token).await {
        Ok(c)  => c,
        Err(e) => return CallResponse::err(format!("whatsapp client init failed: {}", e)),
    };

    if let Some(audio) = &payload.message.audio {
        if let Ok(bytes) = BASE64.decode(&audio.data) {
            // PCM to Ogg/Opus
            let mut pcm_s16 = Vec::with_capacity(bytes.len() / 2);
            for chunk in bytes.chunks_exact(2) {
                pcm_s16.push(i16::from_le_bytes([chunk[0], chunk[1]]));
            }

            let pcm_resampled = if audio.sample_rate.unwrap_or(16000) == 16000 {
                resample_16k_to_48k(&pcm_s16)
            } else {
                pcm_s16
            };

            match encode_opus_ogg(&pcm_resampled, 48000) {
                Ok(ogg_data) => {
                    // Using SDK's upload_media instead of manual reqwest
                    let mime = "audio/ogg".parse().unwrap();
                    match wa_client.message(&phone_number_id).upload_media(ogg_data, mime, "voice.ogg").await {
                        Ok(upload_resp) => {
                            let media = Media::new(MediaSource::Id(upload_resp), MediaType::Audio(AudioExtension::Ogg));
                            match wa_client.message(&phone_number_id).send(&recipient, Draft::media(media)).await {
                                Ok(_) => return CallResponse::ok(serde_json::json!({"ok": true})),
                                Err(e) => return CallResponse::err(format!("whatsapp voice message failed: {}", e)),
                            }
                        },
                        Err(e) => return CallResponse::err(format!("whatsapp media upload failed: {}", e)),
                    }
                },
                Err(e) => return CallResponse::err(format!("opus encoding failed: {}", e)),
            }
        }
    }

    CallResponse::err("send_voice failed")
}

fn resample_16k_to_48k(input: &[i16]) -> Vec<i16> {
    if input.len() < 2 { return input.to_vec(); }
    let mut output = Vec::with_capacity(input.len() * 3);
    for i in 0..input.len() - 1 {
        let s0 = input[i] as i32;
        let s1 = input[i+1] as i32;
        output.push(s0 as i16);
        output.push(((s0 * 2 + s1) / 3) as i16);
        output.push(((s0 + s1 * 2) / 3) as i16);
    }
    let last = input[input.len()-1];
    output.push(last); output.push(last); output.push(last);
    output
}

fn encode_opus_ogg(pcm: &[i16], sample_rate: u32) -> Result<Vec<u8>, String> {
    let mut encoder = OpusEncoder::new(sample_rate as i32, 1, Application::Voip)
        .map_err(|e| format!("encoder init failed: {:?}", e))?;
    let mut output_ogg = Vec::new();
    {
        let mut writer = PacketWriter::new(&mut output_ogg);
        let mut id_header = Vec::new();
        id_header.extend_from_slice(b"OpusHead");
        id_header.push(1); id_header.push(1);
        id_header.extend_from_slice(&0u16.to_le_bytes());
        id_header.extend_from_slice(&(sample_rate).to_le_bytes());
        id_header.extend_from_slice(&0i16.to_le_bytes());
        id_header.push(0);
        writer.write_packet(id_header, 1, ogg::PacketWriteEndInfo::EndPage, 0).map_err(|e| e.to_string())?;
        
        let mut comment_header = Vec::new();
        comment_header.extend_from_slice(b"OpusTags");
        let vendor = b"openlobster-rs";
        comment_header.extend_from_slice(&(vendor.len() as u32).to_le_bytes());
        comment_header.extend_from_slice(vendor);
        comment_header.extend_from_slice(&0u32.to_le_bytes());
        writer.write_packet(comment_header, 1, ogg::PacketWriteEndInfo::EndPage, 0).map_err(|e| e.to_string())?;

        let frame_size = 960;
        let mut abs_granule = 0;
        for chunk in pcm.chunks_exact(frame_size) {
            let f32_chunk: Vec<f32> = chunk.iter().map(|&s| s as f32 / 32768.0).collect();
            let mut encoded = vec![0u8; 1024];
            let len = encoder.encode(&f32_chunk, frame_size, &mut encoded).map_err(|e| format!("{:?}", e))?;
            encoded.truncate(len);
            abs_granule += frame_size as u64;
            writer.write_packet(encoded, 1, ogg::PacketWriteEndInfo::EndPage, abs_granule).map_err(|e| e.to_string())?;
        }
    }
    Ok(output_ogg)
}
