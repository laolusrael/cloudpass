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
usage() { echo -e "${BLUE}$1${NC}"; }

BACKUP=true
SOURCE_PATH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --backup)
            BACKUP=true
            ;;
        --no-backup)
            BACKUP=false
            ;;
        -h|--help)
            usage "Usage: $0 [options] [source-path]"
            echo ""
            usage "Options:"
            echo "  --backup     Backup current config before update (default)"
            echo "  --no-backup  Skip backup"
            echo "  -h, --help   Show this help"
            echo ""
            usage "Arguments:"
            echo "  source-path  Path to new CloudPass files (optional)"
            echo "               If not provided, uses script directory"
            exit 0
            ;;
        *)
            SOURCE_PATH="$1"
            ;;
    esac
    shift
done

if [ "$EUID" -ne 0 ]; then
    error "This script must be run as root (use sudo)"
fi

INSTALL_DIR="/opt/cloudpass"

if [ ! -d "$INSTALL_DIR" ]; then
    error "CloudPass not installed. Run install.sh first."
fi

RUN_USER="${SUDO_USER:-root}"
if [ -f "/etc/systemd/system/cloudpass.service" ]; then
    SERVICE_USER=$(grep "^User=" /etc/systemd/system/cloudpass.service | cut -d= -f2)
    if [ -n "$SERVICE_USER" ]; then
        RUN_USER="$SERVICE_USER"
    fi
fi

info "Updating CloudPass..."

WAS_RUNNING=false
if systemctl is-active --quiet cloudpass 2>/dev/null; then
    WAS_RUNNING=true
    info "Stopping CloudPass service..."
    systemctl stop cloudpass || warn "Failed to stop service, continuing..."
fi

if [ "$BACKUP" = true ]; then
    info "Backing up current configuration..."
    [ -f "$INSTALL_DIR/config.yaml" ] && cp "$INSTALL_DIR/config.yaml" "$INSTALL_DIR/config.yaml.bak"
    [ -f "$INSTALL_DIR/cloudpass" ] && cp "$INSTALL_DIR/cloudpass" "$INSTALL_DIR/cloudpass.bak"
fi

if [ -n "$SOURCE_PATH" ]; then
    if [ -d "$SOURCE_PATH" ]; then
        SOURCE_DIR="$SOURCE_PATH"
    else
        error "Source path does not exist: $SOURCE_PATH"
    fi
else
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    BASE_DIR="$(dirname "$SCRIPT_DIR")"
    
    if [ -f "$SCRIPT_DIR/cloudpass" ]; then
        SOURCE_DIR="$SCRIPT_DIR"
    elif [ -f "$BASE_DIR/cloudpass" ]; then
        SOURCE_DIR="$BASE_DIR"
    else
        error "cloudpass binary not found. Please provide source path or run from extracted release."
    fi
fi

info "Copying files from $SOURCE_DIR..."

if [ -f "$SOURCE_DIR/cloudpass" ]; then
    cp "$SOURCE_DIR/cloudpass" "$INSTALL_DIR/cloudpass"
    chmod +x "$INSTALL_DIR/cloudpass"
    info "Binary updated"
else
    warn "No new binary found, keeping existing"
fi

if [ -f "$SOURCE_DIR/config.yaml" ]; then
    cp "$SOURCE_DIR/config.yaml" "$INSTALL_DIR/config.yaml"
    info "Config updated"
else
    warn "No new config found, keeping existing"
fi

info "Setting ownership to $RUN_USER..."
chown -R "$RUN_USER:$RUN_USER" "$INSTALL_DIR"

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

if [ -n "$SOCKET_PATH" ]; then
    if grep -q "socket_path:" "$INSTALL_DIR/config.yaml" 2>/dev/null; then
        sed -i "s|socket_path:.*|socket_path: $SOCKET_PATH|" "$INSTALL_DIR/config.yaml"
        info "Updated socket path in config"
    fi
fi

info "Reloading systemd and starting service..."
systemctl daemon-reload
systemctl start cloudpass

sleep 2

if systemctl is-active --quiet cloudpass; then
    info ""
    info "=========================================="
    info "  CloudPass updated successfully!"
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
    
    if [ -f "$INSTALL_DIR/config.yaml.bak" ]; then
        info "Backup available at: $INSTALL_DIR/config.yaml.bak"
    fi
else
    error "Service failed to start. Check logs with:"
    error "  sudo journalctl -u cloudpass -n 20"
fi