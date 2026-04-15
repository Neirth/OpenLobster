// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! # openlobster-sdk-base
//!
//! Shared Rust library for OpenLobster plugins.
//!
//! All plugins are always async and always bidirectional: they respond to
//! host requests and can initiate calls to the host at any time.
//!
//! ## Quick start
//!
//! ```rust,ignore
//! use async_trait::async_trait;
//! use openlobster_sdk_base::{run, call_core, emit_log, emit_message, CallResponse, HotConfig, Plugin, PluginInfo};
//! use serde_json::{json, Value};
//!
//! static CONFIG: HotConfig = HotConfig::new();
//!
//! struct MyPlugin;
//!
//! #[async_trait]
//! impl Plugin for MyPlugin {
//!     fn info(&self) -> PluginInfo {
//!         PluginInfo {
//!             id: "my-plugin", name: "my-plugin", version: "0.1.0",
//!             description: "Example plugin",
//!             plugin_type: "ai",
//!             schema:     json!({"type": "object", "properties": {}}),
//!             properties: json!({}),
//!             exports:    vec!["configure", "action"],
//!         }
//!     }
//!
//!     async fn call(&mut self, function: &str, input: Option<Value>) -> CallResponse {
//!         match function {
//!             "configure" => CONFIG.configure(input),
//!             "action"    => {
//!                 emit_log("info", "Processing action");
//!                 match call_core("vault.get", json!({"key": "token"})).await {
//!                     Ok(v)  => CallResponse::ok(v),
//!                     Err(e) => CallResponse::err(e),
//!                 }
//!             }
//!             other => CallResponse::err(format!("unknown: {}", other)),
//!         }
//!     }
//! }
//!
//! #[tokio::main]
//! async fn main() { run(MyPlugin).await; }
//! ```

pub mod config;
pub(crate) mod io;
pub mod protocol;
pub mod rpc;
pub mod runner;
pub mod validation;

// ---------------------------------------------------------------------------
// Convenience re-exports
// ---------------------------------------------------------------------------

/// Convenience re-export of [`config::HotConfig`].
pub use config::HotConfig;

/// Convenience re-export of [`protocol::CallResponse`].
pub use protocol::CallResponse;

/// Convenience re-export of [`protocol::PluginInfo`].
pub use protocol::PluginInfo;

/// Convenience re-export of [`protocol::generate_id`].
pub use protocol::generate_id;

/// Convenience re-export of [`rpc::call_core`].
pub use rpc::call_core;

/// Convenience re-export of [`rpc::emit_log`].
pub use rpc::emit_log;

/// Convenience re-export of [`rpc::emit_message`].
pub use rpc::emit_message;

/// Convenience re-export of [`runner::Plugin`].
pub use runner::Plugin;

/// Convenience re-export of [`runner::run`].
pub use runner::run;

/// Convenience re-export of [`validation::validate_schema`].
pub use validation::validate_schema;
