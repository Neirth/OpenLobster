#!/bin/bash
# OpenLobster One-Click Installer
# This script downloads the latest binary and installs it as a user service.
set -e

# Configuration
REPO="Neirth/OpenLobster"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="openlobster"
DEST="$INSTALL_DIR/$BINARY_NAME"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==>${NC} Starting OpenLobster installation..."

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
    if [[ "$OS_NAME" == "darwin" ]]; then
        echo -e "${RED}Error:${NC} Intel Mac binaries (x86_64) are not currently available for download."
        exit 1
    fi
elif [[ "$ARCH" == "arm64" || "$ARCH" == "aarch64" ]]; then
    ARCH_NAME="arm64"
else
    echo -e "${RED}Error:${NC} Unsupported architecture: $ARCH"
    exit 1
fi

TARGET="${OS_NAME}-${ARCH_NAME}"
echo -e "${BLUE}==>${NC} Detected platform: ${TARGET}"

# 2. Get latest version from GitHub
echo -e "${BLUE}==>${NC} Fetching latest release info..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [[ -z "$LATEST_TAG" ]]; then
    echo -e "${RED}Error:${NC} Could not fetch latest release tag."
    exit 1
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/openlobster-$TARGET"
echo -e "${BLUE}==>${NC} Downloading version $LATEST_TAG..."

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

# 5. Download binary
if command -v curl >/dev/null; then
    HTTP_CODE=$(curl -L -w "%{http_code}" -o "$DEST" "$DOWNLOAD_URL")
    if [ "$HTTP_CODE" -ne 200 ]; then
        echo -e "${RED}Error:${NC} Download failed with HTTP $HTTP_CODE."
        rm -f "$DEST"
        exit 1
    fi
elif command -v wget >/dev/null; then
    if ! wget -O "$DEST" "$DOWNLOAD_URL"; then
        echo -e "${RED}Error:${NC} Download failed."
        rm -f "$DEST"
        exit 1
    fi
else
    echo -e "${RED}Error:${NC} Neither curl nor wget found."
    exit 1
fi
chmod +x "$DEST"

echo -e "${GREEN}==>${NC} Binary installed to $DEST"

# 6. Service Installation
install_systemd() {
    echo -e "${BLUE}==>${NC} Installing systemd user service..."
    SERVICE_DIR="$HOME/.config/systemd/user"
    mkdir -p "$SERVICE_DIR"
    
    cat <<EOFF > "$SERVICE_DIR/openlobster.service"
[Unit]
Description=OpenLobster Personal AI Agent
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
</dict>
</plist>
EOFF

    # Ensure log directory exists
    mkdir -p "$HOME/.openlobster/logs"

    launchctl load -w "$PLIST_DIR/io.openlobster.plist"
    echo -e "${GREEN}==>${NC} Launchd agent loaded and started."
}

# Health check function
wait_for_service() {
    local max_attempts=30
    local attempt=1
    local url="http://localhost:8080"

    echo -e "${BLUE}==>${NC} Waiting for service to be ready..."

    while [ $attempt -le $max_attempts ]; do
        if command -v curl >/dev/null; then
            HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
            if [ "$HTTP_CODE" -eq 200 ]; then
                echo -e "${GREEN}==>${NC} Service is healthy (HTTP 200 OK)"
                return 0
            fi
        elif command -v wget >/dev/null; then
            if wget -q --spider "$url" 2>/dev/null; then
                echo -e "${GREEN}==>${NC} Service is healthy"
                return 0
            fi
        fi

        if [ $attempt -eq $max_attempts ]; then
            echo -e "${RED}Error:${NC} Service health check failed after $max_attempts attempts"
            echo -e "${RED}Error:${NC} Service may not have started correctly"
            return 1
        fi

        sleep 1
        attempt=$((attempt + 1))
    done

    return 1
}

# Check for init systems
if command -v systemctl >/dev/null && systemctl --user >/dev/null 2>&1; then
    install_systemd
    wait_for_service
elif [[ "$OS_NAME" == "darwin" ]] && command -v launchctl >/dev/null; then
    install_launchd
    wait_for_service
else
    echo -e "${RED}Warning:${NC} No supported init system found."
    echo "Run manually: $DEST serve"
fi

# 7. Final message
echo -e "\n${GREEN}Installation Complete!${NC}"
echo "-------------------------------------------------------"
echo "Binary: $DEST"
echo "URL: http://localhost:8080"
echo "-------------------------------------------------------"
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${RED}Warning:${NC} $INSTALL_DIR is not in your PATH."
    echo "Add: export PATH=\"\$PATH:\$HOME/.local/bin\""
fi
