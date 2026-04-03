#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

error() { echo -e "${RED}Error: $1${NC}" >&2; exit 1; }
info() { echo -e "${GREEN}$1${NC}"; }
warn() { echo -e "${YELLOW}$1${NC}"; }

if [ "$EUID" -ne 0 ]; then
    error "This script must be run as root (use sudo)"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="/opt/cloudpass"

# Use the user who ran sudo - this user already has multipass access
RUN_USER="${SUDO_USER:-root}"
SSH_KEY_DIR="/home/$RUN_USER/.cloudpass"
SSH_KEY_TARGET="$SSH_KEY_DIR/multipass_id_rsa"

info "Installing CloudPass..."

# Check for cloudpass binary
if [ -f "$SCRIPT_DIR/cloudpass" ]; then
    SOURCE_DIR="$SCRIPT_DIR"
elif [ -f "$BASE_DIR/cloudpass" ]; then
    SOURCE_DIR="$BASE_DIR"
else
    error "cloudpass binary not found. Please extract the release first."
fi

if [ ! -f "$SOURCE_DIR/config.yaml" ]; then
    error "config.yaml not found. Please extract the release first."
fi

# Create installation directory
info "Creating installation directory..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$SSH_KEY_DIR"

# Copy files
info "Copying files..."
cp "$SOURCE_DIR/cloudpass" "$INSTALL_DIR/"
cp "$SOURCE_DIR/config.yaml" "$INSTALL_DIR/"

# Copy SSH key if it exists
if [ -f "$SSH_KEY_SOURCE" ]; then
    info "Copying SSH key..."
    cp "$SSH_KEY_SOURCE" "$SSH_KEY_TARGET"
    chown "$RUN_USER:$RUN_USER" "$SSH_KEY_TARGET"
    chmod 600 "$SSH_KEY_TARGET"
fi

# Update socket path in config
info "Detecting multipass socket path..."
if snap list multipass &>/dev/null 2>&1; then
    SOCKET_PATH="/var/snap/multipass/common/multipass_socket"
    SSH_KEY_SOURCE="/var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa"
    info "Detected snap installation"
elif dpkg -l multipass &>/dev/null 2>&1; then
    SOCKET_PATH="/var/run/multipass/socket"
    SSH_KEY_SOURCE="/var/lib/multipass/ssh_keys/id_rsa"
    info "Detected apt installation"
else
    warn "Multipass not detected"
    SOCKET_PATH=""
fi

# Update socket path in config if needed
if [ -n "$SOCKET_PATH" ]; then
    if grep -q "socket_path:.*\\\\" "$INSTALL_DIR/config.yaml" 2>/dev/null; then
        sed -i "s|socket_path:.*\\\\.*|socket_path: $SOCKET_PATH|" "$INSTALL_DIR/config.yaml"
    fi
fi

# Set ownership to the user who will run the service
info "Setting ownership to $RUN_USER..."
chown -R "$RUN_USER:$RUN_USER" "$INSTALL_DIR"
chown -R "$RUN_USER:$RUN_USER" "$SSH_KEY_DIR"

# Create systemd service
info "Creating systemd service..."
cat > /etc/systemd/system/cloudpass.service << EOF
[Unit]
Description=CloudPass - Multipass Management UI
After=network.target
Wants=network.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/cloudpass -config $INSTALL_DIR/config.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
info "Enabling and starting service..."
systemctl daemon-reload
systemctl enable cloudpass
systemctl start cloudpass

info ""
info "=========================================="
info "  CloudPass installed successfully!"
info "=========================================="
info ""
info "Access the UI at: http://localhost:8080"
info ""
info "Commands:"
info "  sudo systemctl start cloudpass   # Start service"
info "  sudo systemctl stop cloudpass    # Stop service"
info "  sudo systemctl status cloudpass  # Check status"
info "  sudo journalctl -u cloudpass      # View logs"
info ""