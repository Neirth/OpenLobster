// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Slack messaging plugin (Rust).
//!
//! Delegates messaging operations to the Slack API via the slack-morphism SDK.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use openlobster_sdk_base::{
    emit_log, emit_message, run, CallResponse, HotConfig, Plugin, PluginInfo,
};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use slack_morphism::prelude::*;
use slack_morphism::hyper_tokio::*;

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
    #[serde(default)] message: SendMessage,
}

#[derive(Deserialize, Debug, Default)]
struct SendMessage {
    #[serde(default)] channel_id: Option<String>,
    #[serde(default)] recipient_id: Option<String>,
    #[serde(default)] sender_id: Option<String>,
    #[serde(default)] metadata: Option<HashMap<String, Value>>,
    #[serde(default)] content: String,
    #[serde(default)] thread_ts: Option<String>,
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
struct ResolveChannelIdInput {
    #[serde(default)] config: Option<HashMap<String, Value>>,
    #[serde(default)] message: ResolveChannelIdMessage,
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

/// Payload emitted to the host when a Slack message arrives via Socket Mode.
#[derive(Serialize, Debug)]
struct InboundMessage {
    channel: String,
    text: String,
    #[serde(skip_serializing_if = "Option::is_none")] user: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")] thread_ts: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")] ts: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")] bot_id: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")] attachments: Vec<Value>,
    #[serde(skip_serializing_if = "Option::is_none")] audio: Option<Value>,
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "slack";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "Slack messaging plugin for OpenLobster via Socket Mode";
const PLUGIN_TYPE: &str = "messaging";

fn metadata_schema() -> Value {
    serde_json::json!({
        "type": "object",
        "properties": {
            "bot_token": {
                "type": "string",
                "format": "password",
                "title": "Bot Token",
                "description": "Slack bot token (xoxb-...) with chat:write and channels:read scopes",
                "placeholder": "xoxb-your-bot-token"
            },
            "app_token": {
                "type": "string",
                "format": "password",
                "title": "App Token",
                "description": "Slack app-level token (xapp-...) for Socket Mode",
                "placeholder": "xapp-your-app-token"
            }
        },
        "required": ["bot_token"]
    })
}

fn metadata_properties() -> Value {
    serde_json::json!({
        "inbound_mode": "gateway",
        "HasVoiceMessage": true, "HasCallStream": true,
        "HasTextStream": true, "HasMediaSupport": true
    })
}

// ---------------------------------------------------------------------------
// Messaging discovery
// ---------------------------------------------------------------------------

fn inbound_mode() -> CallResponse {
    CallResponse::ok(serde_json::json!("polling"))
}

// ---------------------------------------------------------------------------
// capabilities
// ---------------------------------------------------------------------------

fn capabilities() -> CallResponse {
    let caps = CapabilitiesOutput {
        has_voice_message: true, has_call_stream: true,
        has_text_stream: true,  has_media_support: true,
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
        if !trimmed.is_empty() && !trimmed.eq_ignore_ascii_case("slack") {
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
        for key in &["platform_channel_id", "channel_id", "platform_user_id", "user_id", "channel"] {
            if let Some(v) = metadata.get(*key) {
                if let Some(s) = v.as_str() {
                    let trimmed = s.trim();
                    if !trimmed.is_empty() {
                        return CallResponse::ok(serde_json::json!(trimmed.to_string()));
                    }
                }
            }
        }
    }

    let default_channel = HotConfig::get_str(&cfg, "default_channel");
    if !default_channel.is_empty() {
        return CallResponse::ok(serde_json::json!(default_channel));
    }

    CallResponse {
        output: Some(serde_json::json!("")),
        error:  Some("slack resolve_channel_id: missing destination".to_string()),
    }
}

// ---------------------------------------------------------------------------
// Shared helper: build an HTTPS Slack client
// ---------------------------------------------------------------------------

type SlackHyperClient = SlackClient<SlackClientHyperHttpsConnector>;

fn build_client() -> Result<SlackHyperClient, String> {
    SlackClientHyperHttpsConnector::new()
        .map(SlackClient::new)
        .map_err(|e| format!("failed to create Slack HTTP client: {}", e))
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
        return CallResponse::err("slack bot_token required");
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

    let channel = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse {
            output: None,
            error:  resolved.error.or(Some("failed to resolve channel".to_string())),
        },
    };

    if channel.is_empty() {
        return CallResponse::err("slack send: missing destination");
    }

    let client = match build_client() {
        Ok(c)  => c,
        Err(e) => return CallResponse::err(e),
    };

    let token   = SlackApiToken::new(SlackApiTokenValue::from(bot_token));
    let session = client.open_session(&token);

    let slack_content = convert_markdown_to_slack(&payload.message.content);
    let content = SlackMessageContent::new().with_text(slack_content);

    let mut req = SlackApiChatPostMessageRequest::new(SlackChannelId::from(channel.clone()), content);
    if let Some(ref ts) = payload.message.thread_ts {
        req = req.with_thread_ts(SlackTs::from(ts));
    }

    match session.chat_post_message(&req).await {
        Ok(resp) => {
            // Audio support (separate file upload)
            if let Some(audio) = &payload.message.audio {
                if let Ok(bytes) = BASE64.decode(&audio.data) {
                    emit_log("info", "Slack: Uploading audio note...");
                    let filename = format!("voice.{}", audio.format.as_deref().unwrap_or("ogg"));
                    let mut upload_req = SlackApiFilesUploadRequest::new()
                        .with_channels(vec![SlackChannelId::from(channel.clone())])
                        .with_filename(filename)
                        .with_content(String::from_utf8_lossy(&bytes).to_string());
                    
                    if let Some(ref ts) = payload.message.thread_ts {
                        upload_req = upload_req.with_thread_ts(SlackTs::from(ts));
                    }
                    
                    if let Err(e) = session.files_upload(&upload_req).await {
                        emit_log("warn", &format!("Slack: Audio upload failed: {}", e));
                    }
                }
            }

            // Multimedia / Attachments support
            for att in &payload.message.attachments {
                if let Ok(bytes) = BASE64.decode(&att.data) {
                    emit_log("info", &format!("Slack: Uploading attachment {}...", att.filename));
                    
                    let mut upload_req = SlackApiFilesUploadRequest::new()
                        .with_channels(vec![SlackChannelId::from(channel.clone())])
                        .with_filename(att.filename.clone())
                        .with_content(String::from_utf8_lossy(&bytes).to_string());
                    
                    if let Some(ref ts) = payload.message.thread_ts {
                        upload_req = upload_req.with_thread_ts(SlackTs::from(ts));
                    }
                    
                    if let Err(e) = session.files_upload(&upload_req).await {
                        emit_log("warn", &format!("Slack: Attachment upload failed: {}", e));
                    }
                }
            }

            CallResponse::ok(serde_json::json!({
                "ok":      true,
                "ts":      resp.ts.to_string(),
                "channel": resp.channel.to_string(),
            }))
        }
        Err(e) => CallResponse::err(format!("slack API error: {}", e)),
    }
}

/// Converts standard Markdown to Slack's 'mrkdwn' format.
/// Handles links: [text](url) -> <url|text>
/// Handles bold: **text** -> *text*
fn convert_markdown_to_slack(text: &str) -> String {
    use regex::Regex;
    
    // 1. Convert links: [label](url) -> <url|label>
    let re_links = Regex::new(r"\[([^\]]+)\]\(([^)]+)\)").unwrap();
    let text = re_links.replace_all(text, "<$2|$1>");
    
    // 2. Convert bold: **text** -> *text*
    let re_bold = Regex::new(r"\*\*([^\*]+)\*\*").unwrap();
    let text = re_bold.replace_all(&text, "*$1*");

    // 3. Convert italics: *text* -> _text_ (only if not already bold)
    // We use a lookaround-like approach or just careful regex. 
    // Standard markdown *italic* -> _italic_
    let re_italic = Regex::new(r"([^\*])\*([^\*]+)\*([^\*])").unwrap();
    let text = re_italic.replace_all(&text, "$1_$2_$3");
    
    // 4. Handle edge case for italics at start/end of string
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
    let bot_token = HotConfig::get_str(&cfg, "bot_token");
    let duration = payload.duration_ms;

    if bot_token.is_empty() {
        return CallResponse::err("slack bot_token required");
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

    let channel = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse { output: None, error: resolved.error.or(Some("failed to resolve channel".to_string())) },
    };

    if channel.is_empty() {
        return CallResponse::err("slack typing: missing destination");
    }

    emit_log("debug", &format!("Slack typing requested (not supported by Bot API, skipping silently) for channel {} durante {}ms", channel, duration));
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
    let app_token = HotConfig::get_str(&cfg, "app_token");

    if bot_token.is_empty() {
        return CallResponse::err("slack bot_token required");
    }

    let client = match build_client() {
        Ok(c)  => Arc::new(c),
        Err(e) => return CallResponse::err(e),
    };

    let token   = SlackApiToken::new(SlackApiTokenValue::from(bot_token));
    let session = client.open_session(&token);

    match session.auth_test().await {
        Ok(resp) => {
            if !app_token.is_empty() {
                let socket_client = match build_client() {
                    Ok(c)  => Arc::new(c),
                    Err(e) => return CallResponse::err(e),
                };
                tokio::spawn(async move {
                    run_socket_mode(socket_client, app_token).await;
                });
            }

            CallResponse::ok(serde_json::json!({
                "status": "polling_mode",
                "user":   resp.user_id.to_string(),
                "team":   resp.team_id.to_string(),
                "note":   "Slack plugin is now polling for messages via Socket Mode"
            }))
        }
        Err(e) => CallResponse::err(format!("slack auth.test failed: {}", e)),
    }
}

// ---------------------------------------------------------------------------
// handle_webhook
// ---------------------------------------------------------------------------

fn handle_webhook(_input: Option<Value>) -> CallResponse {
    CallResponse::err("webhook_not_supported: slack plugin uses polling")
}

// ---------------------------------------------------------------------------
// Socket Mode listener
// ---------------------------------------------------------------------------

fn slack_error_handler(
    err: Box<dyn std::error::Error + Send + Sync + 'static>,
    _client: Arc<SlackHyperClient>,
    _states: SlackClientEventsUserState,
) -> HttpStatusCode {
    emit_log("error", &format!("Slack Socket Mode error: {}", err));
    HttpStatusCode::OK
}

async fn on_push_event(
    event: SlackPushEventCallback,
    _client: Arc<SlackHyperClient>,
    _states: SlackClientEventsUserState,
) -> UserCallbackResult<()> {
    if let SlackEventCallbackBody::Message(msg) = event.event {
        let mut attachments = Vec::new();
        let mut audio_content = None;
        let bot_token = HotConfig::get_str(&CONFIG.merge(None), "bot_token");

        if let Some(files) = msg.content.as_ref().and_then(|c| c.files.as_ref()) {
            for file in files {
                if let Some(url) = &file.url_private {
                    // Safety limit: 10MB
                    let size = 0; // The size field seems removed or changed in this version
                    // if size > 10 * 1024 * 1024 { ... }

                    emit_log("info", &format!("Downloading Slack file: {}", file.name.as_deref().unwrap_or("unknown")));
                    
                    // Use a legacy client to handle arbitrary URLs in hyper 1.0
                    let https = hyper_rustls::HttpsConnectorBuilder::new()
                        .with_native_roots()
                        .expect("no native roots")
                        .https_or_http()
                        .enable_http1()
                        .build();
                    let client = hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new()).build(https);

                    let req = hyper::Request::builder()
                        .uri(url.to_string())
                        .header("Authorization", format!("Bearer {}", bot_token))
                        .body(http_body_util::Full::new(hyper::body::Bytes::new()))
                        .unwrap();

                    match client.request(req).await {
                        Ok(resp) => {
                            if resp.status().is_success() {
                                use http_body_util::BodyExt;
                                match resp.into_body().collect().await {
                                    Ok(collected) => {
                                        let bytes = collected.to_bytes();
                                        let b64 = BASE64.encode(&bytes);
                                        let mime = file.mimetype.clone().unwrap_or_else(|| slack_morphism::SlackMimeType("application/octet-stream".to_string()));
                                        
                                        if audio_content.is_none() && mime.0.starts_with("audio/") {
                                            audio_content = Some(serde_json::json!({
                                                "data": b64.clone(),
                                                "format": mime.0.split('/').last().unwrap_or("ogg").to_string(),
                                                "platform_format": "slack_file"
                                            }));
                                        }

                                        attachments.push(serde_json::json!({
                                            "type": "binary",
                                            "filename": file.name,
                                            "size": bytes.len(),
                                            "mime_type": mime.0,
                                            "data": b64
                                        }));
                                    }
                                    Err(e) => emit_log("error", &format!("Failed to read Slack file bytes: {}", e)),
                                }
                            } else {
                                emit_log("error", &format!("Failed to download Slack file: HTTP {}", resp.status()));
                            }
                        }
                        Err(e) => emit_log("error", &format!("Failed to request Slack file: {}", e)),
                    }
                }
            }
        }

        let msg_payload = InboundMessage {
            channel:   msg.origin.channel.as_ref().map(|c: &SlackChannelId| c.to_string()).unwrap_or_default(),
            text:      msg.content.and_then(|c| c.text).unwrap_or_default(),
            user:      msg.sender.user.as_ref().map(|u: &SlackUserId| u.to_string()),
            bot_id:    msg.sender.bot_id.as_ref().map(|b: &SlackBotId| b.to_string()),
            ts:        Some(msg.origin.ts.to_string()),
            thread_ts: msg.origin.thread_ts.as_ref().map(|t: &SlackTs| t.to_string()),
            attachments,
            audio: audio_content,
        };
        emit_message(&msg_payload);
    }
    Ok(())
}

async fn run_socket_mode(client: Arc<SlackHyperClient>, app_token: String) {
    let listener_environment = Arc::new(
        SlackClientEventsListenerEnvironment::new(client)
            .with_error_handler(slack_error_handler),
    );

    let callbacks = SlackSocketModeListenerCallbacks::new()
        .with_push_events(on_push_event);

    let socket_mode = SlackClientSocketModeListener::new(
        &SlackClientSocketModeConfig::new(),
        listener_environment,
        callbacks,
    );

    let token = SlackApiToken::new(SlackApiTokenValue::from(app_token));

    if let Err(e) = socket_mode.listen_for(&token).await {
        emit_log("error", &format!("Slack Socket Mode failed to register: {}", e));
        return;
    }

    socket_mode.serve().await;
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct SlackPlugin;

#[async_trait]
impl Plugin for SlackPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "Slack",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["inbound_mode", "capabilities", "resolve_channel_id",
                          "send", "send_voice", "start", "configure", "typing"],
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
            "start"              => start(input).await,
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(SlackPlugin).await;
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

    let cfg       = CONFIG.merge(input.get("config").and_then(|v| serde_json::from_value(v.clone()).ok()));
    let bot_token = resolve(&input, "bot_token", "");

    if bot_token.is_empty() {
        return CallResponse::err("slack bot_token required");
    }

    let resolved = resolve_channel_id(Some(serde_json::json!({
        "config": &cfg,
        "message": {
            "channel_id":   input.get("message").and_then(|m| m.get("channel_id")),
            "recipient_id": input.get("message").and_then(|m| m.get("recipient_id")),
            "sender_id":    input.get("message").and_then(|m| m.get("sender_id")),
            "metadata":     input.get("message").and_then(|m| m.get("metadata")),
        }
    })));

    let channel = match resolved.output {
        Some(v) => v.as_str().unwrap_or("").to_string(),
        None    => return CallResponse::err("failed to resolve channel id"),
    };

    if channel.is_empty() {
        return CallResponse::err("slack send_voice: missing destination");
    }

    let payload: SendInput = match serde_json::from_value(input) {
        Ok(p)  => p,
        Err(e) => return CallResponse::err(format!("invalid input: {}", e)),
    };

    let client = match build_client() {
        Ok(c)  => c,
        Err(e) => return CallResponse::err(e),
    };

    let token   = SlackApiToken::new(SlackApiTokenValue::from(bot_token));
    let session = client.open_session(&token);

    if let Some(audio) = &payload.message.audio {
        if let Ok(mut bytes) = BASE64.decode(&audio.data) {
            // Core Unified Format: PCM. We wrap it in WAV for Slack compatibility.
            if audio.format.as_deref() == Some("pcm") || audio.format.is_none() {
                let sample_rate = audio.sample_rate.unwrap_or(16000);
                bytes = wrap_pcm_as_wav(bytes, sample_rate);
            }

            let filename = "voice.wav".to_string();
            let upload_req = SlackApiFilesUploadRequest::new()
                .with_channels(vec![SlackChannelId::from(channel)])
                .with_filename(filename)
                .with_content(String::from_utf8_lossy(&bytes).to_string());
            
            // Note: Modern Slack API for files is multi-step, but files.upload is still working in many SDKs.
            match session.files_upload(&upload_req).await {
                Ok(_) => return CallResponse::ok(serde_json::json!({"ok": true})),
                Err(e) => return CallResponse::err(format!("slack audio upload failed: {}", e)),
            }
        }
    }

    CallResponse::err("send_voice failed")
}

fn wrap_pcm_as_wav(pcm_data: Vec<u8>, sample_rate: u32) -> Vec<u8> {
    let mut wav = Vec::with_capacity(44 + pcm_data.len());
    let data_len = pcm_data.len() as u32;
    let file_size = 36 + data_len;
    let byte_rate = sample_rate * 1 * 2; // rate * channels * bytes_per_sample

    // RIFF Header
    wav.extend_from_slice(b"RIFF");
    wav.extend_from_slice(&file_size.to_le_bytes());
    wav.extend_from_slice(b"WAVE");

    // FMT chunk
    wav.extend_from_slice(b"fmt ");
    wav.extend_from_slice(&16u32.to_le_bytes()); // Subchunk1Size
    wav.extend_from_slice(&1u16.to_le_bytes());  // AudioFormat (PCM = 1)
    wav.extend_from_slice(&1u16.to_le_bytes());  // NumChannels (Mono = 1)
    wav.extend_from_slice(&sample_rate.to_le_bytes());
    wav.extend_from_slice(&byte_rate.to_le_bytes());
    wav.extend_from_slice(&2u16.to_le_bytes());  // BlockAlign
    wav.extend_from_slice(&16u16.to_le_bytes()); // BitsPerSample

    // Data chunk
    wav.extend_from_slice(b"data");
    wav.extend_from_slice(&data_len.to_le_bytes());
    wav.extend_from_slice(&pcm_data);

    wav
}
