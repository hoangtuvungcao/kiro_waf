#!/usr/bin/env bash
# =============================================================================
# Kiro WAF Client - Script Cài Đặt Tự Động
# =============================================================================
# Script này tải và cài đặt Kiro WAF Client từ master server.
# KHÔNG yêu cầu source code hoặc Go compiler.
#
# Sử dụng:
#   bash install-client.sh --license-key <LICENSE_KEY> [--xdp-mode] [--quiet]
#
# Script này idempotent - an toàn khi chạy nhiều lần để cập nhật.
# =============================================================================

set -euo pipefail

# --- Script metadata ---
SCRIPT_VERSION="2.0.0"

# --- Cleanup trap to ensure spinner is stopped on exit ---
cleanup_on_exit() {
    if [[ "${SPINNER_ACTIVE:-0}" -eq 1 ]] && [[ -n "${SPINNER_PID:-}" ]]; then
        kill "$SPINNER_PID" 2>/dev/null || true
        wait "$SPINNER_PID" 2>/dev/null || true
        printf "\r\033[K\033[?25h" 2>/dev/null || true
    fi
}
trap cleanup_on_exit EXIT

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

# --- UI State ---
NO_COLOR=0
NO_ANIMATION=0
SCRIPT_START_TIME=""

# =============================================================================
# UI SYSTEM - Hệ thống giao diện cài đặt
# =============================================================================

# --- Phát hiện hỗ trợ màu sắc và animation ---
# Kiểm tra --quiet, TERM=dumb, ! -t 1 → set NO_COLOR và NO_ANIMATION
detect_color_support() {
    if [[ "${QUIET:-0}" == "1" ]] || [[ "${TERM:-}" == "dumb" ]] || [[ ! -t 1 ]]; then
        NO_COLOR=1
        NO_ANIMATION=1
    fi
}

# --- Mã màu nhất quán ---
# Xanh lá (✓) cho thành công, đỏ (✗) cho lỗi, vàng (⚠) cho cảnh báo,
# cyan (→) cho thông tin tiến trình, trắng đậm cho tiêu đề bước
setup_colors() {
    if [[ "$NO_COLOR" == "1" ]]; then
        RED=''
        GREEN=''
        YELLOW=''
        CYAN=''
        BOLD=''
        NC=''
        SYM_SUCCESS="[OK]"
        SYM_ERROR="[ERROR]"
        SYM_WARN="[WARN]"
        SYM_INFO="[INFO]"
    else
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[1;33m'
        CYAN='\033[0;36m'
        BOLD='\033[1;37m'
        NC='\033[0m'
        SYM_SUCCESS="${GREEN}✓${NC}"
        SYM_ERROR="${RED}✗${NC}"
        SYM_WARN="${YELLOW}⚠${NC}"
        SYM_INFO="${CYAN}→${NC}"
    fi
}

# --- Banner ASCII art logo Kiro WAF ---
# Hiển thị logo với màu teal/cyan, phiên bản script, URL master
print_banner() {
    if [[ "$NO_COLOR" == "1" ]]; then
        echo ""
        echo "  _  ___           __        ___    _____ "
        echo " | |/ (_)_ __ ___  \\ \\      / / \\  |  ___|"
        echo " | ' /| | '__/ _ \\  \\ \\ /\\ / / _ \\ | |_   "
        echo " | . \\| | | | (_) |  \\ V  V / ___ \\|  _|  "
        echo " |_|\\_\\_|_|  \\___/    \\_/\\_/_/   \\_\\_|    "
        echo ""
        echo "  Kiro WAF Client Installer v${SCRIPT_VERSION}"
        echo "  ${MASTER_URL}"
        echo ""
    else
        echo ""
        echo -e "${CYAN}  _  ___           __        ___    _____ ${NC}"
        echo -e "${CYAN} | |/ (_)_ __ ___  \\ \\      / / \\  |  ___|${NC}"
        echo -e "${CYAN} | ' /| | '__/ _ \\  \\ \\ /\\ / / _ \\ | |_   ${NC}"
        echo -e "${CYAN} | . \\| | | | (_) |  \\ V  V / ___ \\|  _|  ${NC}"
        echo -e "${CYAN} |_|\\_\\_|_|  \\___/    \\_/\\_/_/   \\_\\_|    ${NC}"
        echo ""
        echo -e "  ${BOLD}Kiro WAF Client Installer${NC} v${CYAN}${SCRIPT_VERSION}${NC}"
        echo -e "  ${CYAN}${MASTER_URL}${NC}"
        echo ""
    fi
}

# --- In bước với số thứ tự [N/T] ---
# Format: [N/T] Step description
# $1 = bước hiện tại, $2 = tổng số bước, $3 = mô tả bước
print_step() {
    local step=$1
    local total=$2
    local message=$3
    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[${step}/${total}] ${message}"
    else
        echo -e "${BOLD}[${step}/${total}]${NC} ${message}"
    fi
}

# --- Hiển thị hoàn tất bước với thời gian ---
# Hiển thị: "✓ Mô tả hoàn tất (3.2s)"
# $1 = message, $2 = thời gian bắt đầu bước (epoch seconds)
step_complete() {
    local message=$1
    local step_start=$2
    local now
    now=$(date +%s.%N 2>/dev/null || date +%s)
    local duration
    duration=$(LC_NUMERIC=C awk "BEGIN {printf \"%.1f\", $now - $step_start}" 2>/dev/null || echo "0.0")

    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[OK] ${message} (${duration}s)"
    else
        echo -e "${GREEN}✓${NC} ${message} (${CYAN}${duration}s${NC})"
    fi
}

# --- Spinner State ---
SPINNER_PID=""
SPINNER_ACTIVE=0

# --- Progress Bar ---
# Hiển thị thanh ngang [████████░░░░░░░░] 50%
# $1 = current bytes, $2 = total bytes, $3 = bar width (min 20, default 40)
# Cập nhật mỗi 1s hoặc 2% tiến trình
show_progress_bar() {
    local current=$1
    local total=$2
    local width=${3:-40}

    # Enforce minimum width of 20 characters
    if [[ $width -lt 20 ]]; then
        width=20
    fi

    # Avoid division by zero
    if [[ $total -le 0 ]]; then
        return
    fi

    # In quiet/no-animation mode, skip rendering
    if [[ "$NO_ANIMATION" == "1" ]]; then
        return
    fi

    local percent=$((current * 100 / total))
    # Clamp percent to 100
    if [[ $percent -gt 100 ]]; then
        percent=100
    fi

    local filled=$((percent * width / 100))
    local empty=$((width - filled))

    # Build the bar string
    local bar=""
    local i
    for ((i = 0; i < filled; i++)); do
        bar+="█"
    done
    for ((i = 0; i < empty; i++)); do
        bar+="░"
    done

    # Render: \r to overwrite current line, [bar] percent%
    if [[ "$NO_COLOR" == "1" ]]; then
        printf "\r[%s] %3d%%" "$bar" "$percent"
    else
        printf "\r${CYAN}[${NC}%s${CYAN}]${NC} %3d%%" "$bar" "$percent"
    fi

    # Print newline when complete
    if [[ $percent -ge 100 ]]; then
        printf "\n"
    fi
}

# --- Detect Unicode support ---
# Check if terminal supports Unicode (Braille characters for spinner)
# Falls back to simple dots animation if Unicode not supported
detect_unicode_support() {
    UNICODE_SUPPORT=1

    # Check if locale supports UTF-8
    if [[ "${LANG:-}" != *UTF-8* ]] && [[ "${LANG:-}" != *utf8* ]] && \
       [[ "${LC_ALL:-}" != *UTF-8* ]] && [[ "${LC_ALL:-}" != *utf8* ]] && \
       [[ "${LC_CTYPE:-}" != *UTF-8* ]] && [[ "${LC_CTYPE:-}" != *utf8* ]]; then
        # Try to detect via locale command
        if command -v locale &>/dev/null; then
            local charmap
            charmap=$(locale charmap 2>/dev/null || echo "")
            if [[ "$charmap" != "UTF-8" ]]; then
                UNICODE_SUPPORT=0
            fi
        else
            UNICODE_SUPPORT=0
        fi
    fi

    # TERM=dumb or linux console often don't support Unicode well
    if [[ "${TERM:-}" == "dumb" ]] || [[ "${TERM:-}" == "linux" ]]; then
        UNICODE_SUPPORT=0
    fi
}

# --- Spinner ---
# Ký tự Braille: ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ (with fallback to simple dots)
# Tốc độ: 100ms per frame (within 80-120ms range)
# $1 = message text to display alongside spinner
start_spinner() {
    local message="${1:-}"

    # In quiet/no-animation mode, just print the message
    if [[ "$NO_ANIMATION" == "1" ]]; then
        if [[ -n "$message" ]]; then
            echo "[...] ${message}"
        fi
        return
    fi

    # Stop any existing spinner
    if [[ $SPINNER_ACTIVE -eq 1 ]]; then
        stop_spinner
    fi

    SPINNER_ACTIVE=1

    # Run spinner in background subshell
    (
        # Use Braille characters if Unicode supported, otherwise simple dots
        if [[ "${UNICODE_SUPPORT:-1}" == "1" ]]; then
            local frames=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
        else
            local frames=("." ".." "..." "   ")
        fi
        local frame_count=${#frames[@]}
        local idx=0

        # Hide cursor
        printf "\033[?25l"

        while true; do
            if [[ "$NO_COLOR" == "1" ]]; then
                printf "\r%s %s" "${frames[$idx]}" "$message"
            else
                printf "\r${CYAN}%s${NC} %s" "${frames[$idx]}" "$message"
            fi
            idx=$(( (idx + 1) % frame_count ))
            # Sleep 100ms (0.1s) — within 80-120ms range
            sleep 0.1
        done
    ) &

    SPINNER_PID=$!
    # Disown so the spinner doesn't produce job control messages
    disown "$SPINNER_PID" 2>/dev/null || true
}

# Dừng spinner và xóa dòng animation
stop_spinner() {
    if [[ $SPINNER_ACTIVE -eq 1 ]] && [[ -n "$SPINNER_PID" ]]; then
        # Kill the spinner background process
        kill "$SPINNER_PID" 2>/dev/null || true
        wait "$SPINNER_PID" 2>/dev/null || true
        SPINNER_PID=""
        SPINNER_ACTIVE=0

        # Clear the spinner line and show cursor
        printf "\r\033[K"
        printf "\033[?25h"
    fi
}

# --- Xóa dòng animation trước khi in lỗi ---
# Đảm bảo spinner/progress bar bị xóa trước khi hiển thị thông báo lỗi
clear_animation_line() {
    # Stop spinner if active
    if [[ $SPINNER_ACTIVE -eq 1 ]]; then
        stop_spinner
    fi
    # Clear current line regardless (handles progress bar case)
    # Only output escape codes if not in quiet/no-color mode
    if [[ "$NO_COLOR" != "1" ]]; then
        printf "\r\033[K"
    fi
}

# --- Hàm tiện ích logging (sử dụng mã màu nhất quán) ---
log_info() {
    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[INFO] $1"
    else
        echo -e "${CYAN}→${NC} $1"
    fi
}

log_success() {
    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[OK] $1"
    else
        echo -e "${GREEN}✓${NC} $1"
    fi
}

log_warn() {
    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[WARN] $1"
    else
        echo -e "${YELLOW}⚠${NC} $1"
    fi
}

log_error() {
    # Clear any active animation before printing error
    clear_animation_line
    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[ERROR] $1"
    else
        echo -e "${RED}✗${NC} $1"
    fi
}

# --- OS Detection ---
# Parses /etc/os-release to detect distribution and version.
# Sets global variables: DISTRO, DISTRO_VERSION, PKG_MANAGER
detect_os() {
    start_spinner "Đang phát hiện hệ điều hành..."

    if [[ ! -f /etc/os-release ]]; then
        stop_spinner
        print_error_with_suggestion \
            "Phát hiện hệ điều hành" \
            "Không tìm thấy /etc/os-release" \
            "Đảm bảo hệ điều hành được hỗ trợ: ${SUPPORTED_DISTROS}"
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
            stop_spinner
            print_error_with_suggestion \
                "Phát hiện hệ điều hành" \
                "Hệ điều hành không được hỗ trợ: ${ID:-unknown}" \
                "Sử dụng một trong các bản phân phối: ${SUPPORTED_DISTROS}"
            exit 1
            ;;
    esac

    stop_spinner
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

    start_spinner "Đang cài đặt dependency thiếu: ${missing_deps[*]}"

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

    stop_spinner
    log_success "Đã cài đặt dependency: ${missing_deps[*]}"
}

# --- Bước 1: Kiểm tra quyền root ---
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error_with_suggestion \
            "Kiểm tra quyền root" \
            "Script yêu cầu quyền root để cài đặt binary và tạo service" \
            "Chạy lại với sudo: sudo bash install-client.sh --license-key <KEY>"
        exit 1
    fi
    log_success "Đang chạy với quyền root"
}

# --- Bước 2: Phân tích tham số ---
parse_args() {
    LICENSE_KEY=""
    XDP_MODE="false"
    QUIET=0

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --license-key)
                if [[ -n "${2:-}" ]]; then
                    LICENSE_KEY="$2"
                    shift 2
                else
                    print_error_with_suggestion \
                        "Phân tích tham số" \
                        "Thiếu giá trị cho --license-key" \
                        "Cung cấp license key: --license-key <YOUR_KEY>"
                    exit 1
                fi
                ;;
            --master-url)
                if [[ -n "${2:-}" ]]; then
                    MASTER_URL="$2"
                    API_BASE="${MASTER_URL}/api/v1/download"
                    shift 2
                else
                    print_error_with_suggestion \
                        "Phân tích tham số" \
                        "Thiếu giá trị cho --master-url" \
                        "Cung cấp URL master server: --master-url https://your-server.com"
                    exit 1
                fi
                ;;
            --xdp-mode)
                XDP_MODE="true"
                shift
                ;;
            --quiet|-q)
                QUIET=1
                shift
                ;;
            --help|-h)
                echo "Sử dụng: bash install-client.sh [--license-key <LICENSE_KEY>] [--master-url <URL>] [--xdp-mode] [--quiet]"
                echo ""
                echo "Tùy chọn:"
                echo "  --license-key <KEY>   License key từ Kiro WAF (tùy chọn, tự đăng ký Community nếu bỏ qua)"
                echo "  --master-url <URL>    URL master server (mặc định: https://firewall.vpsgen.com)"
                echo "  --xdp-mode            Cài đặt dependency build XDP (clang, llvm, libbpf-dev)"
                echo "  --quiet, -q           Tắt animation và màu sắc (cho CI/CD)"
                echo "  --help, -h            Hiển thị trợ giúp"
                exit 0
                ;;
            *)
                print_error_with_suggestion \
                    "Phân tích tham số" \
                    "Tham số không hợp lệ: $1" \
                    "Xem trợ giúp: bash install-client.sh --help"
                exit 1
                ;;
        esac
    done

    # Re-detect color support after parsing --quiet flag
    detect_color_support
    detect_unicode_support
    setup_colors

    if [[ -n "$LICENSE_KEY" ]]; then
        log_success "License key đã được cung cấp: ${LICENSE_KEY:0:8}..."
    else
        log_info "Không có license key — sẽ tự đăng ký gói Community miễn phí"
    fi
    if [[ "$XDP_MODE" == "true" ]]; then
        log_info "Chế độ XDP được bật"
    fi
}

# --- Bước 2b: Tự đăng ký Community license ---
# Gọi POST /api/v1/register trên master server để lấy license key miễn phí.
# Chỉ chạy khi --license-key không được cung cấp.
auto_register_community() {
    if [[ -n "$LICENSE_KEY" ]]; then
        return 0
    fi

    log_info "Đăng ký gói Community miễn phí..."

    # Detect hostname
    local node_hostname
    node_hostname=$(hostname -f 2>/dev/null || hostname 2>/dev/null || echo "unknown")

    # Generate fingerprint from machine-id (SHA-256 hash)
    local fingerprint=""
    if [[ -f /etc/machine-id ]]; then
        fingerprint=$(sha256sum /etc/machine-id | awk '{print $1}')
    elif [[ -f /var/lib/dbus/machine-id ]]; then
        fingerprint=$(sha256sum /var/lib/dbus/machine-id | awk '{print $1}')
    else
        # Fallback: generate from hostname + date (less stable but functional)
        fingerprint=$(echo "${node_hostname}-$(cat /proc/sys/kernel/random/boot_id 2>/dev/null || date +%s)" | sha256sum | awk '{print $1}')
    fi

    # Call POST /api/v1/register
    local register_url="${MASTER_URL}/api/v1/register"
    local payload="{\"hostname\":\"${node_hostname}\",\"fingerprint\":\"${fingerprint}\"}"

    local response
    response=$(curl -sf --connect-timeout 30 \
        -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$register_url" 2>/dev/null) || {
        local curl_exit=$?
        if [[ $curl_exit -eq 6 ]] || [[ $curl_exit -eq 7 ]] || [[ $curl_exit -eq 28 ]]; then
            print_error_with_suggestion \
                "Đăng ký Community" \
                "Không thể kết nối đến master server (${MASTER_URL})" \
                "Kiểm tra kết nối mạng hoặc cung cấp --license-key thủ công"
        elif [[ $curl_exit -eq 22 ]]; then
            print_error_with_suggestion \
                "Đăng ký Community" \
                "Server từ chối yêu cầu đăng ký (có thể bị rate limit)" \
                "Thử lại sau hoặc cung cấp --license-key thủ công"
        else
            print_error_with_suggestion \
                "Đăng ký Community" \
                "Không thể đăng ký tự động với master server" \
                "Cung cấp license key thủ công: --license-key <YOUR_KEY>"
        fi
        exit 1
    }

    # Extract license_key from JSON response
    LICENSE_KEY=$(echo "$response" | grep -o '"license_key":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [[ -z "$LICENSE_KEY" ]]; then
        print_error_with_suggestion \
            "Đăng ký Community" \
            "Response từ server không chứa license key hợp lệ" \
            "Thử lại sau hoặc cung cấp --license-key thủ công"
        exit 1
    fi

    log_success "Đăng ký thành công! License key: ${LICENSE_KEY:0:8}..."
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
        print_error_with_suggestion \
            "Kiểm tra dependency hệ thống" \
            "Không thể cài đặt dependency bắt buộc: ${missing[*]}" \
            "Cài đặt thủ công: sudo ${PKG_MANAGER:-apt} install ${missing[*]}"
        exit 1
    fi

    log_success "Tất cả yêu cầu hệ thống đã đáp ứng"
}

# --- Bước 4: Lấy thông tin phiên bản và checksum ---
fetch_release_info() {
    start_spinner "Đang lấy thông tin phiên bản mới nhất..."

    RELEASE_INFO=$(curl -sf --connect-timeout 30 -H "X-License-Key: ${LICENSE_KEY}" "${API_BASE}/info" 2>/dev/null) || {
        local curl_exit=$?
        stop_spinner
        if [[ $curl_exit -eq 6 ]] || [[ $curl_exit -eq 7 ]] || [[ $curl_exit -eq 28 ]]; then
            print_error_with_suggestion \
                "Lấy thông tin phiên bản" \
                "Không thể kết nối đến master server (${MASTER_URL})" \
                "Kiểm tra kết nối mạng: ping ${MASTER_URL#https://}"
        elif [[ $curl_exit -eq 22 ]]; then
            print_error_with_suggestion \
                "Lấy thông tin phiên bản" \
                "License key không hợp lệ hoặc bị từ chối" \
                "Xác minh license key đúng và còn hiệu lực"
        else
            print_error_with_suggestion \
                "Lấy thông tin phiên bản" \
                "Không thể kết nối đến master server hoặc license key không hợp lệ" \
                "Kiểm tra kết nối mạng và xác minh license key"
        fi
        exit 1
    }

    # Phân tích JSON response
    CLIENT_VERSION=$(echo "$RELEASE_INFO" | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
    CLIENT_SHA256=$(echo "$RELEASE_INFO" | grep -o '"client_sha256":"[^"]*"' | head -1 | cut -d'"' -f4)
    XDP_SHA256=$(echo "$RELEASE_INFO" | grep -o '"xdp_sha256":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [[ -z "$CLIENT_VERSION" || -z "$CLIENT_SHA256" ]]; then
        stop_spinner
        print_error_with_suggestion \
            "Phân tích thông tin phiên bản" \
            "Response từ server không chứa thông tin phiên bản hợp lệ" \
            "Thử lại sau hoặc liên hệ hỗ trợ kỹ thuật"
        exit 1
    fi

    stop_spinner
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

    # Get total file size first for progress bar
    local total_bytes=0
    total_bytes=$(curl -sI --connect-timeout 30 -H "X-License-Key: ${LICENSE_KEY}" \
        "${API_BASE}/client-waf" 2>/dev/null | grep -i "content-length" | awk '{print $2}' | tr -d '\r' || echo "0")

    if [[ "$NO_ANIMATION" == "0" ]] && [[ "$total_bytes" -gt 0 ]]; then
        # Download with progress bar based on bytes downloaded / total bytes
        curl -sf --connect-timeout 30 -H "X-License-Key: ${LICENSE_KEY}" \
            -o "$tmp_file" \
            --write-out "" \
            "${API_BASE}/client-waf" 2>/dev/null &
        local curl_pid=$!

        # Monitor download progress
        local last_percent=0
        local last_update_time
        last_update_time=$(date +%s)

        while kill -0 "$curl_pid" 2>/dev/null; do
            if [[ -f "$tmp_file" ]]; then
                local current_bytes
                current_bytes=$(stat -c%s "$tmp_file" 2>/dev/null || echo "0")
                local current_percent=$((current_bytes * 100 / total_bytes))
                local now
                now=$(date +%s)
                local time_diff=$((now - last_update_time))

                # Update every 1 second or every 2% progress (whichever comes first)
                if [[ $time_diff -ge 1 ]] || [[ $((current_percent - last_percent)) -ge 2 ]]; then
                    show_progress_bar "$current_bytes" "$total_bytes" 40
                    last_percent=$current_percent
                    last_update_time=$now
                fi
            fi
            sleep 0.2
        done

        # Wait for curl to finish and check exit code
        wait "$curl_pid" || {
            clear_animation_line
            print_error_with_suggestion \
                "Tải binary client" \
                "Kết nối bị gián đoạn hoặc server từ chối yêu cầu" \
                "Kiểm tra kết nối mạng và thử lại: bash install-client.sh --license-key <KEY>"
            rm -f "$tmp_file"
            exit 1
        }

        # Show 100% completion
        show_progress_bar "$total_bytes" "$total_bytes" 40
    else
        # Fallback: download without progress bar (quiet mode or unknown size)
        curl -sf --connect-timeout 30 -H "X-License-Key: ${LICENSE_KEY}" \
            -o "$tmp_file" \
            "${API_BASE}/client-waf" || {
            print_error_with_suggestion \
                "Tải binary client" \
                "Không thể tải binary từ server (lỗi mạng hoặc xác thực)" \
                "Kiểm tra kết nối mạng và xác minh license key"
            rm -f "$tmp_file"
            exit 1
        }
    fi

    # Xác minh SHA-256 checksum
    log_info "Đang xác minh checksum SHA-256..."
    local actual_sha256
    actual_sha256=$(sha256sum "$tmp_file" | awk '{print $1}')

    if [[ "$actual_sha256" != "$CLIENT_SHA256" ]]; then
        print_error_with_suggestion \
            "Xác minh checksum SHA-256" \
            "Checksum không khớp — file có thể bị thay đổi trong quá trình tải" \
            "Thử tải lại hoặc kiểm tra kết nối mạng có bị can thiệp"
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
    start_spinner "Đang tạo systemd service..."

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

    stop_spinner
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
    start_spinner "Đang kích hoạt và khởi động service..."

    systemctl daemon-reload

    # Enable service (idempotent — safe to call if already enabled)
    systemctl enable "$SERVICE_NAME" 2>/dev/null

    # Start (or restart) the service
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        # Service is already running (shouldn't happen if we stopped it for update,
        # but handles the case where SKIP_DOWNLOAD=true and service was never stopped)
        stop_spinner
        log_info "Service đang chạy. Giữ nguyên trạng thái."
    else
        systemctl start "$SERVICE_NAME"
        stop_spinner
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

# --- Error Handling with Suggestion ---
# Dừng animation, hiển thị tên bước + nguyên nhân + gợi ý khắc phục
# $1 = step name (tên bước thất bại)
# $2 = cause (nguyên nhân có thể)
# $3 = suggestion (gợi ý hành động khắc phục)
print_error_with_suggestion() {
    local step="${1:-Không xác định}"
    local cause="${2:-Lỗi không xác định}"
    local suggestion="${3:-Kiểm tra log hệ thống}"

    # Stop any active animation (spinner/progress bar)
    clear_animation_line

    if [[ "$NO_COLOR" == "1" ]]; then
        echo "[ERROR] ${step}"
        echo "  Nguyen nhan: ${cause}"
        echo "  -> ${suggestion}"
    else
        echo -e "${RED}✗${NC} ${BOLD}${step}${NC}"
        echo -e "  Nguyên nhân: ${cause}"
        echo -e "  ${YELLOW}→${NC} ${suggestion}"
    fi
}

# --- Bước 11: In tóm tắt với box-drawing characters ---
# Box-drawing characters: ┌─┐│└─┘
# Chứa: version, binary path, service status, IP, lệnh hữu ích
print_summary_box() {
    local box_width=70

    # Detect server IP
    local server_ip=""
    server_ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "N/A")
    if [[ -z "$server_ip" ]]; then
        server_ip="N/A"
    fi

    # Detect service status
    local service_status=""
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        service_status="Đang chạy (active)"
    else
        service_status="Không hoạt động (inactive)"
    fi

    # Calculate total install duration
    local total_duration="N/A"
    if [[ -n "${SCRIPT_START_TIME:-}" ]]; then
        local now
        now=$(date +%s.%N 2>/dev/null || date +%s)
        total_duration=$(LC_NUMERIC=C awk "BEGIN {printf \"%.1f\", $now - $SCRIPT_START_TIME}" 2>/dev/null || echo "N/A")
        total_duration="${total_duration}s"
    fi

    echo ""

    if [[ "$NO_COLOR" == "1" ]]; then
        # Plain text mode — use ASCII box-drawing approximation
        # Use ASCII-only text for proper alignment in non-Unicode terminals
        local plain_service_status=""
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            plain_service_status="Running (active)"
        else
            plain_service_status="Stopped (inactive)"
        fi

        local border_h=""
        for ((i = 0; i < box_width - 2; i++)); do border_h+="-"; done

        echo "+${border_h}+"
        printf "| %-$((box_width - 4))s |\n" "[OK] CAI DAT KIRO WAF CLIENT HOAN TAT"
        echo "+${border_h}+"
        printf "| %-$((box_width - 4))s |\n" ""
        printf "| %-$((box_width - 4))s |\n" "  Phien ban:     ${CLIENT_VERSION}"
        printf "| %-$((box_width - 4))s |\n" "  Binary:        ${INSTALL_BIN}"
        printf "| %-$((box_width - 4))s |\n" "  Service:       ${plain_service_status}"
        printf "| %-$((box_width - 4))s |\n" "  IP Server:     ${server_ip}"
        printf "| %-$((box_width - 4))s |\n" "  Thoi gian:     ${total_duration}"
        printf "| %-$((box_width - 4))s |\n" ""
        printf "| %-$((box_width - 4))s |\n" "  Lenh huu ich:"
        printf "| %-$((box_width - 4))s |\n" "    Trang thai:  systemctl status ${SERVICE_NAME}"
        printf "| %-$((box_width - 4))s |\n" "    Log:         journalctl -u ${SERVICE_NAME} -f"
        printf "| %-$((box_width - 4))s |\n" "    Khoi dong:   systemctl restart ${SERVICE_NAME}"
        printf "| %-$((box_width - 4))s |\n" "    Dung:        systemctl stop ${SERVICE_NAME}"
        printf "| %-$((box_width - 4))s |\n" ""
        echo "+${border_h}+"
    else
        # Unicode box-drawing characters: ┌─┐│└─┘
        local border_h=""
        for ((i = 0; i < box_width - 2; i++)); do border_h+="─"; done

        # Helper function to pad line content to box width
        # Uses a fixed-width approach to avoid UTF-8 multi-byte issues
        _box_line() {
            local content="$1"
            echo -e "${GREEN}│${NC} ${content}"
        }

        echo -e "${GREEN}┌${border_h}┐${NC}"
        echo -e "${GREEN}│${NC} ${GREEN}✓ CÀI ĐẶT KIRO WAF CLIENT HOÀN TẤT${NC}"
        echo -e "${GREEN}├${border_h}┤${NC}"
        echo -e "${GREEN}│${NC}"
        echo -e "${GREEN}│${NC}  Phiên bản:     ${CYAN}${CLIENT_VERSION}${NC}"
        echo -e "${GREEN}│${NC}  Binary:        ${INSTALL_BIN}"
        echo -e "${GREEN}│${NC}  Service:       ${service_status}"
        echo -e "${GREEN}│${NC}  IP Server:     ${CYAN}${server_ip}${NC}"
        echo -e "${GREEN}│${NC}  Thời gian:     ${total_duration}"
        echo -e "${GREEN}│${NC}"
        echo -e "${GREEN}├${border_h}┤${NC}"
        echo -e "${GREEN}│${NC}  ${YELLOW}Lệnh hữu ích:${NC}"
        echo -e "${GREEN}│${NC}    Trạng thái:  systemctl status ${SERVICE_NAME}"
        echo -e "${GREEN}│${NC}    Log:         journalctl -u ${SERVICE_NAME} -f"
        echo -e "${GREEN}│${NC}    Khởi động:   systemctl restart ${SERVICE_NAME}"
        echo -e "${GREEN}│${NC}    Dừng:        systemctl stop ${SERVICE_NAME}"
        echo -e "${GREEN}│${NC}"
        echo -e "${GREEN}└${border_h}┘${NC}"
    fi

    echo ""
}

# --- Main ---
main() {
    # Record script start time
    SCRIPT_START_TIME=$(date +%s.%N 2>/dev/null || date +%s)

    # Initialize UI system - detect color/animation support early
    # (will be re-run after parse_args in case --quiet is passed)
    detect_color_support
    detect_unicode_support
    setup_colors

    # Parse arguments first (may set QUIET flag)
    parse_args "$@"

    # Display banner after color detection is finalized
    print_banner

    # Auto-register Community license if no --license-key provided
    auto_register_community

    check_root
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
    print_summary_box
}

main "$@"
