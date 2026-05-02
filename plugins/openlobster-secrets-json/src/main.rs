// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster encrypted-JSON secrets plugin (Rust port of openlobster-secrets-json).
//!
//! Stores secrets in an AES-256-GCM encrypted JSON file.
//! Compatible byte-for-byte with the Go implementation's wire format.

use aes_gcm::{
    aead::{Aead, KeyInit},
    Aes256Gcm, Key, Nonce,
};
use base64::{engine::general_purpose, Engine};
use async_trait::async_trait;
use openlobster_sdk_base::{run, CallResponse, Plugin, PluginInfo, HotConfig};
use rand::RngCore;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::io::Write;
use std::path::Path;
use std::sync::{LazyLock, Mutex};

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: openlobster_sdk_base::HotConfig = openlobster_sdk_base::HotConfig::new();

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "file";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "JSON-based secret storage plugin for OpenLobster";
const PLUGIN_TYPE: &str = "secrets";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "title": "Storage Path",
                "description": "Absolute path to the JSON file where secrets are stored",
                "default": "~/.openlobster/secrets.json",
                "placeholder": "/home/user/.openlobster/secrets.json"
            },
            "key": {
                "type": "string",
                "format": "password",
                "title": "Encryption Key",
                "description": "Internal encryption key to obfuscate secrets on disk",
                "placeholder": "Enter a strong passphrase"
            }
        },
        "required": ["path"],
        "additionalProperties": false
    })
}

fn metadata_properties() -> Value { json!({}) }

// ---------------------------------------------------------------------------
// Per-call helpers — hot config wins, matching Go behavior
// ---------------------------------------------------------------------------

/// Returns per-call config merged with hot config (hot config wins).
fn merged_config(input: &Value) -> HashMap<String, Value> {
    let per_call: Option<HashMap<String, Value>> = input
        .get("config")
        .and_then(|v| serde_json::from_value(v.clone()).ok());
    CONFIG.merge(per_call)
}

fn input_str<'a>(input: &'a Value, key: &str) -> &'a str {
    input.get(key).and_then(Value::as_str).unwrap_or("")
}

// ---------------------------------------------------------------------------
// Path resolution (mirrors Go resolveStoragePath)
// ---------------------------------------------------------------------------

fn resolve_storage_path(path: &str) -> String {
    let p = path.trim();
    if p.is_empty() {
        let home = home_dir();
        return format!("{}/.openlobster/secrets.json", home);
    }
    let p = if p.starts_with("~/") {
        format!("{}/{}", home_dir(), &p[2..])
    } else {
        p.to_string()
    };
    if p.ends_with('/') || p.ends_with('\\') {
        return format!("{}secrets.json", p);
    }
    if Path::new(&p).is_dir() {
        return format!("{}/secrets.json", p);
    }
    if Path::new(&p).extension().is_none() {
        return format!("{}/secrets.json", p);
    }
    p
}

fn home_dir() -> String {
    std::env::var("HOME")
        .or_else(|_| std::env::var("USERPROFILE"))
        .unwrap_or_else(|_| ".".to_string())
}

// ---------------------------------------------------------------------------
// Key derivation (mirrors Go resolveEncryptionKey)
// ---------------------------------------------------------------------------

fn resolve_key(override_key: &str) -> [u8; 32] {
    let raw = override_key.trim();
    let raw = if raw.is_empty() {
        std::env::var("OPENLOBSTER_SECRET_KEY")
            .unwrap_or_default()
            .trim()
            .to_string()
    } else {
        raw.to_string()
    };

    if raw.is_empty() {
        let mut h = Sha256::new();
        h.update(b"OpenLobster");
        return h.finalize().into();
    }

    if let Ok(bytes) = general_purpose::STANDARD.decode(&raw) {
        if bytes.len() == 32 {
            let mut key = [0u8; 32];
            key.copy_from_slice(&bytes);
            return key;
        }
    }
    if let Ok(bytes) = general_purpose::URL_SAFE.decode(&raw) {
        if bytes.len() == 32 {
            let mut key = [0u8; 32];
            key.copy_from_slice(&bytes);
            return key;
        }
    }
    if let Ok(bytes) = hex::decode(&raw) {
        if bytes.len() == 32 {
            let mut key = [0u8; 32];
            key.copy_from_slice(&bytes);
            return key;
        }
    }
    let mut h = Sha256::new();
    h.update(raw.as_bytes());
    h.finalize().into()
}

// ---------------------------------------------------------------------------
// AES-256-GCM encrypt / decrypt (wire-compatible with Go)
// ---------------------------------------------------------------------------

const NONCE_SIZE: usize = 12;

fn encrypt(key_bytes: &[u8; 32], plain: &[u8]) -> Result<Vec<u8>, String> {
    let key    = Key::<Aes256Gcm>::from_slice(key_bytes);
    let cipher = Aes256Gcm::new(key);

    let mut nonce_bytes = [0u8; NONCE_SIZE];
    rand::thread_rng().fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);

    let ciphertext = cipher.encrypt(nonce, plain)
        .map_err(|e| format!("encrypt: {}", e))?;

    let mut out = Vec::with_capacity(NONCE_SIZE + ciphertext.len());
    out.extend_from_slice(&nonce_bytes);
    out.extend_from_slice(&ciphertext);
    Ok(out)
}

fn decrypt(key_bytes: &[u8; 32], data: &[u8]) -> Result<Vec<u8>, String> {
    if data.len() < NONCE_SIZE {
        return Err("ciphertext too short".to_string());
    }
    let key    = Key::<Aes256Gcm>::from_slice(key_bytes);
    let cipher = Aes256Gcm::new(key);
    let nonce  = Nonce::from_slice(&data[..NONCE_SIZE]);

    cipher.decrypt(nonce, &data[NONCE_SIZE..])
        .map_err(|e| format!("decrypt: {}", e))
}

// ---------------------------------------------------------------------------
// Secrets file I/O
// ---------------------------------------------------------------------------

fn load_secrets(path: &str, key: &[u8; 32]) -> Result<HashMap<String, String>, String> {
    let p = Path::new(path);
    if !p.exists() { return Ok(HashMap::new()); }
    let bytes = std::fs::read(p).map_err(|e| e.to_string())?;
    if bytes.is_empty() { return Ok(HashMap::new()); }
    let plain = decrypt(key, &bytes)?;
    serde_json::from_slice::<HashMap<String, String>>(&plain)
        .map_err(|e| format!("parse secrets file: {}", e))
}

fn save_secrets(path: &str, key: &[u8; 32], data: &HashMap<String, String>) -> Result<(), String> {
    if let Some(parent) = Path::new(path).parent() {
        if !parent.as_os_str().is_empty() {
            std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
    }
    let plain      = serde_json::to_vec(data).map_err(|e| e.to_string())?;
    let ciphertext = encrypt(key, &plain)?;
    write_secret_file(path, &ciphertext)
}

#[cfg(unix)]
fn write_secret_file(path: &str, data: &[u8]) -> Result<(), String> {
    use std::os::unix::fs::OpenOptionsExt;
    let mut file = std::fs::OpenOptions::new()
        .write(true).create(true).truncate(true).mode(0o600)
        .open(path).map_err(|e| e.to_string())?;
    file.write_all(data).map_err(|e| e.to_string())
}

#[cfg(not(unix))]
fn write_secret_file(path: &str, data: &[u8]) -> Result<(), String> {
    std::fs::write(path, data).map_err(|e| e.to_string())
}

// ---------------------------------------------------------------------------
// Secret operations
// ---------------------------------------------------------------------------

static FILE_MUTEX: LazyLock<Mutex<()>> = LazyLock::new(|| Mutex::new(()));

fn fn_get(input: Value) -> CallResponse {
    let cfg = merged_config(&input);
    let key_str  = HotConfig::get_str(&cfg, "key");
    let path_raw = HotConfig::get_str(&cfg, "path");
    let secret_key = input_str(&input, "key");

    if secret_key.is_empty() { return CallResponse::err("key is required"); }

    let path    = resolve_storage_path(&path_raw);
    let enc_key = resolve_key(&key_str);

    let _guard = FILE_MUTEX.lock().unwrap();
    match load_secrets(&path, &enc_key) {
        Err(e) => CallResponse::err(e),
        Ok(data) => match data.get(secret_key) {
            Some(value) => CallResponse::ok(json!({"value": value, "found": true})),
            None        => CallResponse::ok(json!({"found": false})),
        },
    }
}

fn fn_set(input: Value) -> CallResponse {
    let cfg = merged_config(&input);
    let key_str  = HotConfig::get_str(&cfg, "key");
    let path_raw = HotConfig::get_str(&cfg, "path");
    let secret_key   = input_str(&input, "key");
    let secret_value = input_str(&input, "value");

    if secret_key.is_empty() { return CallResponse::err("key is required"); }

    let path    = resolve_storage_path(&path_raw);
    let enc_key = resolve_key(&key_str);

    let _guard = FILE_MUTEX.lock().unwrap();
    let mut data = match load_secrets(&path, &enc_key) {
        Ok(d)  => d,
        Err(e) => return CallResponse::err(e),
    };
    data.insert(secret_key.to_string(), secret_value.to_string());
    match save_secrets(&path, &enc_key, &data) {
        Ok(())  => CallResponse::ok(json!({"ok": true})),
        Err(e)  => CallResponse::err(e),
    }
}

fn fn_delete(input: Value) -> CallResponse {
    let cfg = merged_config(&input);
    let key_str  = HotConfig::get_str(&cfg, "key");
    let path_raw = HotConfig::get_str(&cfg, "path");
    let secret_key = input_str(&input, "key");

    if secret_key.is_empty() { return CallResponse::err("key is required"); }

    let path    = resolve_storage_path(&path_raw);
    let enc_key = resolve_key(&key_str);

    let _guard = FILE_MUTEX.lock().unwrap();
    let mut data = match load_secrets(&path, &enc_key) {
        Ok(d)  => d,
        Err(e) => return CallResponse::err(e),
    };
    data.remove(secret_key);
    match save_secrets(&path, &enc_key, &data) {
        Ok(())  => CallResponse::ok(json!({"ok": true})),
        Err(e)  => CallResponse::err(e),
    }
}

fn fn_list(input: Value) -> CallResponse {
    let cfg = merged_config(&input);
    let key_str  = HotConfig::get_str(&cfg, "key");
    let path_raw = HotConfig::get_str(&cfg, "path");
    let prefix   = input_str(&input, "prefix");

    let path    = resolve_storage_path(&path_raw);
    let enc_key = resolve_key(&key_str);

    let _guard = FILE_MUTEX.lock().unwrap();
    match load_secrets(&path, &enc_key) {
        Err(e) => CallResponse::err(e),
        Ok(data) => {
            let mut keys: Vec<String> = data
                .keys()
                .filter(|k| prefix.is_empty() || k.starts_with(prefix))
                .cloned()
                .collect();
            keys.sort();
            CallResponse::ok(json!({"keys": keys}))
        }
    }
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct SecretsJsonPlugin;

#[async_trait]
impl Plugin for SecretsJsonPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "JSON Secrets",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC, plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(), properties: metadata_properties(),
            exports: vec!["configure", "get", "set", "delete", "list"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        match function {
            "configure" => CONFIG.configure(input),
            "get"       => fn_get(input.unwrap_or(Value::Null)),
            "set"       => fn_set(input.unwrap_or(Value::Null)),
            "delete"    => fn_delete(input.unwrap_or(Value::Null)),
            "list"      => fn_list(input.unwrap_or(Value::Null)),
            other       => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(SecretsJsonPlugin).await;
}
