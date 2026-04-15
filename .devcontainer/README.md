# Copyright (c) OpenLobster contributors
# SPDX-License-Identifier: MIT

# Documentation: https://containers.dev/implementors/json_reference/

# This file describes the dev container for OpenLobster.
# - Node.js 20.x, Go 1.22, pnpm, and GitHub CLI are installed.
# - VS Code extensions for Go, Node, Docker, ESLint, Prettier, and GitHub are pre-installed.
# - Ports 5173 (frontend), 8080 (backend), and 9229 (Node debug) are forwarded.
# - On creation, dependencies are installed and both frontend and backend are built.
#
# To use:
# 1. Open this folder in VS Code with the Dev Containers extension.
# 2. Reopen in container when prompted.
# 3. Use pnpm scripts for development.
