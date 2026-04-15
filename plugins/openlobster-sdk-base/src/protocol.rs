// Copyright (c) OpenLobster contributors. See LICENSE for details.

// SPDX-License-Identifier: Apache-2.0

//! JSON-RPC 2.0 wire types and plugin protocol types.
//!
//! IDs are UUID v4 strings on both sides of the STDIO pipe. Either side
//! (host or plugin) may initiate a request; the responder echoes the UUID.

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// ID generation
// ---------------------------------------------------------------------------

/// Generates a new UUID v4 string for use as a JSON-RPC request id.
pub fn generate_id() -> String {
    uuid::Uuid::new_v4().to_string()
}

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 envelope types
// ---------------------------------------------------------------------------

/// A message arriving on stdin — either a request from the host or a response
/// to a plugin-initiated call.
///
/// Classify with [`RpcIncoming::is_response`] before dispatching.
#[derive(Deserialize, Debug)]
pub struct RpcIncoming {
    /// `"2.0"` (informational, not validated at runtime).
    #[allow(dead_code)]
    #[serde(default)]
    pub jsonrpc: Option<String>,

    /// Polymorphic ID (String, Number, or Null) present in requests and responses.
    pub id: Option<serde_json::Value>,

    /// Method name present in requests/notifications; absent in responses.
    pub method: Option<String>,

    /// Request parameters.
    #[serde(default)]
    pub params: Option<serde_json::Value>,

    /// Successful result (responses only).
    #[serde(default)]
    pub result: Option<serde_json::Value>,

    /// Error object (error responses only).
    pub error: Option<RpcError>,
}

impl RpcIncoming {
    /// Returns `true` when this is a response to a plugin-initiated request
    /// (has `id`, no `method`).
    pub fn is_response(&self) -> bool {
        self.method.is_none() && self.id.is_some()
    }
}

/// An outbound JSON-RPC 2.0 message written to stdout.
///
/// Used both for responses to host requests and for plugin-initiated requests.
#[derive(Serialize)]
pub struct RpcResponse {
    pub jsonrpc: &'static str,
    /// Echo of the request ID.
    pub id: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<RpcError>,
}

impl RpcResponse {
    /// Constructs a successful response.
    pub fn ok(id: Option<serde_json::Value>, result: serde_json::Value) -> Self {
        Self { jsonrpc: "2.0", id, result: Some(result), error: None }
    }

    /// Constructs an error response.
    pub fn err(id: Option<serde_json::Value>, code: i32, message: impl Into<String>) -> Self {
        Self {
            jsonrpc: "2.0",
            id,
            result: None,
            error: Some(RpcError { code, message: message.into() }),
        }
    }
}

/// JSON-RPC 2.0 error object.
#[derive(Serialize, Deserialize, Debug)]
pub struct RpcError {
    pub code: i32,
    pub message: String,
}

// ---------------------------------------------------------------------------
// Plugin protocol types
// ---------------------------------------------------------------------------

/// The value returned by every exported plugin function.
///
/// Use [`CallResponse::ok`] and [`CallResponse::err`] rather than constructing
/// this struct directly.
#[derive(Serialize, Deserialize, Default)]
pub struct CallResponse {
    /// Successful result, absent when `error` is set.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output: Option<serde_json::Value>,
    /// Error message, absent when `output` is set.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

impl CallResponse {
    /// Constructs a successful response with the given serialisable value.
    pub fn ok(output: impl Serialize) -> Self {
        Self {
            output: Some(
                serde_json::to_value(output).unwrap_or(serde_json::Value::Null),
            ),
            error: None,
        }
    }

    /// Constructs an error response with the given message.
    pub fn err(message: impl Into<String>) -> Self {
        Self { output: None, error: Some(message.into()) }
    }
}

/// Metadata returned by the `GetInfo` handshake.
#[derive(Serialize, Clone)]
pub struct PluginInfo {
    pub id: &'static str,
    pub name: &'static str,
    pub version: &'static str,
    pub description: &'static str,
    #[serde(rename = "type")]
    pub plugin_type: &'static str,
    pub schema: serde_json::Value,
    pub properties: serde_json::Value,
    pub exports: Vec<&'static str>,
}
