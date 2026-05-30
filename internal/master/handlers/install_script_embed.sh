#!/usr/bin/env bash
# =============================================================================
# Kiro WAF Client - Script Cài Đặt Tự Động
# =============================================================================
# Script này tải và cài đặt Kiro WAF Client từ master server.
# KHÔNG yêu cầu source code hoặc Go compiler.
#
# Sử dụng:
#   bash install-client.sh --license-key <LICENSE_KEY> [--xdp-mode]
#
# Script này idempotent - an toàn khi chạy nhiều lần để cập nhật.
# =============================================================================

set -euo pipefail

# --- Supported distros ---
SUPPORTED_DISTROS="Ubuntu, Debian, CentOS, Rocky, Fedora, Arch"

# --- Cấu hình ---
MASTER_URL="https://firewall.vpsgen.com"
API_BASE="${MASTER_URL}/api/v1/download"
INSTALL_BIN="/usr/local/bin/kiro-client-waf"
XDP_DIR="/usr/lib/kiro/xdp"
XDP_FILE="${XDP_DIR}/xdp_filter.o"
CONFIG_DIR="/etc/kiro"
ENV_FILE="${CONFIG_DIR}/kiro-client.env"
SERVICE_NAME="kiro-client-waf"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# --- Màu sắc output ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# --- Hàm tiện ích ---
log_info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[CẢNH BÁO]${NC} $1"
}

log_error() {
    echo -e "${RED}[LỖI]${NC} $1"
}

# --- OS Detection ---
# Parses /etc/os-release to detect distribution and version.
# Sets global variables: DISTRO, DISTRO_VERSION, PKG_MANAGER
detect_os() {
    log_info "Đang phát hiện hệ điều hành..."

    if [[ ! -f /etc/os-release ]]; then
        log_error "Không tìm thấy /etc/os-release. Không thể phát hiện hệ điều hành."
        log_error "Các bản phân phối được hỗ trợ: ${SUPPORTED_DISTROS}"
        exit 1
    fi

    # Source os-release to get ID and VERSION_ID
    # shellcheck disable=SC1091
    source /etc/os-release

    DISTRO=""
    DISTRO_VERSION="${VERSION_ID:-unknown}"
    PKG_MANAGER=""

    case "${ID:-}" in
        ubuntu)
            DISTRO="Ubuntu"
            PKG_MANAGER="apt"
            ;;
        debian)
            DISTRO="Debian"
            PKG_MANAGER="apt"
            ;;
        centos)
            DISTRO="CentOS"
            PKG_MANAGER="yum"
            ;;
        rocky)
            DISTRO="Rocky"
            PKG_MANAGER="dnf"
            ;;
        fedora)
            DISTRO="Fedora"
            PKG_MANAGER="dnf"
            ;;
        arch)
            DISTRO="Arch"
            PKG_MANAGER="pacman"
            ;;
        *)
            log_error "Hệ điều hành không được hỗ trợ: ${ID:-unknown}"
            log_error "Các bản phân phối được hỗ trợ: ${SUPPORTED_DISTROS}"
            exit 1
            ;;
    esac

    log_success "Phát hiện OS: ${DISTRO} ${DISTRO_VERSION} (package manager: ${PKG_MANAGER})"
}

# --- Dependency Installation ---
# Installs missing required dependencies using the detected package manager.
# Arguments:
#   $@ - list of package names to install
install_dependencies() {
    local missing_deps=()

    # Check for curl
    if ! command -v curl &>/dev/null; then
        missing_deps+=("curl")
    fi

    # Check for sha256sum (provided by coreutils)
    if ! command -v sha256sum &>/dev/null; then
        case "$PKG_MANAGER" in
            apt)    missing_deps+=("coreutils") ;;
            yum)    missing_deps+=("coreutils") ;;
            dnf)    missing_deps+=("coreutils") ;;
            pacman) missing_deps+=("coreutils") ;;
        esac
    fi

    # Check for systemctl (provided by systemd)
    if ! command -v systemctl &>/dev/null; then
        case "$PKG_MANAGER" in
            apt)    missing_deps+=("systemd") ;;
            yum)    missing_deps+=("systemd") ;;
            dnf)    missing_deps+=("systemd") ;;
            pacman) missing_deps+=("systemd") ;;
        esac
    fi

    # Install XDP build dependencies if --xdp-mode is enabled
    if [[ "${XDP_MODE:-false}" == "true" ]]; then
        log_info "Chế độ XDP được bật - kiểm tra dependency build XDP..."

        if ! command -v clang &>/dev/null; then
            case "$PKG_MANAGER" in
                apt)    missing_deps+=("clang") ;;
                yum)    missing_deps+=("clang") ;;
                dnf)    missing_deps+=("clang") ;;
                pacman) missing_deps+=("clang") ;;
            esac
        fi

        if ! command -v llc &>/dev/null; then
            case "$PKG_MANAGER" in
                apt)    missing_deps+=("llvm") ;;
                yum)    missing_deps+=("llvm") ;;
                dnf)    missing_deps+=("llvm") ;;
                pacman) missing_deps+=("llvm") ;;
            esac
        fi

        # libbpf-dev (package name varies by distro)
        local libbpf_installed=false
        case "$PKG_MANAGER" in
            apt)
                if dpkg -l libbpf-dev &>/dev/null 2>&1; then
                    libbpf_installed=true
                fi
                ;;
            yum|dnf)
                if rpm -q libbpf-devel &>/dev/null 2>&1; then
                    libbpf_installed=true
                fi
                ;;
            pacman)
                if pacman -Qi libbpf &>/dev/null 2>&1; then
                    libbpf_installed=true
                fi
                ;;
        esac

        if [[ "$libbpf_installed" == "false" ]]; then
            case "$PKG_MANAGER" in
                apt)    missing_deps+=("libbpf-dev") ;;
                yum)    missing_deps+=("libbpf-devel") ;;
                dnf)    missing_deps+=("libbpf-devel") ;;
                pacman) missing_deps+=("libbpf") ;;
            esac
        fi
    fi

    # Install missing dependencies
    if [[ ${#missing_deps[@]} -eq 0 ]]; then
        log_success "Tất cả dependency đã được cài đặt"
        return 0
    fi

    log_info "Đang cài đặt dependency thiếu: ${missing_deps[*]}"

    case "$PKG_MANAGER" in
        apt)
            apt-get update -qq
            apt-get install -y -qq "${missing_deps[@]}"
            ;;
        yum)
            yum install -y -q "${missing_deps[@]}"
            ;;
        dnf)
            dnf install -y -q "${missing_deps[@]}"
            ;;
        pacman)
            pacman -Sy --noconfirm "${missing_deps[@]}"
            ;;
    esac

    log_success "Đã cài đặt dependency: ${missing_deps[*]}"
}

# --- Bước 1: Kiểm tra quyền root ---
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "Script này yêu cầu quyền root. Vui lòng chạy với sudo."
        echo "  Sử dụng: sudo bash install-client.sh --license-key <KEY> [--xdp-mode]"
        exit 1
    fi
    log_success "Đang chạy với quyền root"
}

# --- Bước 2: Phân tích tham số ---
parse_args() {
    LICENSE_KEY=""
    XDP_MODE="false"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --license-key)
                if [[ -n "${2:-}" ]]; then
                    LICENSE_KEY="$2"
                    shift 2
                else
                    log_error "Thiếu giá trị cho --license-key"
                    exit 1
                fi
                ;;
            --xdp-mode)
                XDP_MODE="true"
                shift
                ;;
            --help|-h)
                echo "Sử dụng: bash install-client.sh --license-key <LICENSE_KEY> [--xdp-mode]"
                echo ""
                echo "Tùy chọn:"
                echo "  --license-key <KEY>   License key từ Kiro WAF (bắt buộc)"
                echo "  --xdp-mode            Cài đặt dependency build XDP (clang, llvm, libbpf-dev)"
                echo "  --help, -h            Hiển thị trợ giúp"
                exit 0
                ;;
            *)
                log_error "Tham số không hợp lệ: $1"
                echo "  Sử dụng: bash install-client.sh --license-key <KEY> [--xdp-mode]"
                exit 1
                ;;
        esac
    done

    if [[ -z "$LICENSE_KEY" ]]; then
        log_error "License key là bắt buộc."
        echo "  Sử dụng: bash install-client.sh --license-key <KEY> [--xdp-mode]"
        exit 1
    fi

    log_success "License key đã được cung cấp: ${LICENSE_KEY:0:8}..."
    if [[ "$XDP_MODE" == "true" ]]; then
        log_info "Chế độ XDP được bật"
    fi
}

# --- Bước 3: Kiểm tra yêu cầu hệ thống ---
check_requirements() {
    log_info "Kiểm tra yêu cầu hệ thống..."

    # OS detection and dependency installation handle everything now
    detect_os
    install_dependencies

    # Final verification that all required tools are available
    local missing=()
    command -v curl &>/dev/null || missing+=("curl")
    command -v sha256sum &>/dev/null || missing+=("sha256sum")
    command -v systemctl &>/dev/null || missing+=("systemctl")

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Không thể cài đặt dependency bắt buộc: ${missing[*]}"
        exit 1
    fi

    log_success "Tất cả yêu cầu hệ thống đã đáp ứng"
}

# --- Bước 4: Lấy thông tin phiên bản và checksum ---
fetch_release_info() {
    log_info "Đang lấy thông tin phiên bản mới nhất..."

    RELEASE_INFO=$(curl -sf --connect-timeout 30 -H "X-License-Key: ${LICENSE_KEY}" "${API_BASE}/info" 2>/dev/null) || {
        log_error "Không thể kết nối đến master server hoặc license key không hợp lệ."
        log_error "Vui lòng kiểm tra license key và kết nối mạng."
        exit 1
    }

    # Phân tích JSON response
    CLIENT_VERSION=$(echo "$RELEASE_INFO" | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
    CLIENT_SHA256=$(echo "$RELEASE_INFO" | grep -o '"client_sha256":"[^"]*"' | head -1 | cut -d'"' -f4)
    XDP_SHA256=$(echo "$RELEASE_INFO" | grep -o '"xdp_sha256":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [[ -z "$CLIENT_VERSION" || -z "$CLIENT_SHA256" ]]; then
        log_error "Không thể phân tích thông tin phiên bản từ server."
        exit 1
    fi

    log_success "Phiên bản mới nhất: ${CLIENT_VERSION}"
}

# --- Bước 4b: Kiểm tra phiên bản hiện tại (idempotency) ---
# Compares installed binary version with server version.
# Sets SKIP_DOWNLOAD=true if versions match.
check_existing_version() {
    SKIP_DOWNLOAD="false"

    if [[ ! -f "$INSTALL_BIN" ]]; then
        log_info "Chưa có binary cài đặt. Sẽ tải phiên bản mới."
        return 0
    fi

    # Try to get version from existing binary
    local installed_version=""
    if installed_version=$("$INSTALL_BIN" --version 2>/dev/null); then
        # Strip any prefix like "kiro-client-waf " or "v" to get clean version
        installed_version=$(echo "$installed_version" | grep -oP '[\d]+\.[\d]+\.[\d]+[^\s]*' | head -1 || echo "")
    fi

    if [[ -z "$installed_version" ]]; then
        log_info "Không thể xác định phiên bản binary hiện tại. Sẽ tải lại."
        return 0
    fi

    log_info "Phiên bản hiện tại: ${installed_version}"
    log_info "Phiên bản server:   ${CLIENT_VERSION}"

    if [[ "$installed_version" == "$CLIENT_VERSION" ]]; then
        log_success "Binary đã ở phiên bản mới nhất (${CLIENT_VERSION}). Bỏ qua tải xuống."
        SKIP_DOWNLOAD="true"
    else
        log_info "Phiên bản khác nhau. Sẽ cập nhật binary."
    fi
}

# --- Bước 4c: Dừng service trước khi thay thế binary ---
# Stops the running service before binary replacement to avoid file-in-use issues.
stop_service_for_update() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_info "Đang dừng service hiện tại trước khi thay thế binary..."
        systemctl stop "$SERVICE_NAME"
        log_success "Đã dừng service ${SERVICE_NAME}"
    fi
}

# --- Bước 5: Tải binary client ---
download_client_binary() {
    log_info "Đang tải binary kiro-client-waf..."

    local tmp_file="/tmp/kiro-client-waf.download"

    curl -sf --connect-timeout 30 -H "X-License-Key: ${LICENSE_KEY}" \
        -o "$tmp_file" \
        "${API_BASE}/client-waf" || {
        log_error "Không thể tải binary client. Kiểm tra license key và kết nối mạng."
        rm -f "$tmp_file"
        exit 1
    }

    # Xác minh SHA-256 checksum
    log_info "Đang xác minh checksum SHA-256..."
    local actual_sha256
    actual_sha256=$(sha256sum "$tmp_file" | awk '{print $1}')

    if [[ "$actual_sha256" != "$CLIENT_SHA256" ]]; then
        log_error "Checksum không khớp! File có thể bị thay đổi."
        log_error "  Mong đợi: ${CLIENT_SHA256}"
        log_error "  Thực tế:  ${actual_sha256}"
        rm -f "$tmp_file"
        exit 1
    fi

    log_success "Checksum SHA-256 hợp lệ"

    # Cài đặt binary
    mv "$tmp_file" "$INSTALL_BIN"
    chmod 755 "$INSTALL_BIN"
    log_success "Đã cài đặt binary tại ${INSTALL_BIN}"
}

# --- Bước 6: Tải XDP object ---
download_xdp_object() {
    log_info "Đang tải XDP filter object..."

    mkdir -p "$XDP_DIR"

    local tmp_file="/tmp/xdp_filter.o.download"

    curl -sf -H "X-License-Key: ${LICENSE_KEY}" \
        -o "$tmp_file" \
        "${API_BASE}/xdp-filter" || {
        log_warn "Không thể tải XDP object. XDP filtering sẽ không khả dụng."
        rm -f "$tmp_file"
        return 0
    }

    # Xác minh SHA-256 nếu có
    if [[ -n "${XDP_SHA256:-}" ]]; then
        local actual_sha256
        actual_sha256=$(sha256sum "$tmp_file" | awk '{print $1}')
        if [[ "$actual_sha256" != "$XDP_SHA256" ]]; then
            log_warn "Checksum XDP object không khớp. Bỏ qua cài đặt XDP."
            rm -f "$tmp_file"
            return 0
        fi
    fi

    mv "$tmp_file" "$XDP_FILE"
    chmod 644 "$XDP_FILE"
    log_success "Đã cài đặt XDP object tại ${XDP_FILE}"
}

# --- Bước 7: Tạo systemd service ---
create_systemd_service() {
    log_info "Đang tạo systemd service..."

    cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=Kiro WAF Client - Bảo vệ máy chủ web
Documentation=https://firewall.vpsgen.com
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/kiro-client-waf
EnvironmentFile=/etc/kiro/kiro-client.env
Restart=always
RestartSec=5
LimitNOFILE=65535
LimitMEMLOCK=infinity

# Bảo mật
NoNewPrivileges=false
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/log/kiro /var/lib/kiro

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=kiro-client-waf

[Install]
WantedBy=multi-user.target
EOF

    log_success "Đã tạo systemd service: ${SERVICE_FILE}"
}

# --- Bước 8: Tạo thư mục cấu hình ---
# Preserves existing config files on re-run (idempotency).
# Only creates new config if ENV_FILE does not exist.
create_config() {
    log_info "Đang kiểm tra cấu hình..."

    mkdir -p "$CONFIG_DIR"
    mkdir -p /var/log/kiro
    mkdir -p /var/lib/kiro

    # Preserve existing config files — do not overwrite on re-run
    if [[ -f "$ENV_FILE" ]]; then
        log_success "File cấu hình đã tồn tại tại ${ENV_FILE}. Giữ nguyên cấu hình hiện có."
        return 0
    fi

    log_info "Đang tạo cấu hình mới..."

    # Hỏi backend URL cho lần cài đặt đầu tiên
    local backend_url=""

    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Nhập URL backend (website cần bảo vệ)${NC}"
    echo -e "${CYAN}  Ví dụ: http://127.0.0.1:8080 hoặc http://localhost:3000${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    read -rp "Backend URL: " backend_url

    if [[ -z "$backend_url" ]]; then
        backend_url="http://127.0.0.1:8080"
        log_warn "Sử dụng backend URL mặc định: ${backend_url}"
    fi

    # Ghi file environment mới
    cat > "$ENV_FILE" << EOF
# Kiro WAF Client - Cấu hình
# Được tạo bởi install-client.sh vào $(date '+%Y-%m-%d %H:%M:%S')

# License key xác thực với master server
KIRO_LICENSE_KEY=${LICENSE_KEY}

# URL master server
KIRO_MASTER_URL=${MASTER_URL}

# URL backend cần bảo vệ
KIRO_BACKEND_URL=${backend_url}

# Cổng lắng nghe (mặc định: 80)
KIRO_LISTEN_ADDR=:80

# Đường dẫn XDP object
KIRO_XDP_PATH=${XDP_FILE}

# Log level: debug, info, warn, error
KIRO_LOG_LEVEL=info
EOF

    chmod 600 "$ENV_FILE"
    log_success "Đã tạo cấu hình tại ${ENV_FILE}"
}

# --- Bước 9: Kích hoạt và khởi động service ---
# Ensures service is enabled and running after installation or re-run.
# Handles both fresh install and update scenarios.
enable_and_start_service() {
    log_info "Đang kích hoạt và khởi động service..."

    systemctl daemon-reload

    # Enable service (idempotent — safe to call if already enabled)
    systemctl enable "$SERVICE_NAME" 2>/dev/null

    # Start (or restart) the service
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        # Service is already running (shouldn't happen if we stopped it for update,
        # but handles the case where SKIP_DOWNLOAD=true and service was never stopped)
        log_info "Service đang chạy. Giữ nguyên trạng thái."
    else
        systemctl start "$SERVICE_NAME"
    fi

    # Chờ service khởi động
    sleep 2

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_success "Service ${SERVICE_NAME} đang chạy"
    else
        log_warn "Service chưa khởi động. Kiểm tra log: journalctl -u ${SERVICE_NAME}"
    fi
}

# --- Bước 10: Health check ---
health_check() {
    log_info "Đang kiểm tra sức khỏe hệ thống..."

    local retries=3
    local wait=2

    for i in $(seq 1 $retries); do
        if curl -sf -o /dev/null "http://127.0.0.1:80/__kiro/health" 2>/dev/null; then
            log_success "Health check thành công"
            return 0
        fi
        if [[ $i -lt $retries ]]; then
            sleep $wait
        fi
    done

    log_warn "Health check không phản hồi. Service có thể cần thêm thời gian khởi động."
    log_warn "Kiểm tra trạng thái: systemctl status ${SERVICE_NAME}"
}

# --- Bước 11: In tóm tắt ---
print_summary() {
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✓ CÀI ĐẶT KIRO WAF CLIENT HOÀN TẤT${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  Phiên bản:     ${CYAN}${CLIENT_VERSION}${NC}"
    echo -e "  Binary:        ${INSTALL_BIN}"
    echo -e "  XDP Object:    ${XDP_FILE}"
    echo -e "  Cấu hình:      ${ENV_FILE}"
    echo -e "  Service:       ${SERVICE_NAME}"
    echo ""
    echo -e "  ${YELLOW}Lệnh hữu ích:${NC}"
    echo -e "    Trạng thái:  systemctl status ${SERVICE_NAME}"
    echo -e "    Log:         journalctl -u ${SERVICE_NAME} -f"
    echo -e "    Khởi động:   systemctl restart ${SERVICE_NAME}"
    echo -e "    Dừng:        systemctl stop ${SERVICE_NAME}"
    echo ""

    # Kiểm tra trạng thái cuối cùng
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo -e "  Trạng thái:    ${GREEN}● Đang chạy${NC}"
    else
        echo -e "  Trạng thái:    ${YELLOW}○ Không hoạt động${NC}"
    fi

    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# --- Main ---
main() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Kiro WAF Client - Cài Đặt Tự Động${NC}"
    echo -e "${CYAN}  https://firewall.vpsgen.com${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    check_root
    parse_args "$@"
    check_requirements
    fetch_release_info

    # Idempotency: check if binary already at target version
    check_existing_version

    if [[ "$SKIP_DOWNLOAD" == "true" ]]; then
        # Binary is already at the correct version — skip download and binary replacement.
        # Still ensure service file exists, service is enabled+running, and config is preserved.
        create_systemd_service
        create_config
        enable_and_start_service
    else
        # New version available or fresh install — stop service, replace binary, restart.
        stop_service_for_update
        download_client_binary
        download_xdp_object
        create_systemd_service
        create_config
        enable_and_start_service
    fi

    health_check
    print_summary
}

main "$@"
