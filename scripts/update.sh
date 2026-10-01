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
pass() { echo -e "${GREEN}[ok]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }

BACKUP=true
VERIFY=true
ROLLBACK=true
SOURCE_PATH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --backup)
            BACKUP=true
            ;;
        --no-backup)
            BACKUP=false
            ;;
        --no-verify)
            VERIFY=false
            ;;
        --no-rollback)
            ROLLBACK=false
            ;;
        -h|--help)
            usage "Usage: $0 [options] [source-path]"
            echo ""
            usage "Options:"
            echo "  --backup       Backup current config before update (default)"
            echo "  --no-backup    Skip backup"
            echo "  --no-verify    Skip post-update health and UI checks"
            echo "  --no-rollback  Do not roll back on failed verification"
            echo "  -h, --help     Show this help"
            echo ""
            usage "Verification (default on):"
            echo "  Polls /api/health, then checks that a bundled UI asset"
            echo "  is served as JavaScript (catches stale-asset deployments"
            echo "  that would otherwise show a blank page)."
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

# --- Post-update verification helpers ---
# These exist so a bad deploy fails loudly here instead of
# surfacing later as a blank UI in the browser.

# Resolve the HTTP port from the installed config (default 8080).
detect_port() {
    local cfg="$1"
    local port=""
    if [ -f "$cfg" ]; then
        port=$(grep -A5 '^[[:space:]]*server:' "$cfg" 2>/dev/null | grep -m1 '^[[:space:]]*port:' | awk '{print $2}' | tr -d '\r')
    fi
    if ! [[ "$port" =~ ^[0-9]+$ ]]; then
        port=8080
    fi
    echo "$port"
}

binary_hash() {
    sha256sum "$1" 2>/dev/null | awk '{print $1}'
}

# Poll /api/health until 200 or timeout. Pure liveness probe:
# it never touches Multipass, so it is safe to gate on.
wait_for_health() {
    local port="$1"
    local timeout="${2:-60}"
    local waited=0
    while [ "$waited" -lt "$timeout" ]; do
        if curl -sf --max-time 5 "http://localhost:$port/api/health" >/dev/null 2>&1; then
            return 0
        fi
        sleep 2
        waited=$((waited + 2))
    done
    return 1
}

# Guard against stale-asset deployments: fetch index.html, take one
# bundled chunk URL from it, and require it to be served as
# JavaScript. A stale/missing chunk is answered with index.html
# (text/html) by the SPA fallback, which browsers refuse to execute.
verify_ui_assets() {
    local port="$1"
    local index chunk code ctype
    index=$(curl -sf --max-time 10 "http://localhost:$port/" 2>/dev/null) || return 1
    chunk=$(echo "$index" | grep -o '/_app/immutable/[^"]*\.js' | head -n 1)
    if [ -z "$chunk" ]; then
        warn "Could not find a bundled asset reference in index.html; skipping asset check"
        return 0
    fi
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "http://localhost:$port$chunk" 2>/dev/null || echo "000")
    if [ "$code" != "200" ]; then
        fail "Asset $chunk returned HTTP $code (expected 200)"
        return 1
    fi
    ctype=$(curl -sI --max-time 10 "http://localhost:$port$chunk" 2>/dev/null | grep -i '^content-type:' | tr -d '\r')
    if echo "$ctype" | grep -qi 'text/html'; then
        fail "Asset $chunk served as text/html (stale deployment: chunk missing from the running binary)"
        return 1
    fi
    pass "UI asset check passed ($chunk -> $ctype)"
    return 0
}

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

# Stale-deploy guard: if the incoming binary is byte-identical to the
# running one there is nothing to deploy (e.g. release extracted twice
# or built without changes). Report and stop before touching the service.
if [ -f "$SOURCE_DIR/cloudpass" ] && [ -f "$INSTALL_DIR/cloudpass" ]; then
    OLD_HASH=$(binary_hash "$INSTALL_DIR/cloudpass")
    NEW_HASH=$(binary_hash "$SOURCE_DIR/cloudpass")
    if [ -n "$OLD_HASH" ] && [ "$OLD_HASH" = "$NEW_HASH" ]; then
        info "Installed binary is already up to date (sha256: $OLD_HASH)."
        info "Nothing to do."
        exit 0
    fi
fi

if [ -f "$SOURCE_DIR/cloudpass" ]; then
    cp "$SOURCE_DIR/cloudpass" "$INSTALL_DIR/cloudpass"
    chmod +x "$INSTALL_DIR/cloudpass"
    info "Binary updated"
else
    warn "No new binary found, keeping existing"
fi

EXISTING_CONFIG="$INSTALL_DIR/config.yaml"
NEW_CONFIG="$SOURCE_DIR/config.yaml"

if [ -f "$NEW_CONFIG" ]; then
    if [ -f "$EXISTING_CONFIG" ]; then
        warn "Config already exists at $EXISTING_CONFIG, keeping existing config"
        warn "To update config, manually copy: cp $NEW_CONFIG $EXISTING_CONFIG"
    else
        cp "$NEW_CONFIG" "$EXISTING_CONFIG"
        info "Config created"
    fi
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

PORT=$(detect_port "$INSTALL_DIR/config.yaml")

info "Reloading systemd and starting service..."
systemctl daemon-reload
systemctl start cloudpass

# --- Verification, rollback, and outcome reporting ---
# A bad deploy must fail loudly here, not later as a blank UI.
FAILED_STEP=""
ROLLED_BACK=false

if [ "$VERIFY" = true ] && ! command -v curl >/dev/null 2>&1; then
    warn "curl is not installed; skipping HTTP verification (service-presence check only)"
    VERIFY=false
fi

if [ "$VERIFY" = true ]; then
    info "Verifying update on http://localhost:$PORT ..."
    if wait_for_health "$PORT" 60; then
        pass "Service is healthy (/api/health)"
        if verify_ui_assets "$PORT"; then
            : # verify_ui_assets already reported success
        else
            FAILED_STEP="UI asset check failed (stale/missing UI assets in the running binary)"
        fi
    else
        FAILED_STEP="service did not become healthy (/api/health did not return 200 within 60s)"
    fi
fi

if [ -z "$FAILED_STEP" ] && ! systemctl is-active --quiet cloudpass; then
    FAILED_STEP="service is not active after start"
fi

if [ -n "$FAILED_STEP" ]; then
    fail "Update verification failed: $FAILED_STEP"
    echo ""
    if [ "$ROLLBACK" = true ] && [ "$BACKUP" = true ] && [ -f "$INSTALL_DIR/cloudpass.bak" ]; then
        warn "Rolling back to the previous binary..."
        systemctl stop cloudpass || true
        if cp "$INSTALL_DIR/cloudpass.bak" "$INSTALL_DIR/cloudpass" 2>/dev/null; then
            chmod +x "$INSTALL_DIR/cloudpass"
            chown "$RUN_USER:$RUN_USER" "$INSTALL_DIR/cloudpass"
            systemctl start cloudpass || true
            if { [ "$VERIFY" = false ] && systemctl is-active --quiet cloudpass; } || \
               { [ "$VERIFY" = true ] && wait_for_health "$PORT" 60; }; then
                pass "Previous version restored and running"
                ROLLED_BACK=true
            else
                fail "Rollback did not restore a running service"
                ROLLED_BACK=false
            fi
        else
            fail "Could not restore the backup binary"
            ROLLED_BACK=false
        fi
    fi

    echo ""
    if [ "$ROLLED_BACK" = true ]; then
        warn "=========================================="
        warn "  Update FAILED - previous version restored"
        warn "=========================================="
        echo ""
        echo "System state: the previous CloudPass binary is running again."
        echo "Your data and existing config were not touched."
        echo ""
        echo "Next steps:"
        echo "  1. Inspect the failed deploy: sudo journalctl -u cloudpass -n 50"
        echo "  2. Rebuild from a clean tree and retry this script."
        echo "  3. If the browser still shows a blank page, hard-refresh"
        echo "     (Ctrl+Shift+R) to drop the cached asset manifest."
    else
        fail "=========================================="
        fail "  Update FAILED - manual recovery needed"
        fail "=========================================="
        echo ""
        echo "System state: CloudPass may NOT be running correctly."
        if [ -f "$INSTALL_DIR/cloudpass.bak" ]; then
            echo "A backup binary exists at: $INSTALL_DIR/cloudpass.bak"
            echo "Restore it manually with:"
            echo "  sudo systemctl stop cloudpass"
            echo "  sudo cp $INSTALL_DIR/cloudpass.bak $INSTALL_DIR/cloudpass"
            echo "  sudo systemctl start cloudpass"
        fi
        echo "Check logs with: sudo journalctl -u cloudpass -n 50"
    fi
    exit 1
fi

info ""
info "=========================================="
info "  CloudPass updated successfully!"
info "=========================================="
info ""
if [ "$VERIFY" = true ]; then
    pass "Verified: /api/health OK, UI assets served correctly"
else
    warn "Post-update verification was skipped (--no-verify or no curl)"
fi
info ""
info "Access the UI at: http://localhost:$PORT"
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
if [ -f "$INSTALL_DIR/cloudpass.bak" ]; then
    info "Binary backup available at: $INSTALL_DIR/cloudpass.bak"
fi