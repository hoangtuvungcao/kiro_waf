#!/bin/bash
# Kiro WAF - Deploy Script
# Build binaries locally and deploy to VPS 198 (master+client) and VPS 153 (client)
# Usage: ./deploy.sh [all|198|153]

set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────
VPS_198="root@103.77.246.198"
VPS_153="root@103.77.246.153"

BUILD_DIR="build"
REMOTE_BIN="/usr/local/bin"
REMOTE_CONFIG="/etc/kiro"
REMOTE_SERVICE_DIR="/etc/systemd/system"

# ─── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_err()   { echo -e "${RED}[ERROR]${NC} $*"; }

# ─── Build ────────────────────────────────────────────────────────────────────
build_binaries() {
    log_info "Building binaries..."
    make build
    log_ok "Build complete: ${BUILD_DIR}/kiro-master, kiro-client, kiro-cli"
}

# ─── Deploy to VPS 198 (Master + Client) ─────────────────────────────────────
deploy_198() {
    log_info "Deploying to VPS 198 (${VPS_198}) — Master + Client WAF..."

    # Upload binaries
    log_info "  Uploading binaries..."
    scp -q "${BUILD_DIR}/kiro-master" "${VPS_198}:${REMOTE_BIN}/kiro-master"
    scp -q "${BUILD_DIR}/kiro-client" "${VPS_198}:${REMOTE_BIN}/kiro-client-waf"
    scp -q "${BUILD_DIR}/kiro-cli" "${VPS_198}:${REMOTE_BIN}/kiro-cli"

    # Upload systemd service files
    log_info "  Uploading systemd services..."
    scp -q deployments/systemd/kiro-client-waf.service "${VPS_198}:${REMOTE_SERVICE_DIR}/kiro-client-waf.service"
    scp -q deployments/systemd/kiro-master.service "${VPS_198}:${REMOTE_SERVICE_DIR}/kiro-master.service"

    # Upload example config (don't overwrite existing)
    log_info "  Syncing config structure..."
    ssh "${VPS_198}" "mkdir -p ${REMOTE_CONFIG} /var/lib/kiro /var/log/kiro"

    # Check if YAML config exists, if not create from example
    ssh "${VPS_198}" bash -s << 'REMOTE_SCRIPT'
if [[ ! -f /etc/kiro/kiro.yaml ]]; then
    echo "[WARN] No /etc/kiro/kiro.yaml found. Creating from defaults..."
    echo "# Please configure this file. See docs/configuration.md" > /etc/kiro/kiro.yaml
    chmod 600 /etc/kiro/kiro.yaml
    echo "[INFO] Created /etc/kiro/kiro.yaml — please edit with your settings"
else
    echo "[OK] /etc/kiro/kiro.yaml exists, preserving"
fi
REMOTE_SCRIPT

    # Reload and restart services
    log_info "  Restarting services..."
    ssh "${VPS_198}" "systemctl daemon-reload && systemctl restart kiro-master && systemctl restart kiro-client-waf"

    # Verify
    log_info "  Verifying..."
    ssh "${VPS_198}" "systemctl is-active kiro-master && systemctl is-active kiro-client-waf"
    log_ok "VPS 198 deployed successfully"
}

# ─── Deploy to VPS 153 (Client only) ─────────────────────────────────────────
deploy_153() {
    log_info "Deploying to VPS 153 (${VPS_153}) — Client WAF..."

    # Upload binaries
    log_info "  Uploading binaries..."
    scp -q "${BUILD_DIR}/kiro-client" "${VPS_153}:${REMOTE_BIN}/kiro-client-waf"
    scp -q "${BUILD_DIR}/kiro-cli" "${VPS_153}:${REMOTE_BIN}/kiro-cli"

    # Upload systemd service file
    log_info "  Uploading systemd service..."
    scp -q deployments/systemd/kiro-client-waf.service "${VPS_153}:${REMOTE_SERVICE_DIR}/kiro-client-waf.service"

    # Ensure directories exist
    log_info "  Syncing config structure..."
    ssh "${VPS_153}" "mkdir -p ${REMOTE_CONFIG} /var/lib/kiro /var/log/kiro"

    # Check if YAML config exists
    ssh "${VPS_153}" bash -s << 'REMOTE_SCRIPT'
if [[ ! -f /etc/kiro/kiro.yaml ]]; then
    echo "[WARN] No /etc/kiro/kiro.yaml found. Creating from defaults..."
    echo "# Please configure this file. See docs/configuration.md" > /etc/kiro/kiro.yaml
    chmod 600 /etc/kiro/kiro.yaml
    echo "[INFO] Created /etc/kiro/kiro.yaml — please edit with your settings"
else
    echo "[OK] /etc/kiro/kiro.yaml exists, preserving"
fi
REMOTE_SCRIPT

    # Reload and restart
    log_info "  Restarting services..."
    ssh "${VPS_153}" "systemctl daemon-reload && systemctl restart kiro-client-waf"

    # Verify
    log_info "  Verifying..."
    ssh "${VPS_153}" "systemctl is-active kiro-client-waf"
    log_ok "VPS 153 deployed successfully"
}

# ─── Status check ────────────────────────────────────────────────────────────
check_status() {
    echo ""
    log_info "=== VPS 198 Status ==="
    ssh "${VPS_198}" "systemctl status kiro-master --no-pager -l | head -10; echo '---'; systemctl status kiro-client-waf --no-pager -l | head -10" 2>/dev/null || log_err "Cannot reach VPS 198"

    echo ""
    log_info "=== VPS 153 Status ==="
    ssh "${VPS_153}" "systemctl status kiro-client-waf --no-pager -l | head -10" 2>/dev/null || log_err "Cannot reach VPS 153"
}

# ─── Main ─────────────────────────────────────────────────────────────────────
TARGET="${1:-all}"

case "$TARGET" in
    all)
        build_binaries
        deploy_198
        deploy_153
        echo ""
        log_ok "All deployments complete!"
        check_status
        ;;
    198)
        build_binaries
        deploy_198
        ;;
    153)
        build_binaries
        deploy_153
        ;;
    status)
        check_status
        ;;
    build)
        build_binaries
        ;;
    *)
        echo "Usage: $0 [all|198|153|status|build]"
        echo "  all    - Build and deploy to both VPS (default)"
        echo "  198    - Build and deploy to VPS 198 only"
        echo "  153    - Build and deploy to VPS 153 only"
        echo "  status - Check service status on both VPS"
        echo "  build  - Build binaries only (no deploy)"
        exit 1
        ;;
esac
