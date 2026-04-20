// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! Internal STDIO write helper.
//!
//! Both the runner (responses to host requests) and the RPC module (plugin-
//! initiated requests) write a single newline-terminated JSON line to stdout.
//! This helper acquires the stdout lock, writes, and flushes atomically so
//! that lines from different code paths never interleave.

use std::io::Write;

/// Writes `line` followed by a newline to stdout and flushes immediately.
///
/// The stdout lock is held for the duration of the write so concurrent callers
/// (e.g. the runner and [`crate::rpc::call_core`]) produce complete lines.
pub(crate) fn write_line(line: &str) {
    let stdout = std::io::stdout();
    let mut handle = stdout.lock();
    let _ = writeln!(handle, "{}", line);
    let _ = handle.flush();
}
