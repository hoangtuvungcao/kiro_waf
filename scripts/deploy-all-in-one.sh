#!/usr/bin/env bash
# =============================================================================
# Kiro WAF — Deploy All-in-One (Master + Client on a single VPS)
# =============================================================================
# Deploys both Master Server and Client Node on the same VPS.
# Run as root on the target VPS.
#
# Usage:
#   scp -r . root@<VPS_IP>:/opt/kiro_waf/
#   ssh root@<VPS_IP> 'bash /opt/kiro_waf/scripts/deploy-all-in-one.sh'
#
# For a more complete deployment (with Nginx, TLS, systemd hardening),
# use scripts/deploy_master.sh instead.
# =============================================================================

set -euo pipefail

# --- Configuration ---
KIRO_DIR="${KIRO_DIR:-/opt/kiro_waf}"
BUILD_DIR="${KIRO_DIR}/build"
DATA_DIR="/var/lib/kiro-master"
LOG_DIR="/var/log/kiro"
CONFIG_DIR="/etc/kiro"

# Master Server config
MASTER_ADDR="${KIRO_MASTER_ADDR:-:8080}"
MASTER_DB="${DATA_DIR}/master.db"
MASTER_ADMIN_KEY="${KIRO_MASTER_ADMIN_KEY:-$(openssl rand -base64 32 2>/dev/null || echo "CHANGE_ME_$(date +%s)")}"
MASTER_ADMIN_IPS=""  # Empty = allow all IPs
MASTER_SESSION_TTL="12h"

# Client Node config
CLIENT_LISTEN="${KIRO_CLIENT_LISTEN:-:80}"
CLIENT_BACKEND="http://127.0.0.1:8080"  # Proxy to master for testing
CLIENT_MASTER_URL="http://127.0.0.1:8080"
CLIENT_LICENSE_KEY="${KIRO_CLIENT_LICENSE_KEY:-KIRO-TEST-0001-DEPLOY}"
CLIENT_COOKIE_SECRET="$(openssl rand -base64 32 2>/dev/null || echo "kiro-deploy-secret-$(date +%s)")"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# --- Step 1: Check environment ---
check_environment() {
    log_info "Checking environment..."

    if [[ $EUID -ne 0 ]]; then
        log_error "This script must run as root"
        exit 1
    fi

    if [[ ! -f "${KIRO_DIR}/cmd/kiro-master/main.go" ]]; then
        log_error "Project root invalid: ${KIRO_DIR}/cmd/kiro-master/main.go not found"
        exit 1
    fi

    if [[ ! -f "${KIRO_DIR}/cmd/kiro-client/main.go" ]]; then
        log_error "Project root invalid: ${KIRO_DIR}/cmd/kiro-client/main.go not found"
        exit 1
    fi

    # Install Go if missing
    if ! command -v go &>/dev/null; then
        log_info "Installing Go..."
        apt-get update -qq
        apt-get install -y -qq golang-go make gcc
    fi

    # Install clang (required for XDP)
    if ! command -v clang &>/dev/null; then
        log_info "Installing clang/llvm for XDP..."
        apt-get install -y -qq clang llvm libelf-dev libbpf-dev
    fi

    # Create directories
    mkdir -p "$DATA_DIR" "$LOG_DIR" "$CONFIG_DIR" "$BUILD_DIR"

    log_success "Environment OK"
}

# --- Step 2: Build binaries ---
build_binaries() {
    log_info "Building all binaries..."

    cd "$KIRO_DIR"

    # Build Go binaries using Makefile
    make clean 2>/dev/null || true
    make build

    log_success "Go binaries built"
}

# --- Step 3: Build XDP object (optional — system works without it) ---
build_xdp() {
    log_info "Building XDP object (optional)..."

    cd "$KIRO_DIR"

    local xdp_src="internal/client/xdp/xdp_filter.c"
    if [[ ! -f "${xdp_src}" ]]; then
        log_warn "XDP source not found — skipping XDP (L7 protection still active)"
        return 0
    fi

    if ! command -v clang &>/dev/null; then
        log_warn "clang not installed — skipping XDP build (L7 protection still active)"
        return 0
    fi

    # Install linux-headers if asm/types.h is missing
    if [[ ! -f /usr/include/asm/types.h ]] && [[ ! -f "/usr/include/$(uname -m)-linux-gnu/asm/types.h" ]]; then
        log_info "Installing linux-headers for XDP compilation..."
        apt-get install -y -qq linux-headers-"$(uname -r)" gcc-multilib 2>/dev/null || \
            apt-get install -y -qq linux-libc-dev 2>/dev/null || true
    fi

    mkdir -p "${BUILD_DIR}"

    # Add arch-specific include path
    local arch_include=""
    if [[ -d "/usr/include/$(uname -m)-linux-gnu" ]]; then
        arch_include="-I/usr/include/$(uname -m)-linux-gnu"
    fi

    if clang -O2 -g -target bpf -D__TARGET_ARCH_x86 -Wall \
        ${arch_include} -c "${xdp_src}" -o "${BUILD_DIR}/xdp_filter.o" 2>/dev/null; then
        log_success "XDP object compiled: ${BUILD_DIR}/xdp_filter.o"
    else
        log_warn "XDP compilation failed — skipping (L7 WAF protection still fully active)"
        log_warn "To fix: apt install linux-headers-$(uname -r) gcc-multilib"
    fi
}

# --- Step 4: Stop services and install binaries ---
install_binaries() {
    log_info "Installing binaries..."

    # Stop running services BEFORE replacing binaries (avoid "Text file busy")
    systemctl stop kiro-master.service 2>/dev/null || true
    systemctl stop kiro-client-waf.service 2>/dev/null || true
    log_info "Stopped services for binary replacement"

    # Install binaries
    install -m 0755 "${BUILD_DIR}/kiro-master" /usr/local/bin/kiro-master
    install -m 0755 "${BUILD_DIR}/kiro-client" /usr/local/bin/kiro-client-waf
    install -m 0755 "${BUILD_DIR}/kiro-cli" /usr/local/bin/kiro-cli

    # Install XDP object
    mkdir -p /usr/lib/kiro/xdp
    if [[ -f "${BUILD_DIR}/xdp_filter.o" ]]; then
        install -m 0644 "${BUILD_DIR}/xdp_filter.o" /usr/lib/kiro/xdp/xdp_filter.o
        log_success "Installed XDP object to /usr/lib/kiro/xdp/"
    fi

    log_success "Binaries installed"
}

# --- Step 5: Configure Master Server ---
configure_master() {
    log_info "Configuring Master Server..."

    cat > "${CONFIG_DIR}/kiro-master.env" << EOF
# Kiro WAF Master Server Configuration
# Generated: $(date '+%Y-%m-%d %H:%M:%S')

KIRO_MASTER_ADDR=${MASTER_ADDR}
KIRO_MASTER_DB=${MASTER_DB}
KIRO_MASTER_ADMIN_KEY=${MASTER_ADMIN_KEY}
KIRO_MASTER_ADMIN_IPS=${MASTER_ADMIN_IPS}
KIRO_MASTER_SESSION_TTL=${MASTER_SESSION_TTL}
EOF
    chmod 600 "${CONFIG_DIR}/kiro-master.env"

    # Install systemd service from deployments/
    if [[ -f "${KIRO_DIR}/deployments/systemd/kiro-master.service" ]]; then
        install -m 0644 "${KIRO_DIR}/deployments/systemd/kiro-master.service" \
            /etc/systemd/system/kiro-master.service
    else
        # Fallback: create inline service file
        cat > /etc/systemd/system/kiro-master.service << 'EOF'
[Unit]
Description=Kiro WAF Master Server
Documentation=https://firewall.vpsgen.com
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/kiro-master
EnvironmentFile=/etc/kiro/kiro-master.env
Restart=always
RestartSec=5
LimitNOFILE=65535

StandardOutput=journal
StandardError=journal
SyslogIdentifier=kiro-master

[Install]
WantedBy=multi-user.target
EOF
    fi

    log_success "Master Server configured"
}

# --- Step 6: Configure Client Node ---
configure_client() {
    log_info "Configuring Client Node..."

    cat > "${CONFIG_DIR}/kiro-client.env" << EOF
# Kiro WAF Client Node Configuration
# Generated: $(date '+%Y-%m-%d %H:%M:%S')

KIRO_LICENSE_KEY=${CLIENT_LICENSE_KEY}
KIRO_MASTER_URL=${CLIENT_MASTER_URL}
KIRO_BACKEND_URL=${CLIENT_BACKEND}
KIRO_LISTEN_ADDR=${CLIENT_LISTEN}
KIRO_CLIENT_COOKIE_SECRET=${CLIENT_COOKIE_SECRET}
KIRO_LOG_LEVEL=info
KIRO_XDP_PATH=/usr/lib/kiro/xdp/xdp_filter.o
EOF
    chmod 600 "${CONFIG_DIR}/kiro-client.env"

    # Install systemd service from deployments/
    if [[ -f "${KIRO_DIR}/deployments/systemd/kiro-client-waf.service" ]]; then
        install -m 0644 "${KIRO_DIR}/deployments/systemd/kiro-client-waf.service" \
            /etc/systemd/system/kiro-client-waf.service
    else
        # Fallback: create inline service file
        cat > /etc/systemd/system/kiro-client-waf.service << 'EOF'
[Unit]
Description=Kiro WAF Client Node
Documentation=https://firewall.vpsgen.com
After=network-online.target kiro-master.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/kiro-client-waf
EnvironmentFile=/etc/kiro/kiro-client.env
Restart=always
RestartSec=5
LimitNOFILE=65535
LimitMEMLOCK=infinity

StandardOutput=journal
StandardError=journal
SyslogIdentifier=kiro-client-waf

[Install]
WantedBy=multi-user.target
EOF
    fi

    log_success "Client Node configured"
}

# --- Step 7: Start services ---
start_services() {
    log_info "Starting services..."

    systemctl daemon-reload

    # Start Master Server first
    systemctl enable kiro-master
    systemctl start kiro-master
    sleep 3

    if systemctl is-active --quiet kiro-master; then
        log_success "Master Server running (${MASTER_ADDR})"
    else
        log_error "Master Server failed to start"
        journalctl -u kiro-master --no-pager -n 20
        exit 1
    fi

    # Start Client Node
    systemctl enable kiro-client-waf
    systemctl start kiro-client-waf
    sleep 2

    if systemctl is-active --quiet kiro-client-waf; then
        log_success "Client Node running (${CLIENT_LISTEN})"
    else
        log_warn "Client Node not started (may need valid license)"
        journalctl -u kiro-client-waf --no-pager -n 10
    fi
}

# --- Step 8: Health check ---
health_check() {
    log_info "Running health checks..."

    # Wait for master to be ready
    local retries=5
    for i in $(seq 1 $retries); do
        if curl -sf "http://127.0.0.1:8080/healthz" &>/dev/null; then
            break
        fi
        sleep 2
    done

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Master Server
    if curl -sf "http://127.0.0.1:8080/healthz" &>/dev/null; then
        echo -e "  Master Server:  ${GREEN}● Running${NC} (port 8080)"
    else
        echo -e "  Master Server:  ${RED}● Down${NC}"
    fi

    # Client Node
    if systemctl is-active --quiet kiro-client-waf; then
        echo -e "  Client Node:    ${GREEN}● Running${NC} (port 80)"
    else
        echo -e "  Client Node:    ${YELLOW}○ Inactive${NC}"
    fi

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# --- Step 9: Print summary ---
print_summary() {
    local public_ip
    public_ip="$(curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"

    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✓ KIRO WAF DEPLOY COMPLETE${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${CYAN}Access:${NC}"
    echo -e "    Homepage:      http://${public_ip}:8080/"
    echo -e "    Admin Panel:   http://${public_ip}:8080/admin/"
    echo -e "    Health Check:  http://${public_ip}:8080/healthz"
    echo ""
    echo -e "  ${CYAN}Admin Key:${NC} ${MASTER_ADMIN_KEY}"
    echo ""
    echo -e "  ${CYAN}Service management:${NC}"
    echo -e "    systemctl status kiro-master"
    echo -e "    systemctl status kiro-client-waf"
    echo -e "    journalctl -u kiro-master -f"
    echo -e "    journalctl -u kiro-client-waf -f"
    echo ""
    echo -e "  ${CYAN}CLI:${NC}"
    echo -e "    kiro-cli version"
    echo -e "    kiro-cli status"
    echo ""
    echo -e "  ${YELLOW}Next steps:${NC}"
    echo -e "    1. Set up Nginx reverse proxy + TLS (use scripts/deploy_master.sh)"
    echo -e "    2. Create a license via Admin Panel"
    echo -e "    3. Restrict admin access IPs"
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# --- Main ---
main() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Kiro WAF — Deploy All-in-One${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    check_environment
    build_binaries
    build_xdp
    install_binaries
    configure_master
    configure_client
    start_services
    health_check
    print_summary
}

main "$@"
