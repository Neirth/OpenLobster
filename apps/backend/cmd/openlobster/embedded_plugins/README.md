Copyright (c) OpenLobster contributors. See LICENSE for details.

# Embedded Plugin Bundle

This directory is used at build time to embed builtin WASM plugins into the backend binary via go:embed.

The backend build script copies only the curated builtin catalog into this directory before running `go build`.
Do not commit generated `.wasm` files from this folder.
