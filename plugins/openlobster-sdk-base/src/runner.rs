// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! Plugin trait and STDIO main-loop entry point.
//!
//! The protocol is always async and always bidirectional: both the host and
//! the plugin may initiate JSON-RPC 2.0 requests at any time, and both sides
//! respond to the other.  All request IDs are UUID v4 strings.
//!
//! ```text
//! host  ──GetInfo(uuid)───────────────────► plugin
//! plugin ◄──result(uuid)───────────────────
//!
//! host  ──Call(uuid, fn, input)───────────► plugin
//! plugin ◄──result(uuid)───────────────────
//!
//! plugin ──Call(uuid, method, params)─────► host
//!        ◄──result(uuid)──────────────────
//!
//! host  ──Close(uuid)─────────────────────► plugin
//! plugin ◄──result(uuid, {})───────────────
//! ```
//!
//! # Usage
//!
//! ```rust,ignore
//! use async_trait::async_trait;
//! use openlobster_sdk_base::{run, CallResponse, HotConfig, Plugin, PluginInfo};
//! use serde_json::Value;
//!
//! static CONFIG: HotConfig = HotConfig::new();
//!
//! struct MyPlugin;
//!
//! #[async_trait]
//! impl Plugin for MyPlugin {
//!     fn info(&self) -> PluginInfo { /* ... */ }
//!
//!     async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
//!         match function {
//!             "configure" => CONFIG.configure(input),
//!             "fetch"     => fetch_impl(input).await,
//!             other       => CallResponse::err(format!("unknown: {}", other)),
//!         }
//!     }
//! }
//!
//! #[tokio::main]
//! async fn main() { run(MyPlugin).await; }
//! ```

use async_trait::async_trait;

use crate::io::write_line;
use crate::protocol::{CallResponse, PluginInfo, RpcIncoming, RpcResponse};
use crate::rpc;
use crate::validation::validate_schema;

// ---------------------------------------------------------------------------
// Plugin trait
// ---------------------------------------------------------------------------

/// An OpenLobster plugin.
///
/// Plugins are always async and always bidirectional: they respond to host
/// requests via [`Plugin::call`] and can initiate calls to the host via
/// [`crate::call_core`].
///
/// Implement this trait and call [`run`] from `#[tokio::main]`.
#[async_trait]
pub trait Plugin {
    /// Returns static metadata about this plugin.
    fn info(&self) -> PluginInfo;

    /// Dispatches a single function call asynchronously and returns its result.
    ///
    /// `function` is one of the names declared in [`PluginInfo::exports`].
    /// `input` is the raw JSON value from the host (may be `None`).
    async fn call(&mut self, function: &str, input: Option<serde_json::Value>)
        -> CallResponse;
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

/// Runs the bidirectional JSON-RPC 2.0 STDIO loop for a plugin.
///
/// Spawns a reader task that classifies each line arriving on stdin:
/// - **Response** (has `id`, no `method`): routed to the [`crate::call_core`]
///   future that is awaiting it.
/// - **Request** (has `method` + `id`) or **notification** (has `method`, no
///   `id`): forwarded to the async dispatch loop on the caller's task.
///
/// The dispatch loop and any concurrent [`crate::call_core`] calls are
/// scheduled by the tokio runtime without blocking threads.
///
/// Must be `.await`-ed from a `#[tokio::main]` async main.
///
/// # Panics
///
/// Panics if the plugin's JSON Schema fails validation.
pub async fn run<P: Plugin>(mut plugin: P) {
    let info = plugin.info();
    validate_schema(&info.schema)
        .unwrap_or_else(|e| panic!("plugin {}: invalid schema: {}", info.id, e));

    rpc::init_pending();

    let (req_tx, mut req_rx) = tokio::sync::mpsc::channel::<RpcIncoming>(32);

    // Reader task: reads stdin line by line and classifies each message.
    tokio::spawn(async move {
        use tokio::io::{AsyncBufReadExt, BufReader};
        let mut lines = BufReader::new(tokio::io::stdin()).lines();

        loop {
            match lines.next_line().await {
                Ok(Some(line)) => {
                    let trimmed = line.trim().to_string();
                    if trimmed.is_empty() { continue; }

                    match serde_json::from_str::<RpcIncoming>(&trimmed) {
                        Ok(incoming) if incoming.is_response() => {
                            // Response to a plugin-initiated call_core() — route
                            // it to the awaiting future via the pending map.
                            if let Some(id_val) = incoming.id {
                                let result: Result<serde_json::Value, String> =
                                    if let Some(e) = incoming.error {
                                        Err(e.message)
                                    } else {
                                        Ok(incoming.result.unwrap_or(serde_json::Value::Null))
                                    };
                                rpc::route_response(id_val, result);
                            }
                        }
                        Ok(incoming) => {
                            // Request or notification from the host.
                            if req_tx.send(incoming).await.is_err() { break; }
                        }
                        Err(e) => {
                            write_line(
                                &serde_json::to_string(&RpcResponse::err(
                                    None, -32700, format!("parse error: {}", e),
                                ))
                                .unwrap_or_default(),
                            );
                        }
                    }
                }
                Ok(None) | Err(_) => break,
            }
        }
    });

    // Dispatch loop: processes host requests on the caller's task.
    while let Some(incoming) = req_rx.recv().await {
        let id = incoming.id.clone();
        let method = incoming.method.as_deref().unwrap_or("").trim();

        match method {
            "get_info" => {
                let val = serde_json::to_value(&plugin.info())
                    .unwrap_or(serde_json::Value::Null);
                write_line(
                    &serde_json::to_string(&RpcResponse::ok(id, val))
                        .unwrap_or_default(),
                );
            }
            "close" => {
                write_line(
                    &serde_json::to_string(&RpcResponse::ok(id, serde_json::json!({})))
                        .unwrap_or_default(),
                );
                break;
            }
            other if info.exports.contains(&other) => {
                let resp = plugin.call(other, incoming.params).await;
                let val = serde_json::to_value(&resp)
                    .unwrap_or(serde_json::Value::Null);
                write_line(
                    &serde_json::to_string(&RpcResponse::ok(id, val))
                        .unwrap_or_default(),
                );
            }
            other => {
                write_line(
                    &serde_json::to_string(&RpcResponse::err(
                        id, -32601, format!("method not found: {}", other),
                    ))
                    .unwrap_or_default(),
                );
            }
        }
    }
}
