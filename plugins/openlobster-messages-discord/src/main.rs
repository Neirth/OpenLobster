// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Discord messaging plugin (Rust).
//!
//! Delegates messaging operations to the twilight SDK.

use std::collections::HashMap;

use async_trait::async_trait;
use openlobster_sdk_base::{run, emit_log, emit_message, Plugin, CallResponse, HotConfig, PluginInfo};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use twilight_http::Client as HttpClient;
use twilight_model::id::marker::ChannelMarker;
use twilight_model::id::Id;
use twilight_gateway::{Cluster, Intents, Event};
use futures::stream::StreamExt;
use opus_rs::{OpusEncoder, Application};
use twilight_model::channel::message::{Message, MessageFlags};
use hyper::Client;
use hyper_rustls::HttpsConnectorBuilder;
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
    #[serde(default)] audio: Option<AudioContent>,
    #[serde(default)] attachments: Vec<Attachment>,
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
struct StartInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
}

#[derive(Deserialize, Debug, Default)]
struct TypingInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
    message: SendMessage,
    #[serde(default)] duration_ms: u64,
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

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "discord";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Discord messaging plugin for OpenLobster via bot gateway";
const PLUGIN_TYPE: &str = "messaging";

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "token": {
                "type": "string",
                "format": "password",
                "title": "Bot Token",
                "description": "Discord bot token from the Discord Developer Portal",
                "placeholder": "Enter your bot token"
            }
        },
        "required": ["token"]
    })
}

fn metadata_properties() -> Value { serde_json::json!({"inbound_mode": "gateway"}) }

// ---------------------------------------------------------------------------
// Messaging discovery
// ---------------------------------------------------------------------------

fn inbound_mode() -> CallResponse {
    CallResponse::ok(serde_json::json!("gateway"))
}

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
// resolve_channel_id
// ---------------------------------------------------------------------------

fn resolve_channel_id(input: Option<Value>) -> CallResponse {
    let payload: ResolveChannelIdInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => ResolveChannelIdInput::default(),
    };

    let cfg = CONFIG.merge(payload.config);

    if let Some(recipient_id) = &payload.message.recipient_id {
        let trimmed = recipient_id.trim();
        if !trimmed.is_empty() {
            return CallResponse::ok(serde_json::json!(trimmed.to_string()));
        }
    }

    if let Some(channel_id) = &payload.message.channel_id {
        let trimmed = channel_id.trim();
        if !trimmed.is_empty() && !trimmed.eq_ignore_ascii_case("discord") {
            return CallResponse::ok(serde_json::json!(trimmed.to_string()));
        }
    }

    if let Some(metadata) = &payload.message.metadata {
        if let Some(v) = metadata.get("platform_channel_id") {
            if let Some(s) = v.as_str() {
                return CallResponse::ok(serde_json::json!(s.trim().to_string()));
            }
        }
    }

    let default_channel = HotConfig::get_str(&cfg, "default_channel_id");
    if !default_channel.is_empty() {
        return CallResponse::ok(serde_json::json!(default_channel));
    }

    let sender_id = payload.message.sender_id.as_deref().unwrap_or("").trim();
    if !sender_id.is_empty() {
        return CallResponse::ok(serde_json::json!(sender_id.to_string()));
    }

    if let Some(metadata) = &payload.message.metadata {
        for key in &["platform_user_id", "recipient_id", "sender_id"] {
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

    CallResponse {
        output: Some(serde_json::json!("")),
        error: Some("discord resolve_channel_id: missing destination".to_string()),
    }
}

// ---------------------------------------------------------------------------
// send
// ---------------------------------------------------------------------------

async fn send(input: Option<Value>) -> CallResponse {
    let payload: SendInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None => return CallResponse::err("send requires input"),
    };

    let cfg   = CONFIG.merge(payload.config);
    let token = HotConfig::get_str(&cfg, "token");

    if token.is_empty() {
        return CallResponse::err("discord token required");
    }

    let http = HttpClient::new(token.clone());

    // If recipient_id is provided, we must create a private channel first.
    if let Some(recipient_id_str) = payload.message.recipient_id.as_deref().filter(|s| !s.trim().is_empty()) {
        if let Ok(user_id_u64) = recipient_id_str.trim().parse::<u64>() {
            let user_id = Id::new(user_id_u64);
            match http.create_private_channel(user_id).await {
                Ok(resp) => {
                    match resp.model().await {
                        Ok(channel) => {
                            match http.create_message(channel.id).content(&payload.message.content) {
                                Ok(req) => match req.await {
                                    Ok(_) => return CallResponse::ok(serde_json::json!({"ok": true})),
                                    Err(e) => return CallResponse::err(format!("discord DM send failed: {}", e)),
                                },
                                Err(e) => return CallResponse::err(format!("discord DM builder failed: {}", e)),
                            }
                        }
                        Err(e) => return CallResponse::err(format!("failed to parse DM channel model: {}", e)),
                    }
                }
                Err(e) => return CallResponse::err(format!("failed to create DM channel: {}", e)),
            }
        }
    }

    // Default: use resolved channel_id
    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":  payload.message.channel_id,
            "recipient_id": payload.message.recipient_id,
            "sender_id":   payload.message.sender_id,
            "metadata":    payload.message.metadata
        }
    })));

    let channel_id_str = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve channel id".to_string())) },
    };

    if channel_id_str.is_empty() {
        return CallResponse::err("discord send: missing destination");
    }

    if let Ok(channel_id_u64) = channel_id_str.parse::<u64>() {
        let cid = Id::<ChannelMarker>::new(channel_id_u64);
        let mut builder = http.create_message(cid);
        
        let mut twilight_attachments = Vec::new();
        let mut attachment_id = 0;

        // Audio support
        if let Some(audio) = &payload.message.audio {
            if let Ok(bytes) = BASE64.decode(&audio.data) {
                let filename = format!("voice.{}", audio.format.as_deref().unwrap_or("ogg"));
                twilight_attachments.push(twilight_model::http::attachment::Attachment {
                    description: None,
                    file: bytes,
                    filename,
                    id: attachment_id,
                });
                attachment_id += 1;
            }
        }

        // Multimedia / Attachments support
        for att in &payload.message.attachments {
            if let Ok(bytes) = BASE64.decode(&att.data) {
                twilight_attachments.push(twilight_model::http::attachment::Attachment {
                    description: None,
                    file: bytes,
                    filename: att.filename.clone(),
                    id: attachment_id,
                });
                attachment_id += 1;
            }
        }

        if !twilight_attachments.is_empty() {
            builder = match builder.attachments(&twilight_attachments) {
                Ok(b) => b,
                Err(e) => return CallResponse::err(format!("discord attachments failed: {}", e)),
            };
        }

        match builder.content(&payload.message.content) {
            Ok(req) => match req.await {
                Ok(_) => return CallResponse::ok(serde_json::json!({"ok": true})),
                Err(e) => return CallResponse::err(format!("discord send failed: {}", e)),
            },
            Err(e) => return CallResponse::err(format!("discord builder failed: {}", e)),
        }
    }

    CallResponse::err("invalid channel id format")
}

// ---------------------------------------------------------------------------
// typing
// ---------------------------------------------------------------------------

async fn typing(input: Option<Value>) -> CallResponse {
    let payload: TypingInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("typing requires input"),
    };

    let cfg   = CONFIG.merge(payload.config);
    let token = HotConfig::get_str(&cfg, "token");
    let duration = payload.duration_ms;

    if token.is_empty() {
        return CallResponse::err("discord token required");
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

    let channel_id_str = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve channel id".to_string())) },
    };

    if channel_id_str.is_empty() {
        return CallResponse::err("discord typing: missing destination");
    }

    if let Ok(channel_id_u64) = channel_id_str.parse::<u64>() {
        let cid = Id::<ChannelMarker>::new(channel_id_u64);
        
        // Autonomous background loop to keep the typing indicator alive
        tokio::spawn(async move {
            let http = HttpClient::new(token);
            let start_time = tokio::time::Instant::now();
            let duration_ms = if duration == 0 { 10000 } else { duration };

            while start_time.elapsed().as_millis() < duration_ms as u128 {
                let _ = http.create_typing_trigger(cid).await;
                // Discord typing lasts ~10s, so refresh every 7s
                tokio::time::sleep(tokio::time::Duration::from_secs(7)).await;
            }
        });

        return CallResponse::ok(serde_json::json!({"ok": true}));
    }

    CallResponse::err("invalid channel id format")
}

async fn start(input: Option<Value>) -> CallResponse {
    let payload: StartInput = match input {
        Some(v) => serde_json::from_value(v).unwrap_or_default(),
        None    => return CallResponse::err("start requires input"),
    };

    let cfg   = CONFIG.merge(payload.config);
    let token = HotConfig::get_str(&cfg, "token");

    if token.is_empty() {
        return CallResponse::err("discord token required");
    }

    let http = HttpClient::new(token.clone());

    let _bot_id = match http.current_user().await {
        Ok(user) => {
            let id = user.model().await.map(|u| u.id.to_string()).unwrap_or_else(|_| "unknown".to_string());
            emit_log("info", &format!("Discord bot validated. Bot ID: {}", id));
            id
        }
        Err(e) => return CallResponse::err(format!("failed to authenticate: {}", e)),
    };

    // Spawn gateway cluster for receiving events
    let token_clone = token.clone();
    tokio::spawn(async move {
        // Expand intents for maximal detection surface
        let intents = Intents::GUILD_MESSAGES 
            | Intents::DIRECT_MESSAGES 
            | Intents::MESSAGE_CONTENT
            | Intents::GUILD_MEMBERS
            | Intents::DIRECT_MESSAGE_REACTIONS;

        let (cluster, mut events) = match Cluster::builder(token_clone.clone(), intents).build().await {
            Ok(pair) => {
                emit_log("info", &format!("Created Discord cluster with {} shards", pair.0.shards().count()));
                pair
            },
            Err(e) => {
                emit_log("error", &format!("Failed to create Discord cluster: {}", e));
                return;
            }
        };

        // Start the cluster
        cluster.up().await;
        
        // Give the gateway time to stabilize
        tokio::time::sleep(std::time::Duration::from_secs(5)).await;

        emit_log("info", "Discord gateway cluster started and stabilized");

        // Process events
        while let Some((shard_id, event)) = events.next().await {
            // HIGH-TRANSPARENCY DIAGNOSTICS: Log raw event via Debug trait
            emit_log("debug", &format!("[RAW_GATEWAY] Shard {}: {:?}", shard_id, event));

            // LOG ALL EVENTS for diagnosis
            let kind = event.kind();
            emit_log("debug", &format!("[GATEWAY] Shard {}: Event received: {:?}", shard_id, kind));

            match event {
                Event::ShardDisconnected(ref payload) => {
                    emit_log("error", &format!("[GATEWAY] Shard {} DISCONNECTED: code={:?}, reason={:?}", 
                        shard_id, payload.code, payload.reason));
                }
                Event::MessageCreate(ref msg) => {
                    let author = msg.author.clone();
                    emit_log("info", &format!("Message Inbound: from={} content='{}'", author.id, msg.content));
                    
                    let mut attachments_json = Vec::new();
                    let mut audio_content = None;

                    for attachment in &msg.attachments {
                        // Safety limit: 10MB
                        if attachment.size > 10 * 1024 * 1024 {
                            emit_log("warn", &format!("Skipping Discord attachment {} ({} bytes): Too Large", attachment.filename, attachment.size));
                            continue;
                        }

                        emit_log("info", &format!("Downloading Discord attachment: {}", attachment.filename));
                        
                        let client = hyper::Client::builder().build::<_, hyper::Body>(hyper_rustls::HttpsConnectorBuilder::new()
                            .with_native_roots()
                            .https_or_http()
                            .enable_http1()
                            .build());

                        match hyper::Request::get(&attachment.proxy_url).body(hyper::Body::empty()) {
                            Ok(req) => {
                                match client.request(req).await {
                                    Ok(resp) => {
                                        if resp.status().is_success() {
                                            match hyper::body::to_bytes(resp.into_body()).await {
                                                Ok(bytes) => {
                                                    let b64 = BASE64.encode(&bytes);
                                                    let mime = attachment.content_type.clone().unwrap_or_else(|| "application/octet-stream".to_string());
                                                    
                                                    if audio_content.is_none() && mime.starts_with("audio/") {
                                                        audio_content = Some(serde_json::json!({
                                                            "data": b64.clone(),
                                                            "format": mime.split('/').last().unwrap_or("ogg").to_string(),
                                                            "platform_format": "discord_attachment"
                                                        }));
                                                    }

                                                    attachments_json.push(serde_json::json!({
                                                        "type": "binary",
                                                        "filename": attachment.filename,
                                                        "size": attachment.size,
                                                        "mime_type": mime,
                                                        "data": b64
                                                    }));
                                                }
                                                Err(e) => emit_log("error", &format!("Failed to read Discord attachment bytes: {}", e)),
                                            }
                                        } else {
                                            emit_log("error", &format!("Failed to download Discord attachment: HTTP {}", resp.status()));
                                        }
                                    }
                                    Err(e) => emit_log("error", &format!("Failed to request Discord attachment: {}", e)),
                                }
                            }
                            Err(e) => emit_log("error", &format!("Failed to build Discord download request: {}", e)),
                        }
                    }

                    emit_message(&serde_json::json!({
                        "channel_id": msg.channel_id.to_string(),
                        "sender_id": author.id.to_string(),
                        "content": msg.content,
                        "is_group": msg.guild_id.is_some(),
                        "timestamp": msg.timestamp.iso_8601(),
                        "attachments": attachments_json,
                        "audio": audio_content,
                    }));
                }
                _ => {}
            }
        }
    });

    CallResponse::ok(serde_json::json!({
        "status": "gateway_mode",
        "note": "Discord gateway cluster running - inbound messages will be emitted to core"
    }))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct DiscordPlugin;

#[async_trait]
impl Plugin for DiscordPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "Discord",
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
            "resolve_channel_id" => resolve_channel_id(input),
            "send"               => send(input).await,
            "send_voice"         => fn_send_voice(input).await,
            "typing"             => typing(input).await,
            "speaking"           => typing(input).await, // Fallback to typing for Discord
            "start"              => start(input).await,
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(DiscordPlugin).await;
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

    let cfg = input.get("config").cloned().unwrap_or(Value::Null);
    let token = resolve(&input, "token", "");

    if token.is_empty() {
        return CallResponse::err("discord token required");
    }

    let http = HttpClient::new(token);

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   input.get("message").and_then(|m| m.get("channel_id")),
            "recipient_id": input.get("message").and_then(|m| m.get("recipient_id")),
            "sender_id":    input.get("message").and_then(|m| m.get("sender_id")),
            "metadata":     input.get("message").and_then(|m| m.get("metadata"))
        }
    })));

    let channel_id_str = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse::err("failed to resolve channel id"),
    };

    let payload: SendInput = match serde_json::from_value(input) {
        Ok(p)  => p,
        Err(e) => return CallResponse::err(format!("invalid input: {}", e)),
    };

    if let Ok(channel_id_u64) = channel_id_str.parse::<u64>() {
        let cid = Id::<ChannelMarker>::new(channel_id_u64);

        if let Some(audio) = &payload.message.audio {
            if let Ok(bytes) = BASE64.decode(&audio.data) {
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
                        use twilight_model::channel::message::MessageFlags;
                        
                        let attachments = vec![twilight_model::http::attachment::Attachment {
                            description: None,
                            file: ogg_data,
                            filename: "voice.ogg".to_string(),
                            id: 0,
                        }];

                        // Using twilight's CreateMessage with flags. 
                        // Note: Flags 8192 (1 << 13) is IS_VOICE_MESSAGE.
                        // If the SDK version is too old for the constant, we use from_bits_truncate.
                        let mut builder = http.create_message(cid);
                        builder = builder.attachments(&attachments).unwrap();
                        builder = builder.flags(MessageFlags::from_bits_truncate(8192));
                        
                        if !payload.message.content.is_empty() {
                            builder = builder.content(&payload.message.content).unwrap();
                        }

                        match builder.await {
                            Ok(_) => return CallResponse::ok(serde_json::json!({"ok": true})),
                            Err(e) => return CallResponse::err(format!("discord voice send failed: {}", e)),
                        }
                    },
                    Err(e) => return CallResponse::err(format!("opus encoding failed: {}", e)),
                }
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
    // Last sample
    let last = input[input.len()-1];
    output.push(last);
    output.push(last);
    output.push(last);
    output
}

fn encode_opus_ogg(pcm: &[i16], sample_rate: u32) -> Result<Vec<u8>, String> {
    let mut encoder = OpusEncoder::new(sample_rate as i32, 1, Application::Voip)
        .map_err(|e| format!("encoder init failed: {:?}", e))?;
    
    let mut output_ogg = Vec::new();
    {
        let mut writer = PacketWriter::new(&mut output_ogg);
        
        // Opus Identification Header
        let mut id_header = Vec::new();
        id_header.extend_from_slice(b"OpusHead");
        id_header.push(1); // Version
        id_header.push(1); // Mono
        id_header.extend_from_slice(&0u16.to_le_bytes()); // Pre-skip
        id_header.extend_from_slice(&(sample_rate).to_le_bytes());
        id_header.extend_from_slice(&0i16.to_le_bytes()); // Output gain
        id_header.push(0); // Mapping family
        
        writer.write_packet(id_header, 1, ogg::PacketWriteEndInfo::EndPage, 0)
            .map_err(|e| format!("ogg id header failed: {}", e))?;
            
        // Opus Comment Header
        let mut comment_header = Vec::with_capacity(100);
        comment_header.extend_from_slice(b"OpusTags");
        let vendor = b"openlobster-rs";
        comment_header.extend_from_slice(&(vendor.len() as u32).to_le_bytes());
        comment_header.extend_from_slice(vendor);
        comment_header.extend_from_slice(&0u32.to_le_bytes()); // No tags
        
        writer.write_packet(comment_header, 1, ogg::PacketWriteEndInfo::EndPage, 0)
            .map_err(|e| format!("ogg comment header failed: {}", e))?;

        // Encode frames (20ms = 960 samples @ 48kHz)
        let frame_size = 960;
        let mut abs_granule = 0;
        for chunk in pcm.chunks_exact(frame_size) {
            let f32_chunk: Vec<f32> = chunk.iter().map(|&s| s as f32 / 32768.0).collect();
            let mut encoded = vec![0u8; 1024];
            let len = encoder.encode(&f32_chunk, frame_size, &mut encoded)
                .map_err(|e| format!("encode failed: {:?}", e))?;
            encoded.truncate(len);
            
            abs_granule += frame_size as u64;
            writer.write_packet(encoded, 1, ogg::PacketWriteEndInfo::EndPage, abs_granule)
                .map_err(|e| format!("ogg data packet failed: {}", e))?;
        }
    }
    
    Ok(output_ogg)
}
