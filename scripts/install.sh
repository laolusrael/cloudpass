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
usage() {
    echo -e "${BLUE}Usage: $0 [--port PORT]${NC}"
    echo ""
    echo "Options:"
    echo "  --port PORT    Port to run CloudPass on (default: 8080)"
    exit 0
}

if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
fi

PORT=8080

while [[ $# -gt 0 ]]; do
    case $1 in
        --port)
            PORT="$2"
            shift 2
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

if [[ ! "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
    error "Invalid port: $PORT. Port must be between 1 and 65535"
fi

if [ "$EUID" -ne 0 ]; then
    error "This script must be run as root (use sudo)"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="/opt/cloudpass"
CLOUDPASS_USER="cloudpass"
CLOUDPASS_GROUP="cloudpass"
SSH_KEY_SOURCE="/var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa"
SSH_KEY_DIR="/home/$CLOUDPASS_USER/.cloudpass"
SSH_KEY_TARGET="$SSH_KEY_DIR/multipass_id_rsa"

info "Installing CloudPass..."

# Check for cloudpass binary in script directory or base directory
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

if id "$CLOUDPASS_USER" &>/dev/null; then
    warn "User $CLOUDPASS_USER already exists"
else
    info "Creating user $CLOUDPASS_USER..."
    useradd --system --no-create-home --shell /usr/sbin/nologin "$CLOUDPASS_USER" || true
    groupadd --system "$CLOUDPASS_GROUP" 2>/dev/null || true
    usermod -g "$CLOUDPASS_GROUP" "$CLOUDPASS_USER" 2>/dev/null || true
fi

info "Creating installation directory..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$SSH_KEY_DIR"

info "Copying files..."
cp "$SOURCE_DIR/cloudpass" "$INSTALL_DIR/"
cp "$SOURCE_DIR/config.yaml" "$INSTALL_DIR/"

if [ -f "$SSH_KEY_SOURCE" ]; then
    info "Copying SSH key..."
    cp "$SSH_KEY_SOURCE" "$SSH_KEY_TARGET"
    chown "$CLOUDPASS_USER:$CLOUDPASS_GROUP" "$SSH_KEY_TARGET"
    chmod 600 "$SSH_KEY_TARGET"
else
    warn "SSH key not found at $SSH_KEY_SOURCE"
    warn "Terminal access may not work without SSH key"
    warn "You can manually copy it later with:"
    warn "  sudo cp /var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa $SSH_KEY_TARGET"
    warn "  sudo chown $CLOUDPASS_USER:$CLOUDPASS_GROUP $SSH_KEY_TARGET"
    warn "  sudo chmod 600 $SSH_KEY_TARGET"
fi

if [ "$PORT" != "8080" ]; then
    info "Updating port to $PORT..."
    sed -i "s/port: 8080/port: $PORT/" "$INSTALL_DIR/config.yaml"
fi

setup_sudo_permissions() {
    local multipass_path
    
    # Find multipass binary
    if command -v multipass &>/dev/null; then
        multipass_path=$(which multipass)
    elif [ -x /snap/bin/multipass ]; then
        multipass_path="/snap/bin/multipass"
    elif [ -x /usr/bin/multipass ]; then
        multipass_path="/usr/bin/multipass"
    else
        warn "Could not find multipass - sudo permissions not configured"
        warn "CloudPass may fail to control instances without proper permissions"
        return
    fi
    
    info "Configuring sudo permissions for cloudpass user..."
    
    cat > /etc/sudoers.d/cloudpass-multipass << EOF
# CloudPass sudo permissions for multipass CLI
cloudpass ALL=(root) NOPASSWD: $multipass_path list
cloudpass ALL=(root) NOPASSWD: $multipass_path info *
cloudpass ALL=(root) NOPASSWD: $multipass_path start *
cloudpass ALL=(root) NOPASSWD: $multipass_path stop *
cloudpass ALL=(root) NOPASSWD: $multipass_path delete *
cloudpass ALL=(root) NOPASSWD: $multipass_path launch *
cloudpass ALL=(root) NOPASSWD: $multipass_path suspend *
cloudpass ALL=(root) NOPASSWD: $multipass_path resume *
cloudpass ALL=(root) NOPASSWD: $multipass_path images
cloudpass ALL=(root) NOPASSWD: $multipass_path networks
cloudpass ALL=(root) NOPASSWD: $multipass_path create *
EOF
    
    chmod 0440 /etc/sudoers.d/cloudpass-multipass
    info "Sudo permissions configured"
}

setup_sudo_permissions

info "Setting ownership..."
chown -R "$CLOUDPASS_USER:$CLOUDPASS_GROUP" "$INSTALL_DIR"
chown -R "$CLOUDPASS_USER:$CLOUDPASS_GROUP" "$SSH_KEY_DIR"

info "Creating systemd service..."
cat > /etc/systemd/system/cloudpass.service << EOF
[Unit]
Description=CloudPass - Multipass Management UI
After=network.target
Wants=network.target

[Service]
Type=simple
User=$CLOUDPASS_USER
Group=$CLOUDPASS_GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/cloudpass
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

info "Enabling and starting service..."
systemctl daemon-reload
systemctl enable cloudpass
systemctl start cloudpass

info ""
info "=========================================="
info "  CloudPass installed successfully!"
info "=========================================="
info ""
info "Access the UI at: http://localhost:$PORT"
info ""
info "Commands:"
info "  sudo systemctl start cloudpass   # Start service"
info "  sudo systemctl stop cloudpass    # Stop service"
info "  sudo systemctl status cloudpass  # Check status"
info "  sudo journalctl -u cloudpass      # View logs"
info ""
