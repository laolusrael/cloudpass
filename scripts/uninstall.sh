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

CLOUDPASS_USER="cloudpass"
CLOUDPASS_GROUP="cloudpass"
INSTALL_DIR="/opt/cloudpass"

info "Uninstalling CloudPass..."

if systemctl is-active --quiet cloudpass 2>/dev/null; then
    info "Stopping CloudPass service..."
    systemctl stop cloudpass
fi

if systemctl is-enabled --quiet cloudpass 2>/dev/null; then
    info "Disabling CloudPass service..."
    systemctl disable cloudpass
fi

if [ -f /etc/systemd/system/cloudpass.service ]; then
    info "Removing systemd service..."
    rm -f /etc/systemd/system/cloudpass.service
    systemctl daemon-reload
fi

if [ -f /etc/sudoers.d/cloudpass-multipass ]; then
    info "Removing sudo permissions..."
    rm -f /etc/sudoers.d/cloudpass-multipass
fi

# Note: Multipass authentication certificates are NOT removed
# as they may be needed by other users or reinstalls

if [ -d "$INSTALL_DIR" ]; then
    info "Removing installation directory..."
    rm -rf "$INSTALL_DIR"
fi

if id "$CLOUDPASS_USER" &>/dev/null; then
    info "Removing cloudpass user..."
    userdel "$CLOUDPASS_USER" 2>/dev/null || true
fi

if getent group "$CLOUDPASS_GROUP" &>/dev/null; then
    info "Removing cloudpass group..."
    groupdel "$CLOUDPASS_GROUP" 2>/dev/null || true
fi

info ""
info "=========================================="
info "  CloudPass uninstalled successfully!"
info "=========================================="
info ""
