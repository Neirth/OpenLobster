<!-- Copyright (c) OpenLobster contributors. See LICENSE for details. -->

# OpenLobster Plugin SDK Base (Go 1.25)

This module contains the plugin runtime SDK packages used by OpenLobster plugins:

- src/sdk/runtime
- src/sdk/protocol
- src/sdk/transport/stdio
- src/sdk/transport/socket

Purpose:

- Keep plugin compilation independent from the core backend module toolchain.
- Keep a TinyGo compile path for plugin API compatibility checks (`-tags=tinygo`).
- Keep core backend on its own Go version without downgrade.

TinyGo status:

- TinyGo builds currently use a compile-only runtime shim in `src/sdk/runtime`.
- Native host execution still depends on the default non-TinyGo runtime path.

Notes:

- Module path: github.com/neirth/openlobster/plugins/openlobster-sdk-base
- Plugins import SDK packages from src paths, for example:
  github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime
- Plugins should use go.mod replace:
  replace github.com/neirth/openlobster/plugins/openlobster-sdk-base => ../openlobster-sdk-base
