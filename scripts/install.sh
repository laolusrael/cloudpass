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
SSH_KEY_DIR="/home/$CLOUDPASS_USER/.cloudpass"
SSH_KEY_TARGET="$SSH_KEY_DIR/multipass_id_rsa"

detect_multipass_type() {
    if snap list multipass &>/dev/null 2>&1; then
        echo "snap"
    elif dpkg -l multipass &>/dev/null 2>&1; then
        echo "apt"
    else
        echo "none"
    fi
}

get_multipass_paths() {
    local type="$1"
    case "$type" in
        snap)
            SOCKET_PATH="/var/snap/multipass/common/multipass_socket"
            SSH_KEY_SOURCE="/var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa"
            DATA_DIR="/var/snap/multipass/common/data"
            SOCKET_GROUP="sudo"
            ;;
        apt)
            SOCKET_PATH="/var/run/multipass/socket"
            SSH_KEY_SOURCE="/var/lib/multipass/ssh_keys/id_rsa"
            DATA_DIR="/var/lib/multipass"
            SOCKET_GROUP=""
            ;;
        none)
            SOCKET_PATH=""
            SSH_KEY_SOURCE=""
            DATA_DIR=""
            SOCKET_GROUP=""
            ;;
    esac
}

check_multipass_daemon() {
    if ! command -v multipass &>/dev/null; then
        return 1
    fi
    
    if pgrep -x multipassd &>/dev/null; then
        return 0
    fi
    
    return 1
}

setup_socket_permissions() {
    local type="$1"
    
    if [ -z "$SOCKET_PATH" ]; then
        warn "Cannot setup socket permissions: socket path not determined"
        return 1
    fi
    
    if [ ! -S "$SOCKET_PATH" ]; then
        warn "Multipass socket not found at $SOCKET_PATH"
        warn "Is multipassd running?"
        return 1
    fi
    
    if [ "$type" = "snap" ]; then
        info "Adding cloudpass user to sudo group for socket access..."
        usermod -aG sudo "$CLOUDPASS_USER" 2>/dev/null || true
    elif [ "$type" = "apt" ]; then
        local actual_group
        actual_group=$(stat -c '%G' "$SOCKET_PATH" 2>/dev/null)
        
        if [ -n "$actual_group" ] && [ "$actual_group" != "root" ]; then
            info "Adding cloudpass user to $actual_group group for socket access..."
            usermod -aG "$actual_group" "$CLOUDPASS_USER" 2>/dev/null || true
        fi
    fi
    
    return 0
}

is_cloudpass_authenticated() {
    if sudo -u "$CLOUDPASS_USER" multipass list &>/dev/null 2>&1; then
        return 0
    fi
    return 1
}

setup_multipass_auth() {
    info "Setting up multipass authentication..."
    
    local multipass_type
    multipass_type=$(detect_multipass_type)
    
    if [ "$multipass_type" = "none" ]; then
        error "Multipass is not installed. Please install it first:"
        echo ""
        echo "  sudo snap install multipass"
        echo ""
        echo "Or for other distributions, see: https://multipass.run/install"
        exit 1
    fi
    
    info "Detected multipass installation: $multipass_type"
    
    get_multipass_paths "$multipass_type"
    
    if ! check_multipass_daemon; then
        warn "Multipass daemon (multipassd) does not appear to be running"
        warn "Please start multipass and try again"
    fi
    
    setup_socket_permissions "$multipass_type"
    
    if is_cloudpass_authenticated; then
        info "Cloudpass user is already authenticated with multipass"
        return 0
    fi
    
    info "Cloudpass user is not authenticated. Setting up authentication..."
    
    local passphrase
    passphrase=$(openssl rand -base64 24 2>/dev/null | tr -dc 'a-zA-Z0-9' | head -c 32)
    
    if [ -z "$passphrase" ]; then
        error "Could not generate passphrase"
    fi
    
    info "Setting multipass passphrase..."
    if multipass set local.passphrase="$passphrase" 2>/dev/null; then
        info "Passphrase set successfully"
        
        info "Authenticating cloudpass user..."
        if sudo -u "$CLOUDPASS_USER" multipass authenticate "$passphrase" 2>/dev/null; then
            info "Cloudpass user authenticated successfully"
        else
            warn "Could not authenticate cloudpass user automatically"
            print_authentication_guide
            return 1
        fi
    else
        warn "Could not set passphrase (root user may not be authenticated)"
        print_authentication_guide
        return 1
    fi
    
    local config_file="$INSTALL_DIR/config.yaml"
    local env_file="/etc/default/cloudpass"
    
    if [ -f "$config_file" ]; then
        if grep -q "passphrase_env:" "$config_file" 2>/dev/null; then
            sed -i 's/passphrase_env:.*/passphrase_env: "CLOUDPASS_MULTIPASS_PASS"/' "$config_file"
        else
            sed -i '/^multipass:/a\  passphrase_env: "CLOUDPASS_MULTIPASS_PASS"' "$config_file"
        fi
    fi
    
    info "Creating environment file at $env_file..."
    echo "CLOUDPASS_MULTIPASS_PASS=$passphrase" > "$env_file"
    chown root:root "$env_file"
    chmod 600 "$env_file"
    
    if is_cloudpass_authenticated; then
        info "Multipass authentication configured successfully"
        return 0
    else
        warn "Authentication verification failed"
        print_authentication_guide
        return 1
    fi
}

print_authentication_guide() {
    echo ""
    echo "================================================================================"
    warn "WARNING: Could not automatically configure multipass authentication."
    warn "CloudPass may fail to control instances."
    echo ""
    echo "To fix manually:"
    echo ""
    echo "1. As a user with multipass access, run:"
    echo "   multipass set local.passphrase=your_secure_password"
    echo ""
    echo "2. Create /etc/default/cloudpass with:"
    echo "   CLOUDPASS_MULTIPASS_PASS=your_secure_password"
    echo ""
    echo "3. Update config.yaml multipass section:"
    echo "   passphrase_env: \"CLOUDPASS_MULTIPASS_PASS\""
    echo ""
    echo "4. Restart CloudPass: sudo systemctl restart cloudpass"
    echo "================================================================================"
    echo ""
}

info "Installing CloudPass..."

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

info "Creating home directory for multipass client..."
mkdir -p "/home/$CLOUDPASS_USER"
chown "$CLOUDPASS_USER:$CLOUDPASS_GROUP" "/home/$CLOUDPASS_USER"
chmod 700 "/home/$CLOUDPASS_USER"

info "Creating installation directory..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$SSH_KEY_DIR"

info "Copying files..."
cp "$SOURCE_DIR/cloudpass" "$INSTALL_DIR/"
cp "$SOURCE_DIR/config.yaml" "$INSTALL_DIR/"

local multipass_type
multipass_type=$(detect_multipass_type)
get_multipass_paths "$multipass_type"

if [ -f "$SSH_KEY_SOURCE" ]; then
    info "Copying SSH key from $SSH_KEY_SOURCE..."
    cp "$SSH_KEY_SOURCE" "$SSH_KEY_TARGET"
    chown "$CLOUDPASS_USER:$CLOUDPASS_GROUP" "$SSH_KEY_TARGET"
    chmod 600 "$SSH_KEY_TARGET"
else
    warn "SSH key not found at $SSH_KEY_SOURCE"
    warn "Terminal access may not work without SSH key"
    warn "You can manually copy it later with:"
    warn "  sudo cp <path-to-ssh-key> $SSH_KEY_TARGET"
    warn "  sudo chown $CLOUDPASS_USER:$CLOUDPASS_GROUP $SSH_KEY_TARGET"
    warn "  sudo chmod 600 $SSH_KEY_TARGET"
fi

if [ "$PORT" != "8080" ]; then
    info "Updating port to $PORT..."
    sed -i "s/port: 8080/port: $PORT/" "$INSTALL_DIR/config.yaml"
fi

if [ -n "$SOCKET_PATH" ]; then
    info "Updating multipass socket path in config..."
    sed -i "s|socket_path:.*|socket_path: \"$SOCKET_PATH\"|" "$INSTALL_DIR/config.yaml"
fi

setup_multipass_auth || true

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
EnvironmentFile=/etc/default/cloudpass
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/cloudpass -config $INSTALL_DIR/config.yaml
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
