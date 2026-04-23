// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster ElevenLabs audio plugin (Rust).
//!
//! Uses reqwest (rustls-tls) to call the ElevenLabs REST API for TTS and STT.

use async_trait::async_trait;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use openlobster_sdk_base::{run, CallResponse, HotConfig, Plugin, PluginInfo};
use serde_json::{json, Value};

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "elevenlabs";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "ElevenLabs high-quality text-to-speech and speech-to-text plugin";
const PLUGIN_TYPE: &str = "audio";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "api_key": {
                "type": "string",
                "title": "API Key",
                "description": "Your ElevenLabs API key from the dashboard profile",
                "placeholder": "Enter your ElevenLabs key"
            },
            "default_voice_id": {
                "type": "string",
                "title": "Default Voice",
                "description": "Select the default high-quality voice for TTS",
                "default": "HmCe1ONH6vWxNYuZpe8K",
                "enum": [
                    "Ir1QNHvhaJXbAGhT50w3",
                    "Nh2zY9kknu6z4pZy6FhD",
                    "PcAHoDMdlTbdDxdz24IK",
                    "h3l1RP4XfcWsPwoRp9G6",
                    "gI3qgA0k3Ab38AoIEGXI",
                    "o0SveC0zgHFuCsEO3vHR",
                    "HmCe1ONH6vWxNYuZpe8K"
                ],
                "x-enum-labels": [
                    "Sara Martin",
                    "Brian",
                    "David Gaspar",
                    "Sheila",
                    "Carla",
                    "Gabo",
                    "Sam"
                ]
            },
            "default_model_id": {
                "type": "string",
                "title": "Default Model ID",
                "description": "ElevenLabs model ID (e.g., v1, v2, multilingual)",
                "default": "eleven_multilingual_v2",
                "placeholder": "eleven_multilingual_v2"
            },
            "base_url": {
                "type": "string",
                "title": "Base URL",
                "description": "Optional ElevenLabs API base URL override (must include https:// or http://)",
                "default": "https://api.elevenlabs.io",
                "placeholder": "https://api.elevenlabs.io"
            }
        },
        "required": ["api_key"]
    })
}

fn metadata_properties() -> Value {
    json!({"SupportsTTS": true, "SupportsSTT": true})
}

// ---------------------------------------------------------------------------
// Config resolution: input.config[key] → input[key] → hot_config[key] → fallback
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

fn str_field<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(Value::as_str).unwrap_or("")
}

// ---------------------------------------------------------------------------
// TTS
// ---------------------------------------------------------------------------

async fn first_voice(client: &reqwest::Client, api_key: &str, base_url: &str) -> Result<String, String> {
    let url = format!("{}/v1/voices", base_url.trim_end_matches('/'));
    let resp = client.get(&url)
        .header("xi-api-key", api_key)
        .send().await
        .map_err(|e| format!("voices request failed: {}", e))?;
    if !resp.status().is_success() {
        return Err(format!("voices API {}: {}", resp.status(),
            resp.text().await.unwrap_or_default()));
    }
    let body: Value = resp.json().await
        .map_err(|e| format!("voices parse failed: {}", e))?;
    body.get("voices")
        .and_then(Value::as_array)
        .and_then(|arr| arr.first())
        .and_then(|v| v.get("voice_id"))
        .and_then(Value::as_str)
        .map(|s| s.to_string())
        .ok_or_else(|| "no voices available in this account".to_string())
}

async fn fn_tts(input: &Value) -> CallResponse {
    openlobster_sdk_base::emit_log("info", "ElevenLabs: Processing TTS request...");
    let api_key = resolve(input, "api_key", "");
    if api_key.is_empty() { return CallResponse::err("api_key required"); }

    let base_url = resolve(input, "base_url", "https://api.elevenlabs.io");

    let text = str_field(input, "text").to_string();
    if text.is_empty() { return CallResponse::err("text required"); }

    let preferred_voice = {
        let v = str_field(input, "voice_id").to_string();
        if v.is_empty() { resolve(input, "default_voice_id", "") } else { v }
    };
    let model_id      = resolve(input, "model_id",      "eleven_multilingual_v2");
    let output_format = resolve(input, "output_format", "pcm_16000");

    let client = reqwest::Client::new();
    let voice_id = if preferred_voice.is_empty() {
        match first_voice(&client, &api_key, &base_url).await {
            Ok(v)  => v,
            Err(e) => return CallResponse::err(e),
        }
    } else {
        preferred_voice
    };

    let url_base = base_url.trim_end_matches('/');
    let url  = format!("{url_base}/v1/text-to-speech/{voice_id}");
    let body = json!({"text": text, "model_id": model_id});

    match client.post(&url)
        .query(&[("output_format", &output_format)])
        .header("xi-api-key", &api_key)
        .json(&body)
        .send().await
    {
        Ok(resp) if resp.status().is_success() => {
            match resp.bytes().await {
                Ok(b)  => CallResponse::ok(json!({"audio": BASE64.encode(&b), "format": "pcm", "sample_rate": 16000})),
                Err(e) => CallResponse::err(format!("read failed: {}", e)),
            }
        }
        Ok(resp) => CallResponse::err(format!("ElevenLabs TTS {}: {}",
            resp.status(), resp.text().await.unwrap_or_default())),
        Err(e)   => CallResponse::err(format!("request failed: {}", e)),
    }
}

// ---------------------------------------------------------------------------
// STT
// ---------------------------------------------------------------------------

async fn fn_stt(input: &Value) -> CallResponse {
    openlobster_sdk_base::emit_log("info", "ElevenLabs: Processing STT request...");
    let api_key = resolve(input, "api_key", "");
    if api_key.is_empty() { return CallResponse::err("api_key required"); }

    let base_url = resolve(input, "base_url", "https://api.elevenlabs.io");

    let audio_b64 = str_field(input, "audio");
    if audio_b64.is_empty() { return CallResponse::err("audio (base64) required"); }

    let clean = audio_b64
        .trim_start_matches("data:audio/mpeg;base64,")
        .trim_start_matches("data:audio/mp3;base64,")
        .trim_start_matches("data:audio;base64,");

    let audio_bytes = match BASE64.decode(clean) {
        Ok(b)  => b,
        Err(e) => return CallResponse::err(format!("invalid base64 audio: {}", e)),
    };

    let model_id = resolve(input, "model_id", "scribe_v1");

    let part = match reqwest::multipart::Part::bytes(audio_bytes)
        .file_name("audio.mp3")
        .mime_str("audio/mpeg")
    {
        Ok(p)  => p,
        Err(e) => return CallResponse::err(format!("mime error: {}", e)),
    };
    let form = reqwest::multipart::Form::new()
        .part("file", part)
        .text("model_id", model_id);

    let client = reqwest::Client::new();
    let url = format!("{}/v1/speech-to-text", base_url.trim_end_matches('/'));
    match client.post(&url)
        .header("xi-api-key", &api_key)
        .multipart(form)
        .send().await
    {
        Ok(resp) if resp.status().is_success() => {
            match resp.json::<Value>().await {
                Ok(body) => CallResponse::ok(body),
                Err(e)   => CallResponse::err(format!("parse failed: {}", e)),
            }
        }
        Ok(resp) => CallResponse::err(format!("ElevenLabs STT {}: {}",
            resp.status(), resp.text().await.unwrap_or_default())),
        Err(e)   => CallResponse::err(format!("request failed: {}", e)),
    }
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct ElevenLabsPlugin;

#[async_trait]
impl Plugin for ElevenLabsPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "ElevenLabs",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["configure", "tts", "stt"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        let input = input.unwrap_or(Value::Null);
        match function {
            "configure" => CONFIG.configure(Some(input)),
            "tts"       => fn_tts(&input).await,
            "stt"       => fn_stt(&input).await,
            other       => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(ElevenLabsPlugin).await;
}
