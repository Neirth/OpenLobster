#!/bin/bash
# OpenLobster Local Dev Installer
# This script installs local binaries from the dist/ directory as a user service.
set -e

# Configuration
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="openlobster"
DEST="$INSTALL_DIR/$BINARY_NAME"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==>${NC} Starting OpenLobster Local Dev installation..."

# 1. Detect Architecture and OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [[ "$OS" == "darwin" ]]; then
    OS_NAME="darwin"
elif [[ "$OS" == "linux" ]]; then
    OS_NAME="linux"
else
    echo -e "${RED}Error:${NC} Unsupported OS: $OS"
    exit 1
fi

if [[ "$ARCH" == "x86_64" ]]; then
    ARCH_NAME="amd64"
elif [[ "$ARCH" == "arm64" || "$ARCH" == "aarch64" ]]; then
    ARCH_NAME="arm64"
else
    echo -e "${RED}Error:${NC} Unsupported architecture: $ARCH"
    exit 1
fi

TARGET="${OS_NAME}-${ARCH_NAME}"
LOCAL_BINARY="./dist/openlobster-$TARGET"

echo -e "${BLUE}==>${NC} Target platform: ${TARGET}"

# 2. Check for local binary
if [[ ! -f "$LOCAL_BINARY" ]]; then
    echo -e "${RED}Error:${NC} Local binary not found at $LOCAL_BINARY"
    echo "Please build the project first using 'make build-prod' or similar."
    exit 1
fi

echo -e "${BLUE}==>${NC} Found local build: $LOCAL_BINARY"

# 3. Create install dir if it doesn't exist
mkdir -p "$INSTALL_DIR"

# 4. Stop existing service if running (to avoid 'text file busy')
if command -v systemctl >/dev/null && systemctl --user is-active openlobster >/dev/null 2>&1; then
    echo -e "${BLUE}==>${NC} Stopping existing systemd service..."
    systemctl --user stop openlobster
elif [[ "$OS_NAME" == "darwin" ]] && launchctl list io.openlobster >/dev/null 2>&1; then
    echo -e "${BLUE}==>${NC} Stopping existing launchd agent..."
    launchctl unload "$HOME/Library/LaunchAgents/io.openlobster.plist" 2>/dev/null || true
fi

# 5. Install binary
echo -e "${BLUE}==>${NC} Installing binary to $DEST..."
cp "$LOCAL_BINARY" "$DEST"
chmod +x "$DEST"

echo -e "${GREEN}==>${NC} Binary installed successfully."

# 6. Service Installation
install_systemd() {
    echo -e "${BLUE}==>${NC} Installing systemd user service..."
    SERVICE_DIR="$HOME/.config/systemd/user"
    mkdir -p "$SERVICE_DIR"
    
    cat <<EOFF > "$SERVICE_DIR/openlobster.service"
[Unit]
Description=OpenLobster Personal AI Agent (Dev)
After=network.target

[Service]
ExecStart=$DEST serve
Restart=always
Environment=PATH=/usr/bin:/usr/local/bin:$INSTALL_DIR
WorkingDirectory=$HOME

[Install]
WantedBy=default.target
EOFF

    systemctl --user daemon-reload
    systemctl --user enable openlobster
    systemctl --user restart openlobster
    echo -e "${GREEN}==>${NC} Systemd service enabled and started."
}

install_launchd() {
    echo -e "${BLUE}==>${NC} Installing launchd agent..."
    PLIST_DIR="$HOME/Library/LaunchAgents"
    mkdir -p "$PLIST_DIR"
    
    cat <<EOFF > "$PLIST_DIR/io.openlobster.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.openlobster</string>
    <key>ProgramArguments</key>
    <array>
        <string>$DEST</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.openlobster/logs/output.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.openlobster/logs/error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>OPENLOBSTER_CONFIG_ENCRYPT</key>
        <string>0</string>
    </dict>
</dict>
</plist>
EOFF

    # Ensure log directory exists
    mkdir -p "$HOME/.openlobster/logs"

    launchctl load -w "$PLIST_DIR/io.openlobster.plist"
    echo -e "${GREEN}==>${NC} Launchd agent loaded and started."
}

# Check for init systems
if command -v systemctl >/dev/null && systemctl --user >/dev/null 2>&1; then
    install_systemd
elif [[ "$OS_NAME" == "darwin" ]] && command -v launchctl >/dev/null; then
    install_launchd
else
    echo -e "${RED}Warning:${NC} No supported init system found."
    echo "Run manually: $DEST serve"
fi

# 7. Final message
echo -e "\n${GREEN}Dev Installation Complete!${NC}"
echo "-------------------------------------------------------"
echo "Binary: $DEST"
echo "Source: $LOCAL_BINARY"
echo "URL: http://localhost:8080"
echo "-------------------------------------------------------"
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${RED}Warning:${NC} $INSTALL_DIR is not in your PATH."
    echo "Add: export PATH=\"\$PATH:\$HOME/.local/bin\""
fi
