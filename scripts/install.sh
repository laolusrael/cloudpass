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

# Post-install verification (duplicated from update.sh so the release
# archives need no extra shared files): a broken first install must
# fail loudly here, not later as a blank UI in the browser.
PORT=$(grep -A5 '^[[:space:]]*server:' "$INSTALL_DIR/config.yaml" 2>/dev/null | grep -m1 '^[[:space:]]*port:' | awk '{print $2}' | tr -d '\r')
if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
    PORT=8080
fi

VERIFY_FAILED=""
if command -v curl >/dev/null 2>&1; then
    info "Verifying installation on http://localhost:$PORT ..."
    HEALTHY=false
    for _ in $(seq 1 30); do
        if curl -sf --max-time 5 "http://localhost:$PORT/api/health" >/dev/null 2>&1; then
            HEALTHY=true
            break
        fi
        sleep 2
    done
    if [ "$HEALTHY" = true ]; then
        echo -e "${GREEN}[ok]${NC} Service is healthy (/api/health)"
        CHUNK=$(curl -sf --max-time 10 "http://localhost:$PORT/" 2>/dev/null | grep -o '/_app/immutable/[^"]*\.js' | head -n 1)
        if [ -n "$CHUNK" ]; then
            # NOTE: headers via GET (-D -), not HEAD (-I): the server
            # registers GET-only routes and answers HEAD with 405.
            CTYPE=$(curl -s -D - -o /dev/null --max-time 10 "http://localhost:$PORT$CHUNK" 2>/dev/null | grep -i '^content-type:' | tr -d '\r')
            if [ -z "$CTYPE" ]; then
                VERIFY_FAILED="UI asset $CHUNK returned no readable content-type"
            elif echo "$CTYPE" | grep -qi 'text/html'; then
                VERIFY_FAILED="UI asset $CHUNK served as text/html (broken UI build embedded in the binary)"
            else
                echo -e "${GREEN}[ok]${NC} UI assets served correctly"
            fi
        else
            echo -e "${YELLOW}Could not find a bundled asset reference in index.html; skipping asset check${NC}"
        fi
    else
        VERIFY_FAILED="service did not become healthy (/api/health did not return 200 within 60s)"
    fi
else
    echo -e "${YELLOW}curl is not installed; skipping HTTP verification (service-presence check only)${NC}"
fi

if [ -z "$VERIFY_FAILED" ] && ! systemctl is-active --quiet cloudpass; then
    VERIFY_FAILED="service is not active after start"
fi

if [ -n "$VERIFY_FAILED" ]; then
    echo -e "${RED}[FAIL]${NC} Installation verification failed: $VERIFY_FAILED"
    echo ""
    echo "CloudPass may NOT be running correctly."
    echo "Check logs with: sudo journalctl -u cloudpass -n 50"
    exit 1
fi

info ""
info "=========================================="
info "  CloudPass installed successfully!"
info "=========================================="
info ""
info "Verified: service healthy, UI assets served correctly"
info ""
info "Access the UI at: http://localhost:$PORT"
info ""
info "Commands:"
info "  sudo systemctl start cloudpass   # Start service"
info "  sudo systemctl stop cloudpass    # Stop service"
info "  sudo systemctl status cloudpass  # Check status"
info "  sudo journalctl -u cloudpass      # View logs"
info ""