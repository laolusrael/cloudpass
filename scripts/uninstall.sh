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

INSTALL_DIR="/opt/cloudpass"

info "Uninstalling CloudPass..."

# Stop service if running
if systemctl is-active --quiet cloudpass 2>/dev/null; then
    info "Stopping CloudPass service..."
    systemctl stop cloudpass
fi

# Disable service
if systemctl is-enabled --quiet cloudpass 2>/dev/null; then
    info "Disabling CloudPass service..."
    systemctl disable cloudpass
fi

# Remove systemd service
if [ -f /etc/systemd/system/cloudpass.service ]; then
    info "Removing systemd service..."
    rm -f /etc/systemd/system/cloudpass.service
    systemctl daemon-reload
fi

# Remove environment file
if [ -f /etc/default/cloudpass ]; then
    info "Removing cloudpass environment file..."
    rm -f /etc/default/cloudpass
fi

# Remove installation directory
if [ -d "$INSTALL_DIR" ]; then
    info "Removing installation directory..."
    rm -rf "$INSTALL_DIR"
fi

# Clean up SSH key directory if it exists
if [ -d "/home/${SUDO_USER:-root}/.cloudpass" ]; then
    info "Removing SSH key directory..."
    rm -rf "/home/${SUDO_USER:-root}/.cloudpass"
fi

info ""
info "=========================================="
info "  CloudPass uninstalled successfully!"
info "=========================================="
info ""