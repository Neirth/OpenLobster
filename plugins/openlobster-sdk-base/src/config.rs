// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! Hot-configuration management shared by all OpenLobster plugins.
//!
//! Every plugin exposes a `configure` export that stores a configuration map
//! at runtime. Subsequent function calls can supply per-call overrides; the
//! SDK merges them with the stored hot config so that per-call values win.
//!
//! # Usage
//!
//! Declare a module-level static and delegate the `"configure"` arm of your
//! `call` implementation to it:
//!
//! ```rust
//! use openlobster_sdk_base::HotConfig;
//!
//! static CONFIG: HotConfig = HotConfig::new();
//! ```

use std::collections::HashMap;

use serde_json::Value;

use crate::protocol::CallResponse;

/// Thread-safe hot-configuration store.
///
/// Declare a `static` instance per plugin (one per binary). The internal
/// `Mutex` is initialised lazily so `const fn new()` is safe.
pub struct HotConfig {
    inner: std::sync::Mutex<Option<HashMap<String, Value>>>,
}

impl HotConfig {
    /// Creates a new, empty hot-config store.
    ///
    /// Safe to use as a `static` initialiser.
    pub const fn new() -> Self {
        Self { inner: std::sync::Mutex::new(None) }
    }

    /// Handles the `"configure"` exported function.
    ///
    /// Accepts a JSON object whose top-level keys are configuration entries,
    /// or a wrapper `{ "config": { ... } }` produced by the backend loader.
    /// Stores the result and returns `{ "ok": true }`.
    pub fn configure(&self, input: Option<Value>) -> CallResponse {
        let cfg: HashMap<String, Value> = match input {
            Some(v) => {
                let wrapper: HashMap<String, Value> =
                    serde_json::from_value(v).unwrap_or_default();
                match wrapper.get("config") {
                    Some(inner) => {
                        serde_json::from_value(inner.clone()).unwrap_or_default()
                    }
                    None => wrapper,
                }
            }
            None => HashMap::new(),
        };

        if let Ok(mut guard) = self.inner.lock() {
            *guard = Some(cfg);
        }

        CallResponse::ok(serde_json::json!({"ok": true}))
    }

    /// Returns the effective configuration for a single function call.
    ///
    /// `per_call` entries take precedence over the hot config. If the hot
    /// config has not been set yet the per-call map is returned unchanged.
    pub fn merge(
        &self,
        per_call: Option<HashMap<String, Value>>,
    ) -> HashMap<String, Value> {
        let mut out: HashMap<String, Value> = per_call.unwrap_or_default();
        if let Ok(guard) = self.inner.lock() {
            if let Some(hot) = guard.as_ref() {
                for (k, v) in hot {
                    out.entry(k.clone()).or_insert_with(|| v.clone());
                }
            }
        }
        out
    }

    /// Extracts a trimmed string from a config map, returning an empty string
    /// if the key is absent or the value is not a JSON string.
    pub fn get_str(cfg: &HashMap<String, Value>, key: &str) -> String {
        cfg.get(key)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .trim()
            .to_string()
    }

    /// Extracts a boolean from a config map, returning `default` if the key
    /// is absent or the value is not a JSON boolean.
    pub fn get_bool(cfg: &HashMap<String, Value>, key: &str, default: bool) -> bool {
        cfg.get(key).and_then(|v| v.as_bool()).unwrap_or(default)
    }

    /// Extracts an unsigned integer from a config map, returning `default` if
    /// the key is absent or the value is not a JSON number.
    pub fn get_u64(cfg: &HashMap<String, Value>, key: &str, default: u64) -> u64 {
        cfg.get(key).and_then(|v| v.as_u64()).unwrap_or(default)
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn make_config() -> &'static HotConfig {
        // Use a Box-leaked static so each test gets an independent instance.
        Box::leak(Box::new(HotConfig::new()))
    }

    #[test]
    fn configure_flat_map() {
        let cfg = make_config();
        let resp = cfg.configure(Some(json!({"api_key": "sk-test", "model": "gpt-4"})));
        assert!(resp.error.is_none());

        let merged = cfg.merge(None);
        assert_eq!(HotConfig::get_str(&merged, "api_key"), "sk-test");
        assert_eq!(HotConfig::get_str(&merged, "model"), "gpt-4");
    }

    #[test]
    fn configure_wrapped_map() {
        let cfg = make_config();
        cfg.configure(Some(json!({"config": {"api_key": "wrapped-key"}})));

        let merged = cfg.merge(None);
        assert_eq!(HotConfig::get_str(&merged, "api_key"), "wrapped-key");
    }

    #[test]
    fn per_call_wins_over_hot() {
        let cfg = make_config();
        cfg.configure(Some(json!({"model": "hot-model", "api_key": "hot-key"})));

        let per_call: HashMap<String, Value> =
            serde_json::from_value(json!({"model": "call-model"})).unwrap();
        let merged = cfg.merge(Some(per_call));

        assert_eq!(HotConfig::get_str(&merged, "model"), "call-model");
        assert_eq!(HotConfig::get_str(&merged, "api_key"), "hot-key");
    }

    #[test]
    fn missing_key_returns_empty_string() {
        let cfg = HotConfig::new();
        let merged = cfg.merge(None);
        assert_eq!(HotConfig::get_str(&merged, "nonexistent"), "");
    }

    #[test]
    fn get_bool_defaults() {
        let map: HashMap<String, Value> = HashMap::new();
        assert!(!HotConfig::get_bool(&map, "flag", false));
        assert!(HotConfig::get_bool(&map, "flag", true));
    }

    #[test]
    fn get_u64_defaults() {
        let map: HashMap<String, Value> = HashMap::new();
        assert_eq!(HotConfig::get_u64(&map, "limit", 42), 42);
    }
}
