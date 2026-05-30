#!/usr/bin/env bash
# deploy_master.sh — Automated deployment script for Kiro WAF on Ubuntu 22.04
# Installs Master_Server, Client_WAF, XDP_Filter, Nginx reverse proxy, and systemd services.
# Idempotent: safe to run multiple times.
#
# Requirements: 10.1, 10.2, 10.3, 10.4, 10.5
set -euo pipefail

###############################################################################
# Configuration (overridable via environment variables)
###############################################################################
ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
DOMAIN="${KIRO_MASTER_DOMAIN:-firewall.vpsgen.com}"
GO_VERSION="${KIRO_GO_VERSION:-$(awk '$1 == "go" { print $2 }' "${ROOT}/go.mod")}"
MASTER_USER="${KIRO_MASTER_USER:-kiro-master}"
CLIENT_USER="${KIRO_CLIENT_USER:-kiro-client}"
INSTALL_CLIENT="${KIRO_MASTER_INSTALL_CLIENT:-true}"
INSTALL_CERTBOT="${KIRO_INSTALL_CERTBOT:-true}"

# Paths
ENV_DIR="/etc/kiro-master"
ENV_FILE="${ENV_DIR}/master.env"
CLIENT_ENV_DIR="/etc/kiro-client"
CLIENT_ENV_FILE="${CLIENT_ENV_DIR}/client.env"
DB_DIR="/var/lib/kiro-master"
BUILD_DIR="${ROOT}/build/master"
ADMIN_ALLOW_FILE="/etc/nginx/kiro-admin-allow.conf"
XDP_OBJ_DIR="/usr/lib/kiro/xdp"

###############################################################################
# Pre-flight checks
###############################################################################
if [ "${EUID}" -ne 0 ]; then
  echo "ERROR: deploy_master.sh must run as root on the target Ubuntu host" >&2
  exit 1
fi

if [ ! -f "${ROOT}/cmd/kiro-master/main.go" ]; then
  echo "ERROR: project root is invalid (cmd/kiro-master/main.go not found): ${ROOT}" >&2
  exit 1
fi

if [ ! -f "${ROOT}/cmd/kiro-client/main.go" ]; then
  echo "ERROR: project root is invalid (cmd/kiro-client/main.go not found): ${ROOT}" >&2
  exit 1
fi

if [ ! -f "${ROOT}/internal/client/xdp/xdp_filter.c" ]; then
  echo "ERROR: XDP source not found: ${ROOT}/internal/client/xdp/xdp_filter.c" >&2
  exit 1
fi

###############################################################################
# Helper functions
###############################################################################
log() {
  printf '\n\033[1;36m== %s ==\033[0m\n' "$*"
}

log_ok() {
  printf '  \033[1;32m✓\033[0m %s\n' "$*"
}

log_warn() {
  printf '  \033[1;33m⚠\033[0m %s\n' "$*"
}

log_err() {
  printf '  \033[1;31m✗\033[0m %s\n' "$*" >&2
}

read_existing_env() {
  local key="$1"
  local file="${2:-${ENV_FILE}}"
  if [ ! -f "${file}" ]; then
    return 0
  fi
  sed -n "s/^${key}=//p" "${file}" | tail -n 1
}

random_secret() {
  openssl rand -base64 48 | tr -d '\n'
}

detect_public_ip() {
  curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null || \
    curl -4fsS --max-time 5 https://ifconfig.me 2>/dev/null || \
    hostname -I | awk '{print $1}'
}

admin_allow_cidrs() {
  if [ -n "${KIRO_ADMIN_ALLOW_CIDRS:-}" ]; then
    printf '%s\n' "${KIRO_ADMIN_ALLOW_CIDRS}" | tr ',' '\n'
    return
  fi
  if [ -n "${SSH_CONNECTION:-}" ]; then
    set -- ${SSH_CONNECTION}
    if [ -n "${1:-}" ]; then
      printf '%s/32\n' "$1"
    fi
  fi
}

###############################################################################
# Step 1: Install system dependencies
###############################################################################
log "Installing system dependencies"

apt-get update -qq

DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  clang \
  curl \
  libelf-dev \
  libbpf-dev \
  llvm \
  nginx \
  openssl \
  python3 \
  sqlite3 \
  tar \
  wget

log_ok "System packages installed"

# Install certbot if requested
if [ "${INSTALL_CERTBOT}" = "true" ]; then
  if ! command -v certbot >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      certbot python3-certbot-nginx
    log_ok "Certbot installed"
  else
    log_ok "Certbot already installed"
  fi
fi

###############################################################################
# Step 2: Install Go (1.21+ required)
###############################################################################
install_go() {
  if command -v go >/dev/null 2>&1; then
    local current
    current="$(go env GOVERSION | sed 's/^go//')"
    if [ "$(printf '%s\n%s\n' "${GO_VERSION}" "${current}" | sort -V | head -n 1)" = "${GO_VERSION}" ]; then
      log_ok "Go ${current} already installed (>= ${GO_VERSION})"
      return
    fi
  fi
  local archive="/tmp/go${GO_VERSION}.linux-amd64.tar.gz"
  log "Installing Go ${GO_VERSION}"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "${archive}"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "${archive}"
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  rm -f "${archive}"
  log_ok "Go ${GO_VERSION} installed"
}

install_go
GO_BIN="$(command -v go)"

###############################################################################
# Step 3: Stop legacy services if present
###############################################################################
log "Stopping legacy services if present"
systemctl stop kiro-provider.service 2>/dev/null || true
systemctl disable kiro-provider.service 2>/dev/null || true
log_ok "Legacy services handled"

###############################################################################
# Step 4: Create runtime users and data directories
###############################################################################
log "Creating users and directories"

# Master user
if ! id "${MASTER_USER}" >/dev/null 2>&1; then
  useradd --system --home-dir "${DB_DIR}" --shell /usr/sbin/nologin "${MASTER_USER}"
  log_ok "Created user: ${MASTER_USER}"
else
  log_ok "User ${MASTER_USER} already exists"
fi

# Client user
if [ "${INSTALL_CLIENT}" = "true" ] && ! id "${CLIENT_USER}" >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/kiro --shell /usr/sbin/nologin "${CLIENT_USER}"
  log_ok "Created user: ${CLIENT_USER}"
fi

# Data directories: /var/lib/kiro-master/
install -d -m 0750 -o "${MASTER_USER}" -g "${MASTER_USER}" "${DB_DIR}"
log_ok "Created ${DB_DIR}"

# Config directory: /etc/kiro-master/
install -d -m 0750 -o root -g "${MASTER_USER}" "${ENV_DIR}"
log_ok "Created ${ENV_DIR}"

# Build directory
install -d -m 0755 "${BUILD_DIR}"

# XDP object directory: /usr/lib/kiro/xdp/
install -d -m 0755 "${XDP_OBJ_DIR}"
log_ok "Created ${XDP_OBJ_DIR}"

# Client directories: /var/lib/kiro/
if [ "${INSTALL_CLIENT}" = "true" ]; then
  install -d -m 0750 -o root -g "${CLIENT_USER}" "${CLIENT_ENV_DIR}"
  install -d -m 0750 -o "${CLIENT_USER}" -g "${CLIENT_USER}" /var/lib/kiro
  install -d -m 0750 -o "${CLIENT_USER}" -g "${CLIENT_USER}" /var/log/kiro
  install -d -m 0755 /etc/kiro
  log_ok "Created client directories (/var/lib/kiro/, /var/log/kiro/, /etc/kiro/)"
fi

###############################################################################
# Step 5: Generate/preserve secrets and environment
###############################################################################
log "Configuring environment"

admin_key="${KIRO_MASTER_ADMIN_KEY:-$(read_existing_env KIRO_MASTER_ADMIN_KEY)}"
signing_secret="${KIRO_MASTER_SIGNING_SECRET:-$(read_existing_env KIRO_MASTER_SIGNING_SECRET)}"
if [ -z "${admin_key}" ]; then
  admin_key="$(random_secret)"
  log_ok "Generated new admin key"
else
  log_ok "Preserved existing admin key"
fi
if [ -z "${signing_secret}" ]; then
  signing_secret="$(random_secret)"
  log_ok "Generated new signing secret"
else
  log_ok "Preserved existing signing secret"
fi

umask 077
cat >"${ENV_FILE}" <<EOF
KIRO_MASTER_ADDR=127.0.0.1:8080
KIRO_MASTER_DB=${DB_DIR}/master.db
KIRO_MASTER_PUBLIC_BASE_URL=https://${DOMAIN}
KIRO_MASTER_ADMIN_KEY=${admin_key}
KIRO_MASTER_SIGNING_SECRET=${signing_secret}
EOF
chown root:"${MASTER_USER}" "${ENV_FILE}"
chmod 0640 "${ENV_FILE}"
log_ok "Master environment written to ${ENV_FILE}"

###############################################################################
# Step 6: Build Master_Server binary → /usr/local/bin/kiro-master
###############################################################################
log "Building Master_Server binary"
cd "${ROOT}"

# Stop services BEFORE replacing binaries to avoid "Text file busy" error
systemctl stop kiro-master.service 2>/dev/null || true
systemctl stop kiro-client-waf.service 2>/dev/null || true
log_ok "Stopped running services for binary replacement"

"${GO_BIN}" build -trimpath -ldflags "-s -w" -o "${BUILD_DIR}/kiro-master" ./cmd/kiro-master/
install -m 0755 "${BUILD_DIR}/kiro-master" /usr/local/bin/kiro-master
log_ok "Installed /usr/local/bin/kiro-master"

###############################################################################
# Step 7: Build Client_WAF binary → /usr/local/bin/kiro-client-waf
###############################################################################
if [ "${INSTALL_CLIENT}" = "true" ]; then
  log "Building Client_WAF binary"
  "${GO_BIN}" build -trimpath -ldflags "-s -w" -o "${BUILD_DIR}/kiro-client-waf" ./cmd/kiro-client/
  install -m 0755 "${BUILD_DIR}/kiro-client-waf" /usr/local/bin/kiro-client-waf
  log_ok "Installed /usr/local/bin/kiro-client-waf"
fi

###############################################################################
# Step 8: Build XDP object (optional — L7 WAF works without it)
###############################################################################
log "Building XDP object file"
XDP_SRC="${ROOT}/internal/client/xdp/xdp_filter.c"
if [ ! -f "${XDP_SRC}" ]; then
  log_warn "XDP source not found — skipping"
else
  # Install linux-headers if asm/types.h is missing
  if [ ! -f /usr/include/asm/types.h ] && [ ! -f "/usr/include/$(uname -m)-linux-gnu/asm/types.h" ]; then
    apt-get install -y -qq linux-libc-dev 2>/dev/null || true
  fi

  ARCH_INCLUDE=""
  if [ -d "/usr/include/$(uname -m)-linux-gnu" ]; then
    ARCH_INCLUDE="-I/usr/include/$(uname -m)-linux-gnu"
  fi

  if clang -O2 -g -target bpf -D__TARGET_ARCH_x86 -Wall \
      ${ARCH_INCLUDE} -c "${XDP_SRC}" -o "${BUILD_DIR}/xdp_filter.o" 2>&1; then
    install -m 0644 "${BUILD_DIR}/xdp_filter.o" "${XDP_OBJ_DIR}/xdp_filter.o"
    log_ok "Installed ${XDP_OBJ_DIR}/xdp_filter.o"
  else
    log_warn "XDP build failed (missing headers?) — L7 WAF still works without XDP"
    log_warn "Fix: apt install linux-headers-$(uname -r) gcc-multilib linux-libc-dev"
  fi
fi

###############################################################################
# Step 9: Install systemd service files
###############################################################################
log "Installing systemd service files"

# kiro-master.service
if [ ! -f "${ROOT}/deployments/systemd/kiro-master.service" ]; then
  log_err "Missing systemd service file: ${ROOT}/deployments/systemd/kiro-master.service"
  exit 1
fi
install -m 0644 "${ROOT}/deployments/systemd/kiro-master.service" /etc/systemd/system/kiro-master.service
log_ok "Installed kiro-master.service"

# kiro-client-waf.service
if [ "${INSTALL_CLIENT}" = "true" ]; then
  if [ ! -f "${ROOT}/deployments/systemd/kiro-client-waf.service" ]; then
    log_err "Missing systemd service file: ${ROOT}/deployments/systemd/kiro-client-waf.service"
    exit 1
  fi
  install -m 0644 "${ROOT}/deployments/systemd/kiro-client-waf.service" /etc/systemd/system/kiro-client-waf.service
  log_ok "Installed kiro-client-waf.service"
fi

systemctl daemon-reload
log_ok "systemd daemon reloaded"

###############################################################################
# Step 10: Enable and start Master_Server
###############################################################################
log "Enabling and starting kiro-master.service"
systemctl enable kiro-master.service
systemctl restart kiro-master.service
log_ok "kiro-master.service active"

###############################################################################
# Step 11: Configure Client_WAF environment and start (if enabled)
###############################################################################
nginx_upstream="127.0.0.1:8080"

if [ "${INSTALL_CLIENT}" = "true" ]; then
  log "Configuring Client_WAF"

  # Issue local all-in-one client license
  public_ip="${KIRO_CLIENT_PUBLIC_IP:-$(detect_public_ip)}"

  # Get fingerprint (may fail if binary doesn't support it yet)
  fingerprint=""
  if /usr/local/bin/kiro-client-waf --print-fingerprint >/dev/null 2>&1; then
    fingerprint="$(/usr/local/bin/kiro-client-waf --print-fingerprint)"
  fi

  existing_license=""
  if [ -f "${CLIENT_ENV_FILE}" ]; then
    existing_license="$(sed -n 's/^KIRO_LICENSE_KEY=//p' "${CLIENT_ENV_FILE}" | tail -n 1)"
  fi

  if [ -z "${existing_license}" ]; then
    # Wait for master to be ready
    for i in 1 2 3 4 5 6 7 8 9 10; do
      if curl -fsS "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; then
        break
      fi
      sleep 2
    done

    license_id="lic_master_all_in_one_$(date +%Y%m%d%H%M%S)"
    license_response="$(python3 -c "
import json, sys
print(json.dumps({
    'license_id': sys.argv[3],
    'customer_id': 'master_server',
    'customer_name': 'Kiro Master All In One',
    'client_ip': sys.argv[1],
    'fingerprint_hash': sys.argv[2],
    'plan': 'enterprise',
    'valid_days': 3650,
}))" "${public_ip}" "${fingerprint}" "${license_id}" | \
      curl -fsS -X POST http://127.0.0.1:8080/api/v1/admin/licenses \
        -H "X-Admin-Key: ${admin_key}" \
        -H 'Content-Type: application/json' \
        -d @- 2>/dev/null)" || true

    if [ -n "${license_response}" ]; then
      existing_license="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("license_key",""))' <<<"${license_response}" 2>/dev/null)" || true
    fi
    if [ -n "${existing_license}" ]; then
      log_ok "License issued: ${license_id}"
    else
      log_warn "Could not issue license (master API may not be ready). Set KIRO_LICENSE_KEY manually."
      existing_license="PLACEHOLDER_SET_MANUALLY"
    fi
  else
    log_ok "Preserved existing client license"
  fi

  client_secret="${KIRO_CLIENT_COOKIE_SECRET:-$(read_existing_env KIRO_CLIENT_COOKIE_SECRET "${CLIENT_ENV_FILE}")}"
  if [ -z "${client_secret}" ]; then
    client_secret="$(random_secret)"
  fi

  umask 077
  cat >"${CLIENT_ENV_FILE}" <<EOF
KIRO_CLIENT_LISTEN=127.0.0.1:8090
KIRO_BACKEND_URL=http://127.0.0.1:8080
KIRO_MASTER_URL=http://127.0.0.1:8080
KIRO_LICENSE_KEY=${existing_license}
KIRO_PUBLIC_IP=${public_ip}
KIRO_NODE_ID=master-all-in-one
KIRO_CLIENT_COOKIE_SECRET=${client_secret}
KIRO_CLIENT_VERSION=1.0.0
KIRO_UPDATE_COMPONENT=kiro-client-waf
KIRO_UPDATE_CHANNEL=stable
KIRO_UPDATE_SECONDS=300
KIRO_XDP_BLOCKLIST_FILE=/var/lib/kiro/xdp-blocklist.txt
KIRO_AGENT_BINARY=/usr/local/bin/kiro-agent
KIRO_CONFIG=/etc/kiro/kiro.yaml
KIRO_RPM_PER_IP=120
KIRO_SUBNET_RPM=1800
KIRO_HARD_BLOCK_AFTER=360
KIRO_BLOCK_TTL_SECONDS=900
KIRO_POW_DIFFICULTY=4
KIRO_HOLD_SECONDS=2
KIRO_HEARTBEAT_SECONDS=30
KIRO_XDP_SYNC=true
KIRO_CLIENT_LOCKDOWN_XDP=false
EOF
  chown root:"${CLIENT_USER}" "${CLIENT_ENV_FILE}"
  chmod 0640 "${CLIENT_ENV_FILE}"
  log_ok "Client environment written to ${CLIENT_ENV_FILE}"

  systemctl enable kiro-client-waf.service
  systemctl restart kiro-client-waf.service
  nginx_upstream="127.0.0.1:8090"
  log_ok "kiro-client-waf.service active"
fi

###############################################################################
# Step 12: Configure Nginx reverse proxy
###############################################################################
log "Configuring Nginx reverse proxy"

# Generate admin allow list
{
  printf 'allow 127.0.0.1;\n'
  printf 'allow ::1;\n'
  admin_allow_cidrs | while IFS= read -r cidr; do
    cidr="$(printf '%s' "${cidr}" | xargs)"
    if [ -n "${cidr}" ]; then
      printf 'allow %s;\n' "${cidr}"
    fi
  done
  printf 'deny all;\n'
} >"${ADMIN_ALLOW_FILE}"
log_ok "Admin allow list written to ${ADMIN_ALLOW_FILE}"

# Install nginx site config with domain and upstream substitution
NGINX_SRC="${ROOT}/deployments/nginx/kiro-waf.conf"
if [ ! -f "${NGINX_SRC}" ]; then
  log_err "Missing nginx config: ${NGINX_SRC}"
  exit 1
fi
sed \
  -e "s/server_name firewall.vpsgen.com;/server_name ${DOMAIN};/" \
  -e "s#proxy_pass http://127.0.0.1:8080/admin/;#proxy_pass http://${nginx_upstream}/admin/;#" \
  -e "s#proxy_pass http://127.0.0.1:8080/;#proxy_pass http://${nginx_upstream}/;#" \
  "${NGINX_SRC}" >/etc/nginx/sites-available/firewall.conf
log_ok "Nginx site config installed"

# Enable site, remove defaults
ln -sf /etc/nginx/sites-available/firewall.conf /etc/nginx/sites-enabled/firewall.conf
rm -f /etc/nginx/sites-enabled/default
rm -f /etc/nginx/sites-enabled/kiro-management.conf

# Validate nginx config
if nginx -t 2>/dev/null; then
  log_ok "Nginx config test passed"
else
  log_err "Nginx config test FAILED"
  nginx -t
  exit 1
fi

# Enable and reload nginx
systemctl enable --now nginx.service
systemctl reload nginx.service
log_ok "Nginx enabled and reloaded"

###############################################################################
# Step 13: Health checks — verify all services active and healthz respond
###############################################################################
log "Running health checks"

health_ok=true

# Check systemd services are active
for svc in kiro-master nginx; do
  if systemctl is-active --quiet "${svc}.service"; then
    log_ok "${svc}.service is active"
  else
    log_err "${svc}.service is NOT active"
    health_ok=false
  fi
done

if [ "${INSTALL_CLIENT}" = "true" ]; then
  if systemctl is-active --quiet kiro-client-waf.service; then
    log_ok "kiro-client-waf.service is active"
  else
    log_err "kiro-client-waf.service is NOT active"
    health_ok=false
  fi
fi

# Wait a moment for services to be ready
sleep 2

# Check healthz endpoints
if curl -fsS --max-time 5 "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; then
  log_ok "Master_Server healthz OK (http://127.0.0.1:8080/healthz)"
else
  log_err "Master_Server healthz FAILED"
  health_ok=false
fi

if curl -fsS --max-time 5 -H "Host: ${DOMAIN}" "http://127.0.0.1/healthz" >/dev/null 2>&1; then
  log_ok "Nginx → healthz OK (http://127.0.0.1/healthz)"
else
  log_err "Nginx → healthz FAILED"
  health_ok=false
fi

if [ "${INSTALL_CLIENT}" = "true" ]; then
  if curl -fsS --max-time 5 -A "Mozilla/5.0 KiroHealth" "http://127.0.0.1:8090/healthz" >/dev/null 2>&1; then
    log_ok "Client_WAF healthz OK (http://127.0.0.1:8090/healthz)"
  else
    log_err "Client_WAF healthz FAILED"
    health_ok=false
  fi
fi

###############################################################################
# Summary
###############################################################################
echo ""
if [ "${health_ok}" = "true" ]; then
  log "Deployment complete — all health checks passed"
else
  log "Deployment complete — some health checks FAILED (see above)"
fi

cat <<EOF

┌─────────────────────────────────────────────────────────────┐
│  Kiro WAF Master Deployment Summary                         │
├─────────────────────────────────────────────────────────────┤
│  Domain:          ${DOMAIN}
│  Master binary:   /usr/local/bin/kiro-master
│  Client binary:   /usr/local/bin/kiro-client-waf
│  XDP object:      ${XDP_OBJ_DIR}/xdp_filter.o
│  Master service:  kiro-master.service
│  Client service:  kiro-client-waf.service
│  Nginx site:      /etc/nginx/sites-available/firewall.conf
│  Master env:      ${ENV_FILE}
│  Client env:      ${CLIENT_ENV_FILE}
│  Database:        ${DB_DIR}/master.db
│  Data dir:        /var/lib/kiro/
├─────────────────────────────────────────────────────────────┤
│  Admin key is stored only in ${ENV_FILE}                    │
│  Keep this file private.                                    │
└─────────────────────────────────────────────────────────────┘

Next steps:
  • Set up TLS: certbot --nginx -d ${DOMAIN}
  • Verify admin access: curl -H "Host: ${DOMAIN}" http://127.0.0.1/admin/
  • Check logs: journalctl -u kiro-master -f

EOF

if [ "${health_ok}" != "true" ]; then
  exit 1
fi
