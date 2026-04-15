// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! Plugin-initiated JSON-RPC calls to the host.
//!
//! Plugins can call the host at any time from within their [`crate::Plugin::call`]
//! handler.  The call is async: the plugin writes a request to stdout,
//! suspends, and the runner's reader task routes the host's response back to
//! the waiting future via a oneshot channel.
//!
//! # Protocol
//!
//! ```json
//! // plugin → host (request)
//! {"jsonrpc":"2.0","id":"<uuid-v4>","method":"<method>","params":{…}}
//!
//! // host → plugin (response)
//! {"jsonrpc":"2.0","id":"<uuid-v4>","result":{…}}
//! ```
//!
//! # Usage
//!
//! ```rust,ignore
//! async fn my_handler(input: Option<serde_json::Value>) -> CallResponse {
//!     match openlobster_sdk_base::call_core(
//!         "vault.get",
//!         serde_json::json!({ "key": "api_token" }),
//!     ).await {
//!         Ok(value) => CallResponse::ok(value),
//!         Err(msg)  => CallResponse::err(msg),
//!     }
//! }
//! ```

use std::collections::HashMap;
use std::sync::{Mutex, OnceLock};

use serde::Serialize;
use serde_json::Value;

use crate::io::write_line;
use crate::protocol::generate_id;

// ---------------------------------------------------------------------------
// Pending map
// ---------------------------------------------------------------------------

type Sender  = tokio::sync::oneshot::Sender<Result<Value, String>>;
type Pending = Mutex<HashMap<String, Sender>>;

static PENDING: OnceLock<Pending> = OnceLock::new();

/// Initialises the pending map.  Called once by [`crate::runner::run`].
pub(crate) fn init_pending() {
    PENDING.get_or_init(|| Mutex::new(HashMap::new()));
}

/// Routes an incoming response to the [`call_core`] future awaiting it.
pub(crate) fn route_response(id: serde_json::Value, result: Result<Value, String>) {
    let id_str = match id {
        serde_json::Value::String(s) => s,
        serde_json::Value::Number(n) => n.to_string(),
        other => other.to_string(),
    };

    if let Some(pending) = PENDING.get() {
        if let Ok(mut map) = pending.lock() {
            if let Some(tx) = map.remove(&id_str) {
                let _ = tx.send(result);
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Makes an async JSON-RPC call to the host and awaits the response.
pub async fn call_core(method: &str, params: impl Serialize) -> Result<Value, String> {
    let id = generate_id();
    let (tx, rx) = tokio::sync::oneshot::channel();

    PENDING
        .get_or_init(|| Mutex::new(HashMap::new()))
        .lock()
        .map_err(|_| "pending map poisoned".to_string())?
        .insert(id.clone(), tx);

    let request = serde_json::json!({
        "jsonrpc": "2.0",
        "id":      id,
        "method":  method,
        "params":  serde_json::to_value(params).map_err(|e| e.to_string())?,
    });

    write_line(&serde_json::to_string(&request).map_err(|e| e.to_string())?);

    rx.await.map_err(|_| "response channel dropped".to_string())?
}

/// Sends a log message to the host without waiting for a response.
pub fn emit_log(level: &str, message: &str) {
    let request = serde_json::json!({
        "jsonrpc": "2.0",
        "method":  "emit_log",
        "params":  {
            "level":   level,
            "message": message,
        },
    });
    if let Ok(line) = serde_json::to_string(&request) {
        write_line(&line);
    }
}

/// Sends an incoming message to the host without waiting for a response.
pub fn emit_message(payload: &impl Serialize) {
    #[derive(Serialize)]
    struct EmitMessageParams<'a, P: Serialize> {
        r#type: &'a str,
        payload: &'a P,
    }
    let request = serde_json::json!({
        "jsonrpc": "2.0",
        "method":  "emit_message",
        "params":  EmitMessageParams { r#type: "emit_message", payload },
    });
    if let Ok(line) = serde_json::to_string(&request) {
        write_line(&line);
    }
}
