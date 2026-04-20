// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! OpenLobster OpenBao secrets plugin (Rust).
//!
//! Uses the vaultrs SDK for OpenBao/Vault KV v2 operations.

use async_trait::async_trait;
use openlobster_sdk_base::{run, CallResponse, HotConfig, Plugin, PluginInfo};
use serde_json::{json, Value};
use std::collections::HashMap;
use vaultrs::client::{VaultClient, VaultClientSettingsBuilder};
use vaultrs::kv2;

// ---------------------------------------------------------------------------
// Hot config
// ---------------------------------------------------------------------------

static CONFIG: HotConfig = HotConfig::new();

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

const PLUGIN_ID: &str = "openbao";
const PLUGIN_VERSION: &str = "0.1.0";
const PLUGIN_DESC: &str = "OpenBao / HashiCorp Vault KV v2 secrets provider";
const PLUGIN_TYPE: &str = "secrets";

fn metadata_schema() -> Value {
    json!({
        "type": "object",
        "properties": {
            "url": {
                "type": "string",
                "title": "Vault URL",
                "description": "Base URL of the OpenBao or Vault server",
                "placeholder": "https://vault.example.com:8200"
            },
            "token": {
                "type": "string",
                "format": "password",
                "title": "Vault Token",
                "description": "Authentication token with read/write permissions to the specified mount",
                "placeholder": "Enter your vault token"
            },
            "mount": {
                "type": "string",
                "title": "KV Mount Path",
                "description": "The path where the KV v2 engine is mounted",
                "default": "secret",
                "placeholder": "secret"
            }
        },
        "required": ["url", "token"]
    })
}

fn metadata_properties() -> Value { json!({}) }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn str_field<'a>(input: &'a Value, key: &str) -> &'a str {
    input.get(key).and_then(Value::as_str).unwrap_or("")
}

fn per_call_config(input: &Value) -> Option<HashMap<String, Value>> {
    input.get("config").and_then(|c| serde_json::from_value(c.clone()).ok())
}

fn effective_mount(cfg: &HashMap<String, Value>) -> String {
    let m = HotConfig::get_str(cfg, "mount");
    if m.is_empty() { "secret".to_string() } else { m }
}

fn require_client(cfg: &HashMap<String, Value>) -> Result<VaultClient, String> {
    let url = HotConfig::get_str(cfg, "url");
    if url.is_empty() {
        return Err("OpenBao URL not configured (set 'url' in plugin config)".to_string());
    }
    let token = HotConfig::get_str(cfg, "token");
    let settings = VaultClientSettingsBuilder::default()
        .address(&url)
        .token(&token)
        .build()
        .map_err(|e| format!("client settings error: {}", e))?;
    VaultClient::new(settings).map_err(|e| format!("vault client error: {}", e))
}

// ---------------------------------------------------------------------------
// Secret operations
// ---------------------------------------------------------------------------

async fn fn_get(input: &Value) -> CallResponse {
    let key = str_field(input, "key");
    if key.is_empty() { return CallResponse::err("key is required"); }
    let cfg    = CONFIG.merge(per_call_config(input));
    let client = match require_client(&cfg) { Ok(c) => c, Err(e) => return CallResponse::err(e) };
    let mount  = effective_mount(&cfg);
    match kv2::read::<HashMap<String, String>>(&client, &mount, key).await {
        Ok(data) => {
            let value = data.get("value").cloned().unwrap_or_default();
            CallResponse::ok(json!({"value": value}))
        }
        Err(e) => CallResponse::err(format!("OpenBao get failed: {}", e)),
    }
}

async fn fn_set(input: &Value) -> CallResponse {
    let key   = str_field(input, "key");
    let value = str_field(input, "value");
    if key.is_empty() { return CallResponse::err("key is required"); }
    let cfg    = CONFIG.merge(per_call_config(input));
    let client = match require_client(&cfg) { Ok(c) => c, Err(e) => return CallResponse::err(e) };
    let mount  = effective_mount(&cfg);
    let mut data = HashMap::new();
    data.insert("value".to_string(), value.to_string());
    match kv2::set(&client, &mount, key, &data).await {
        Ok(_)  => CallResponse::ok(json!({"ok": true})),
        Err(e) => CallResponse::err(format!("OpenBao set failed: {}", e)),
    }
}

async fn fn_delete(input: &Value) -> CallResponse {
    let key = str_field(input, "key");
    if key.is_empty() { return CallResponse::err("key is required"); }
    let cfg    = CONFIG.merge(per_call_config(input));
    let client = match require_client(&cfg) { Ok(c) => c, Err(e) => return CallResponse::err(e) };
    let mount  = effective_mount(&cfg);
    // Use metadata delete to ensure it disappears from the list
    match kv2::delete_metadata(&client, &mount, key).await {
        Ok(())  => CallResponse::ok(json!({"ok": true})),
        Err(e)  => CallResponse::err(format!("OpenBao delete failed: {}", e)),
    }
}

async fn fn_list(input: &Value) -> CallResponse {
    let cfg    = CONFIG.merge(per_call_config(input));
    let client = match require_client(&cfg) { Ok(c) => c, Err(e) => return CallResponse::err(e) };
    let mount  = effective_mount(&cfg);

    // Recursive list helper
    async fn list_recursive(client: &VaultClient, mount: &str, path: &str) -> Vec<String> {
        let mut results = Vec::new();
        if let Ok(keys) = kv2::list(client, mount, path).await {
            for key in keys {
                let full_path = if path.is_empty() { key.clone() } else { format!("{}{}", path, key) };
                if key.ends_with('/') {
                    results.extend(Box::pin(list_recursive(client, mount, &full_path)).await);
                } else {
                    results.push(full_path);
                }
            }
        }
        results
    }

    let keys = list_recursive(&client, &mount, "").await;
    CallResponse::ok(json!({"keys": keys}))
}

// ---------------------------------------------------------------------------
// Plugin implementation
// ---------------------------------------------------------------------------

struct OpenBaoPlugin;

#[async_trait]
impl Plugin for OpenBaoPlugin {
    fn info(&self) -> PluginInfo {
        PluginInfo {
            id: PLUGIN_ID,
            name: "OpenBao",
            version: PLUGIN_VERSION,
            description: PLUGIN_DESC,
            plugin_type: PLUGIN_TYPE,
            schema: metadata_schema(),
            properties: metadata_properties(),
            exports: vec!["configure", "get", "set", "delete", "list"],
        }
    }

    async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
        let input = input.unwrap_or(Value::Null);
        match function {
            "configure" => CONFIG.configure(Some(input)),
            "get"       => fn_get(&input).await,
            "set"       => fn_set(&input).await,
            "delete"    => fn_delete(&input).await,
            "list"      => fn_list(&input).await,
            other       => CallResponse::err(format!("unknown function: {}", other)),
        }
    }
}

#[tokio::main]
async fn main() {
    run(OpenBaoPlugin).await;
}
