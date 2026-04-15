// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster Ditto mock plugin (Rust).
//!
//! A single binary that impersonates any OpenLobster plugin type by reading its
//! own executable name at startup.  Rename or symlink the binary following the
//! standard project convention and the plugin will self-configure accordingly:
//!
//! ```text
//! openlobster-ai-ditto       →  type: ai
//! openlobster-messages-ditto →  type: messaging
//! openlobster-memory-ditto   →  type: memory
//! openlobster-secrets-ditto  →  type: secrets
//! openlobster-audio-ditto    →  type: audio
//! ```
//!
//! The plugin also reports its `id` and `name` from the binary stem so that the
//! core registers it under the correct identity.
//!
//! ## Mode resolution order (highest to lowest priority)
//!
//! 1. Hot config  — `configure({ "config": { "mode": "messaging" } })`
//! 2. Binary name — segment scan of the executable stem
//! 3. `DITTO_MODE` environment variable — for development / CI overrides
//! 4. Default     — `ai`

use async_trait::async_trait;
use openlobster_sdk_base::{emit_log, run, CallResponse, HotConfig, Plugin, PluginInfo};
use serde_json::{json, Value};
use std::collections::HashMap;
use std::path::Path;
use std::sync::{Arc, LazyLock, Mutex};

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// In-memory store — shared by memory and secrets mode implementations
// ---------------------------------------------------------------------------

static STORE: LazyLock<Arc<Mutex<HashMap<String, Value>>>> =
    LazyLock::new(|| Arc::new(Mutex::new(HashMap::new())));

// ---------------------------------------------------------------------------
// Binary identity — resolved once at startup from the executable path
// ---------------------------------------------------------------------------

/// Stem of the running executable (path and extension stripped, `-rust` suffix
/// removed to match the project naming convention).
static BINARY_STEM: LazyLock<String> = LazyLock::new(|| {
    let raw = std::env::current_exe()
        .ok()
        .and_then(|p| p.file_stem().and_then(|s| s.to_str()).map(str::to_string))
        .or_else(|| {
            std::env::args()
                .next()
                .and_then(|a| {
                    Path::new(&a)
                        .file_stem()
                        .and_then(|s| s.to_str())
                        .map(str::to_string)
                })
        })
        .unwrap_or_else(|| "openlobster-ditto".to_string());

    // Strip the `-rust` toolchain suffix used across the project.
    if raw.ends_with("-rust") {
        raw[..raw.len() - 5].to_string()
    } else {
        raw
    }
});

/// Leaked `&'static str` of the binary stem — used directly in `PluginInfo`.
static EFFECTIVE_ID: LazyLock<&'static str> =
    LazyLock::new(|| Box::leak(BINARY_STEM.clone().into_boxed_str()));

/// Mode inferred from the binary name segments, resolved at startup.
static BINARY_MODE: LazyLock<&'static str> =
    LazyLock::new(|| detect_mode_from_name(&BINARY_STEM));

// ---------------------------------------------------------------------------
// Mode helpers
// ---------------------------------------------------------------------------

/// Scans the `-`-separated segments of a binary name for a known type keyword
/// and returns the corresponding canonical mode string.
///
/// Follows the same segment position used by the project convention:
/// `openlobster-{type}-{provider}`.
fn detect_mode_from_name(name: &str) -> &'static str {
    for seg in name.split('-') {
        match seg {
            "ai"                    => return "ai",
            "messages" | "messaging" => return "messaging",
            "memory"   | "mem"       => return "memory",
            "secrets"  | "secret"    => return "secrets",
            "audio"                  => return "audio",
            _                        => {}
        }
    }
    // No recognisable type segment — defer to env var or default.
    let env = std::env::var("DITTO_MODE").unwrap_or_default();
    mode_as_static(env.trim())
}

/// Maps an arbitrary mode string to its canonical `&'static str` counterpart,
/// defaulting to `"ai"` for unknown values.
fn mode_as_static(mode: &str) -> &'static str {
    match mode {
        "messaging" | "messages" | "msg" => "messaging",
        "memory"    | "mem"              => "memory",
        "secrets"   | "secret"           => "secrets",
        "audio"                          => "audio",
        "ai"                             => "ai",
        _                                => "ai",
    }
}

/// Returns the currently active mode.
///
/// Hot config overrides everything so that a `configure` call during a test
/// session can switch personalities without restarting the binary.
fn current_mode() -> &'static str {
    let cfg = CONFIG.merge(None);
    let hot = HotConfig::get_str(&cfg, "mode");
    if hot.is_empty() {
        *BINARY_MODE
    } else {
        mode_as_static(&hot)
    }
}

// ---------------------------------------------------------------------------
// Per-mode exports
// ---------------------------------------------------------------------------

fn exports_for(mode: &str) -> Vec<&'static str> {
    match mode {
        "ai" => vec![
            "configure", "get_metadata",
            "chat",
        ],
        "messaging" => vec![
            "configure", "get_metadata",
            "capabilities", "inbound_mode", "resolve_channel_id", "send", "start",
        ],
        "memory" => vec![
            "configure", "get_metadata",
            "store", "retrieve", "query",
        ],
        "secrets" => vec![
            "configure", "get_metadata",
            "get", "set", "delete", "list",
        ],
        "audio" => vec![
            "configure", "get_metadata",
            "tts", "stt",
        ],
        _ => vec!["configure", "get_metadata"],
    }
}

// ---------------------------------------------------------------------------
// Per-mode schema
// ---------------------------------------------------------------------------

fn metadata_schema(mode: &str) -> Value {
    match mode {
        "ai" => json!({
            "type": "object",
            "properties": {
                "mode":    {"type": "string"},
                "api_key": {"type": "string"},
                "model":   {"type": "string"}
            }
        }),
        "messaging" => json!({
            "type": "object",
            "properties": {
                "mode":               {"type": "string"},
                "token":              {"type": "string"},
                "default_channel_id": {"type": "string"}
            }
        }),
        "memory" => json!({
            "type": "object",
            "properties": {
                "mode":     {"type": "string"},
                "uri":      {"type": "string"},
                "username": {"type": "string"},
                "password": {"type": "string"}
            }
        }),
        "secrets" => json!({
            "type": "object",
            "properties": {
                "mode": {"type": "string"},
                "path": {"type": "string"},
                "key":  {"type": "string"}
            }
        }),
        "audio" => json!({
            "type": "object",
            "properties": {
                "mode":             {"type": "string"},
                "api_key":          {"type": "string"},
                "default_voice_id": {"type": "string"},
                "default_model_id": {"type": "string"}
            }
        }),
        _ => json!({"type": "object", "properties": {"mode": {"type": "string"}}}),
    }
}

fn metadata_properties(mode: &str) -> Value {
    match mode {
        "ai" => json!({
            "supports_audio_input":  false,
            "supports_audio_output": false
        }),
        "messaging" => json!({
            "HasVoiceMessage": false,
            "HasCallStream":   false,
            "HasTextStream":   true,
            "HasMediaSupport": false
        }),
        "audio" => json!({"SupportsTTS": true, "SupportsSTT": true}),
        _       => json!({}),
    }
}

fn get_metadata() -> CallResponse {
    let mode = current_mode();
    CallResponse::ok(json!({
        "id":          *EFFECTIVE_ID,
        "name":        *EFFECTIVE_ID,
        "version":     env!("CARGO_PKG_VERSION"),
        "description": format!("Ditto mock — impersonating {} plugin type", mode),
        "type":        mode,
        "schema":      metadata_schema(mode),
        "properties":  metadata_properties(mode)
    }))
}

// ---------------------------------------------------------------------------
// AI — chat
// ---------------------------------------------------------------------------

fn ai_chat(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);

    let echo = input
        .get("messages")
        .and_then(Value::as_array)
        .and_then(|msgs| {
            msgs.iter()
                .rev()
                .find(|m| m.get("role").and_then(Value::as_str) == Some("user"))
                .and_then(|m| m.get("content").and_then(Value::as_str))
        })
        .unwrap_or("(no user message)")
        .to_string();

    let model = input
        .get("model")
        .and_then(Value::as_str)
        .unwrap_or("ditto-model")
        .to_string();

    emit_log("debug", &format!("ditto chat: model={}, echo={:?}", model, echo));

    CallResponse::ok(json!({
        "content":     format!("[ditto] {}", echo),
        "tool_calls":  [],
        "stop_reason": "stop",
        "usage":       {"prompt_tokens": 1, "completion_tokens": 1}
    }))
}

// ---------------------------------------------------------------------------
// Messaging — capabilities / inbound_mode / resolve_channel_id / send / start
// ---------------------------------------------------------------------------

fn messaging_capabilities() -> CallResponse {
    CallResponse::ok(json!({
        "HasVoiceMessage": false,
        "HasCallStream":   false,
        "HasTextStream":   true,
        "HasMediaSupport": false
    }))
}

fn messaging_inbound_mode() -> CallResponse {
    CallResponse::ok(json!("polling"))
}

fn messaging_resolve_channel_id(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);

    let channel = input
        .get("message")
        .and_then(|m| m.get("channel_id"))
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
        .unwrap_or("ditto-channel");

    CallResponse::ok(json!(channel))
}

fn messaging_send(input: Option<Value>) -> CallResponse {
    let input   = input.unwrap_or(Value::Null);
    let content = input
        .get("message")
        .and_then(|m| m.get("content"))
        .and_then(Value::as_str)
        .unwrap_or("");

    emit_log("info", &format!("ditto send: {:?}", content));
    CallResponse::ok(json!({"ok": true, "mock": true}))
}

fn messaging_start(input: Option<Value>) -> CallResponse {
    let _ = input;
    emit_log("info", "ditto messaging start: mock gateway active");
    CallResponse::ok(json!({
        "status": "mock_gateway",
        "note":   "Ditto mock messaging started — no real external connection"
    }))
}

// ---------------------------------------------------------------------------
// Memory — store / retrieve / query
// ---------------------------------------------------------------------------

fn str_field<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(Value::as_str).unwrap_or("")
}

fn memory_store(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);

    match str_field(&input, "op") {
        "add_relation" | "delete_relation" | "invalidate_cache" => {
            CallResponse::ok(json!({"ok": true}))
        }
        _ => {
            let content = str_field(&input, "content").to_string();
            let node_id = {
                let mut store = STORE.lock().unwrap();
                let id = format!("ditto-{}", store.len() + 1);
                store.insert(id.clone(), json!({"content": content}));
                id
            };
            emit_log("debug", &format!("ditto store: id={}", node_id));
            CallResponse::ok(json!([node_id]))
        }
    }
}

fn memory_retrieve(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);
    let query = str_field(&input, "query").to_lowercase();
    let limit = input.get("limit").and_then(Value::as_i64).unwrap_or(64) as usize;

    let store = STORE.lock().unwrap();
    let items: Vec<Value> = store
        .iter()
        .filter(|(_, v)| {
            let content = v.get("content")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_lowercase();
            query.is_empty() || content.contains(&query)
        })
        .take(limit)
        .map(|(k, v)| json!({"id": k, "content": v["content"]}))
        .collect();

    CallResponse::ok(Value::Array(items))
}

fn memory_query(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);

    match str_field(&input, "op") {
        "cypher" => CallResponse::ok(json!({"data": [], "errors": []})),
        _ => {
            let store = STORE.lock().unwrap();
            let edges: Vec<Value> = store
                .keys()
                .map(|k| json!({"source": "ditto", "target": k, "label": "HAS_FACT"}))
                .collect();
            CallResponse::ok(json!({"edges": edges}))
        }
    }
}

// ---------------------------------------------------------------------------
// Secrets — get / set / delete / list
// ---------------------------------------------------------------------------

fn secrets_get(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);
    let key   = str_field(&input, "key");
    if key.is_empty() { return CallResponse::err("key is required"); }

    let store = STORE.lock().unwrap();
    match store.get(key) {
        Some(v) => CallResponse::ok(json!({"value": v, "found": true})),
        None    => CallResponse::ok(json!({"found": false})),
    }
}

fn secrets_set(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);
    let key   = str_field(&input, "key").to_string();
    if key.is_empty() { return CallResponse::err("key is required"); }

    let value = input.get("value").cloned().unwrap_or(Value::Null);
    STORE.lock().unwrap().insert(key, value);
    CallResponse::ok(json!({"ok": true}))
}

fn secrets_delete(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);
    let key   = str_field(&input, "key").to_string();
    if key.is_empty() { return CallResponse::err("key is required"); }

    STORE.lock().unwrap().remove(&key);
    CallResponse::ok(json!({"ok": true}))
}

fn secrets_list(input: Option<Value>) -> CallResponse {
    let input  = input.unwrap_or(Value::Null);
    let prefix = str_field(&input, "prefix").to_string();

    let store = STORE.lock().unwrap();
    let mut keys: Vec<String> = store
        .keys()
        .filter(|k| prefix.is_empty() || k.starts_with(&prefix))
        .cloned()
        .collect();
    keys.sort();
    CallResponse::ok(json!({"keys": keys}))
}

// ---------------------------------------------------------------------------
// Audio — tts / stt
// ---------------------------------------------------------------------------

// Minimal silent MPEG frame in base64 — sufficient for interface validation
// without requiring a real TTS service.
const MOCK_AUDIO_B64: &str =
    "SUQzBAAAAAAAI1RTU0UAAAAPAAADTGF2ZjU4LjI5LjEwMAAAAAAAAAAAAAAA\
     //tQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWGluZwAAAA8AAAAC\
     AAADhABVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVV";

fn audio_tts(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);
    let text  = str_field(&input, "text");
    if text.is_empty() { return CallResponse::err("text required"); }

    emit_log("debug", &format!("ditto tts: {:?}", text));
    CallResponse::ok(json!({"audio": MOCK_AUDIO_B64, "format": "mp3"}))
}

fn audio_stt(input: Option<Value>) -> CallResponse {
    let input = input.unwrap_or(Value::Null);
    let audio = str_field(&input, "audio");
    if audio.is_empty() { return CallResponse::err("audio (base64) required"); }

    emit_log("debug", "ditto stt: returning mock transcription");
    CallResponse::ok(json!({
        "text":       "[ditto] mock transcription",
        "confidence": 1.0,
        "language":   "en"
    }))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct DittoPlugin;

#[async_trait]
impl Plugin for DittoPlugin {
    fn info(&self) -> PluginInfo {
        let mode = current_mode();
        PluginInfo {
            id:          *EFFECTIVE_ID,
            name:        *EFFECTIVE_ID,
            version:     env!("CARGO_PKG_VERSION"),
            description: "Ditto mock — impersonates any OpenLobster plugin type",
            plugin_type: mode,
            schema:      metadata_schema(mode),
            properties:  metadata_properties(mode),
            exports:     exports_for(mode),
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            // Common
            "configure"          => CONFIG.configure(input),
            "get_metadata"       => get_metadata(),
            // AI
            "chat"               => ai_chat(input),
            // Messaging
            "capabilities"       => messaging_capabilities(),
            "inbound_mode"       => messaging_inbound_mode(),
            "resolve_channel_id" => messaging_resolve_channel_id(input),
            "send"               => messaging_send(input),
            "start"              => messaging_start(input),
            // Memory
            "store"              => memory_store(input),
            "retrieve"           => memory_retrieve(input),
            "query"              => memory_query(input),
            // Secrets
            "get"                => secrets_get(input),
            "set"                => secrets_set(input),
            "delete"             => secrets_delete(input),
            "list"               => secrets_list(input),
            // Audio
            "tts"                => audio_tts(input),
            "stt"                => audio_stt(input),
            other                => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    emit_log(
        "info",
        &format!(
            "ditto starting: id={}, mode={}",
            *EFFECTIVE_ID,
            current_mode()
        ),
    );
    run(DittoPlugin).await;
}
