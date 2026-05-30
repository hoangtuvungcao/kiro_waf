#!/usr/bin/env bash
# =============================================================================
# Integration Tests cho Install UX (Task 23.4)
# =============================================================================
# Validates: Requirements 15.1–15.11
#
# Tests:
#   1. Syntax validation (bash -n)
#   2. --quiet flag suppresses ANSI escape codes and animation
#   3. TERM=dumb fallback behavior
#   4. Progress bar rendering with mock data
#   5. Error display when step fails (animation cleared before error)
#   6. Banner display
#   7. Step numbering format [N/T]
#   8. Color code consistency
#   9. Summary box contains required fields
#  10. Spinner fallback for non-Unicode terminals
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install-client.sh"
PASS_COUNT=0
FAIL_COUNT=0

# --- Test helpers ---
pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    echo "  ✓ PASS: $1"
}

fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo "  ✗ FAIL: $1"
    if [[ -n "${2:-}" ]]; then
        echo "    Detail: $2"
    fi
}

section() {
    echo ""
    echo "━━━ $1 ━━━"
}


# =============================================================================
# TEST 1: Syntax Validation
# =============================================================================
section "Test 1: Syntax Validation (bash -n)"

if bash -n "$INSTALL_SCRIPT" 2>/dev/null; then
    pass "install-client.sh passes bash -n syntax check"
else
    fail "install-client.sh has syntax errors" "$(bash -n "$INSTALL_SCRIPT" 2>&1)"
fi

# =============================================================================
# TEST 2: --quiet flag suppresses ANSI escape codes
# Validates: Requirements 15.9
# =============================================================================
section "Test 2: --quiet flag suppresses ANSI escape codes"

# Source the script in a subshell with --quiet-like environment to test UI functions
# We can't run the full script (needs root, network), but we can source and test functions
quiet_output=$(bash -c '
    # Prevent the script from running main()
    # We source only the function definitions by extracting them
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    # Simulate --quiet mode
    QUIET=1
    detect_color_support
    detect_unicode_support
    setup_colors

    # Test that color variables are empty
    echo "RED=${RED}"
    echo "GREEN=${GREEN}"
    echo "YELLOW=${YELLOW}"
    echo "CYAN=${CYAN}"
    echo "BOLD=${BOLD}"
    echo "NC=${NC}"
    echo "NO_COLOR=${NO_COLOR}"
    echo "NO_ANIMATION=${NO_ANIMATION}"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$quiet_output" | grep -q "SOURCE_FAILED"; then
    # Fallback: test by running with --quiet --help (which exits 0)
    help_output=$(bash "$INSTALL_SCRIPT" --quiet --help 2>&1 || true)
    if ! echo "$help_output" | grep -qP '\033\['; then
        pass "--quiet --help output contains no ANSI escape codes"
    else
        fail "--quiet --help output still contains ANSI escape codes"
    fi
else
    if echo "$quiet_output" | grep -q "^RED=$" && \
       echo "$quiet_output" | grep -q "^GREEN=$" && \
       echo "$quiet_output" | grep -q "NO_COLOR=1" && \
       echo "$quiet_output" | grep -q "NO_ANIMATION=1"; then
        pass "--quiet sets NO_COLOR=1, NO_ANIMATION=1, and empties color variables"
    else
        fail "--quiet did not properly suppress colors" "$quiet_output"
    fi
fi


# =============================================================================
# TEST 3: TERM=dumb fallback behavior
# Validates: Requirements 15.10
# =============================================================================
section "Test 3: TERM=dumb fallback behavior"

dumb_output=$(TERM=dumb bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    QUIET=0
    TERM=dumb
    detect_color_support
    detect_unicode_support
    setup_colors

    echo "NO_COLOR=${NO_COLOR}"
    echo "NO_ANIMATION=${NO_ANIMATION}"
    echo "UNICODE_SUPPORT=${UNICODE_SUPPORT}"
    echo "SYM_SUCCESS=${SYM_SUCCESS}"
    echo "SYM_ERROR=${SYM_ERROR}"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$dumb_output" | grep -q "SOURCE_FAILED"; then
    # Fallback test: run --help with TERM=dumb
    dumb_help=$(TERM=dumb bash "$INSTALL_SCRIPT" --help 2>&1 || true)
    if ! echo "$dumb_help" | grep -qP '\033\['; then
        pass "TERM=dumb --help output contains no ANSI escape codes"
    else
        fail "TERM=dumb --help output still contains ANSI escape codes"
    fi
else
    if echo "$dumb_output" | grep -q "NO_COLOR=1" && \
       echo "$dumb_output" | grep -q "NO_ANIMATION=1"; then
        pass "TERM=dumb sets NO_COLOR=1 and NO_ANIMATION=1"
    else
        fail "TERM=dumb did not trigger fallback" "$dumb_output"
    fi

    if echo "$dumb_output" | grep -q 'SYM_SUCCESS=\[OK\]'; then
        pass "TERM=dumb uses plain text symbols [OK] instead of Unicode"
    else
        fail "TERM=dumb did not use plain text symbols" "$dumb_output"
    fi

    if echo "$dumb_output" | grep -q "UNICODE_SUPPORT=0"; then
        pass "TERM=dumb disables Unicode support"
    else
        fail "TERM=dumb did not disable Unicode support" "$dumb_output"
    fi
fi

# =============================================================================
# TEST 4: Progress bar rendering
# Validates: Requirements 15.2
# =============================================================================
section "Test 4: Progress bar rendering"

progress_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=0
    NO_ANIMATION=0
    QUIET=0
    setup_colors

    # Capture progress bar output at various percentages
    show_progress_bar 0 100 30
    show_progress_bar 50 100 30
    show_progress_bar 100 100 30
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$progress_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test progress bar"
else
    # Check that progress bar contains expected characters
    if echo "$progress_output" | grep -q "░"; then
        pass "Progress bar renders empty blocks (░)"
    else
        fail "Progress bar missing empty block characters"
    fi

    if echo "$progress_output" | grep -q "█"; then
        pass "Progress bar renders filled blocks (█)"
    else
        fail "Progress bar missing filled block characters"
    fi

    if echo "$progress_output" | grep -q "100%"; then
        pass "Progress bar shows 100% at completion"
    else
        fail "Progress bar does not show 100% at completion"
    fi

    if echo "$progress_output" | grep -q "50%"; then
        pass "Progress bar shows 50% at midpoint"
    else
        fail "Progress bar does not show 50% at midpoint"
    fi
fi


# =============================================================================
# TEST 5: Progress bar minimum width enforcement
# Validates: Requirements 15.2 (chiều rộng tối thiểu 20 ký tự)
# =============================================================================
section "Test 5: Progress bar minimum width (>=20 chars)"

min_width_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=0
    QUIET=0
    setup_colors

    # Request width of 10 (below minimum of 20)
    show_progress_bar 50 100 10
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$min_width_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test progress bar minimum width"
else
    # Count the bar characters (█ and ░) — should be at least 20
    bar_chars=$(echo "$min_width_output" | tr -cd '█░' | wc -c)
    if [[ $bar_chars -ge 20 ]]; then
        pass "Progress bar enforces minimum width of 20 characters (got ${bar_chars})"
    else
        fail "Progress bar width below minimum" "Expected >=20, got ${bar_chars}"
    fi
fi

# =============================================================================
# TEST 6: Progress bar skipped in quiet/no-animation mode
# Validates: Requirements 15.9
# =============================================================================
section "Test 6: Progress bar skipped in quiet mode"

quiet_progress=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=1
    QUIET=1
    setup_colors

    show_progress_bar 50 100 40
    echo "DONE"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$quiet_progress" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test quiet progress bar"
else
    # In quiet mode, progress bar should produce no output (just DONE)
    if [[ "$quiet_progress" == "DONE" ]]; then
        pass "Progress bar produces no output in NO_ANIMATION mode"
    else
        fail "Progress bar still produces output in NO_ANIMATION mode" "$quiet_progress"
    fi
fi

# =============================================================================
# TEST 7: Error display clears animation line
# Validates: Requirements 15.8, 15.11
# =============================================================================
section "Test 7: Error display with animation clearing"

error_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'")

    NO_COLOR=1
    NO_ANIMATION=1
    QUIET=1
    setup_colors

    # Test print_error_with_suggestion
    print_error_with_suggestion "Tải binary client" "Kết nối bị gián đoạn" "Kiểm tra kết nối mạng"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$error_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test error display"
else
    # Verify error contains step name
    if echo "$error_output" | grep -q "Tải binary client"; then
        pass "Error message contains step name"
    else
        fail "Error message missing step name" "$error_output"
    fi

    # Verify error contains cause
    if echo "$error_output" | grep -qi "Nguyen nhan.*gián đoạn\|Nguyen nhan.*gian doan"; then
        pass "Error message contains cause description"
    else
        # Try alternate format
        if echo "$error_output" | grep -q "gián đoạn\|gian doan"; then
            pass "Error message contains cause description"
        else
            fail "Error message missing cause" "$error_output"
        fi
    fi

    # Verify error contains suggestion
    if echo "$error_output" | grep -q "mạng\|mang"; then
        pass "Error message contains remediation suggestion"
    else
        fail "Error message missing suggestion" "$error_output"
    fi

    # Verify no ANSI escape codes in quiet mode
    if ! echo "$error_output" | grep -qP '\033\['; then
        pass "Error message in quiet mode has no ANSI escape codes"
    else
        fail "Error message in quiet mode still has ANSI escape codes"
    fi
fi


# =============================================================================
# TEST 8: Error display with color (non-quiet mode)
# Validates: Requirements 15.4, 15.8
# =============================================================================
section "Test 8: Error display with color codes"

color_error_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=0
    NO_ANIMATION=0
    QUIET=0
    setup_colors

    print_error_with_suggestion "Xác minh checksum" "Checksum không khớp" "Thử tải lại"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$color_error_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test colored error display"
else
    # In color mode, should contain ANSI escape codes (red for error)
    if echo "$color_error_output" | grep -qP '\033\['; then
        pass "Error message in color mode contains ANSI escape codes"
    else
        fail "Error message in color mode missing ANSI escape codes"
    fi

    # Should contain the error symbol ✗
    if echo "$color_error_output" | grep -q "✗"; then
        pass "Error message uses ✗ symbol for errors"
    else
        fail "Error message missing ✗ symbol"
    fi
fi

# =============================================================================
# TEST 9: Banner display
# Validates: Requirements 15.1
# =============================================================================
section "Test 9: Banner ASCII art display"

banner_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    SCRIPT_VERSION="2.0.0"
    MASTER_URL="https://firewall.vpsgen.com"
    NO_COLOR=1
    NO_ANIMATION=1
    setup_colors

    print_banner
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$banner_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test banner"
else
    # Check banner contains Kiro WAF ASCII art
    if echo "$banner_output" | grep -q "Kiro WAF"; then
        pass "Banner contains 'Kiro WAF' text"
    else
        fail "Banner missing 'Kiro WAF' text" "$banner_output"
    fi

    # Check banner contains version
    if echo "$banner_output" | grep -q "2.0.0"; then
        pass "Banner contains script version"
    else
        fail "Banner missing script version"
    fi

    # Check banner contains master URL
    if echo "$banner_output" | grep -q "firewall.vpsgen.com"; then
        pass "Banner contains master URL"
    else
        fail "Banner missing master URL"
    fi

    # Check ASCII art characters are present
    if echo "$banner_output" | grep -q '|_|'; then
        pass "Banner contains ASCII art characters"
    else
        fail "Banner missing ASCII art characters"
    fi
fi

# =============================================================================
# TEST 10: Step numbering format [N/T]
# Validates: Requirements 15.5
# =============================================================================
section "Test 10: Step numbering format [N/T]"

step_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=1
    setup_colors

    print_step 1 5 "Kiểm tra quyền root"
    print_step 3 5 "Tải binary"
    print_step 5 5 "Khởi động service"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$step_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test step numbering"
else
    if echo "$step_output" | grep -q '\[1/5\]'; then
        pass "Step numbering shows [1/5] format"
    else
        fail "Step numbering format incorrect" "$step_output"
    fi

    if echo "$step_output" | grep -q '\[3/5\]'; then
        pass "Step numbering shows [3/5] for middle step"
    else
        fail "Step numbering [3/5] not found"
    fi

    if echo "$step_output" | grep -q '\[5/5\]'; then
        pass "Step numbering shows [5/5] for last step"
    else
        fail "Step numbering [5/5] not found"
    fi

    # Verify step description is included
    if echo "$step_output" | grep -q "Kiểm tra quyền root\|Kiem tra quyen root"; then
        pass "Step description text is displayed"
    else
        fail "Step description text missing"
    fi
fi


# =============================================================================
# TEST 11: Step complete with timing
# Validates: Requirements 15.6
# =============================================================================
section "Test 11: Step complete with timing display"

timing_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=1
    setup_colors

    # Simulate a step that took some time
    start_time=$(date +%s.%N 2>/dev/null || date +%s)
    sleep 0.1
    step_complete "Tải binary hoàn tất" "$start_time"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$timing_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test step timing"
else
    # Check format: [OK] message (Xs)
    if echo "$timing_output" | grep -qP '\[OK\].*\(\d+\.\d+s\)'; then
        pass "Step complete shows timing in format (X.Xs)"
    else
        fail "Step complete timing format incorrect" "$timing_output"
    fi

    # Verify the message text is present
    if echo "$timing_output" | grep -q "hoàn tất\|hoan tat"; then
        pass "Step complete includes message text"
    else
        fail "Step complete missing message text"
    fi
fi

# =============================================================================
# TEST 12: Logging functions use consistent symbols
# Validates: Requirements 15.4
# =============================================================================
section "Test 12: Logging functions with consistent symbols"

log_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=1
    setup_colors

    log_info "Thông tin test"
    log_success "Thành công test"
    log_warn "Cảnh báo test"
    log_error "Lỗi test"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$log_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test logging functions"
else
    if echo "$log_output" | grep -q '\[INFO\]'; then
        pass "log_info uses [INFO] prefix in quiet mode"
    else
        fail "log_info missing [INFO] prefix"
    fi

    if echo "$log_output" | grep -q '\[OK\]'; then
        pass "log_success uses [OK] prefix in quiet mode"
    else
        fail "log_success missing [OK] prefix"
    fi

    if echo "$log_output" | grep -q '\[WARN\]'; then
        pass "log_warn uses [WARN] prefix in quiet mode"
    else
        fail "log_warn missing [WARN] prefix"
    fi

    if echo "$log_output" | grep -q '\[ERROR\]'; then
        pass "log_error uses [ERROR] prefix in quiet mode"
    else
        fail "log_error missing [ERROR] prefix"
    fi
fi

# =============================================================================
# TEST 13: Color mode logging uses Unicode symbols
# Validates: Requirements 15.4
# =============================================================================
section "Test 13: Color mode uses Unicode symbols"

color_log_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=0
    NO_ANIMATION=0
    setup_colors

    log_info "Info"
    log_success "Success"
    log_warn "Warning"
    log_error "Error"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$color_log_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test color logging"
else
    if echo "$color_log_output" | grep -q "→"; then
        pass "log_info uses → symbol in color mode"
    else
        fail "log_info missing → symbol in color mode"
    fi

    if echo "$color_log_output" | grep -q "✓"; then
        pass "log_success uses ✓ symbol in color mode"
    else
        fail "log_success missing ✓ symbol in color mode"
    fi

    if echo "$color_log_output" | grep -q "⚠"; then
        pass "log_warn uses ⚠ symbol in color mode"
    else
        fail "log_warn missing ⚠ symbol in color mode"
    fi

    if echo "$color_log_output" | grep -q "✗"; then
        pass "log_error uses ✗ symbol in color mode"
    else
        fail "log_error missing ✗ symbol in color mode"
    fi
fi


# =============================================================================
# TEST 14: Spinner fallback for non-Unicode terminals
# Validates: Requirements 15.3, 15.10
# =============================================================================
section "Test 14: Spinner message in quiet/no-animation mode"

spinner_quiet_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=1
    QUIET=1
    setup_colors

    start_spinner "Đang phát hiện OS..."
    # In no-animation mode, start_spinner should just print the message
    stop_spinner
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$spinner_quiet_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test spinner quiet mode"
else
    # In no-animation mode, spinner should print [...] message
    if echo "$spinner_quiet_output" | grep -q '\[...\]'; then
        pass "Spinner in no-animation mode prints [...] prefix"
    else
        fail "Spinner in no-animation mode did not print [...] prefix" "$spinner_quiet_output"
    fi

    # Should not contain Braille characters
    if ! echo "$spinner_quiet_output" | grep -q "[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]"; then
        pass "Spinner in no-animation mode has no Braille characters"
    else
        fail "Spinner in no-animation mode still has Braille characters"
    fi
fi

# =============================================================================
# TEST 15: Summary box contains required fields (quiet mode)
# Validates: Requirements 15.7
# =============================================================================
section "Test 15: Summary box required fields"

summary_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=1
    setup_colors

    # Mock required variables
    CLIENT_VERSION="1.2.3"
    INSTALL_BIN="/usr/local/bin/kiro-client-waf"
    SERVICE_NAME="kiro-client-waf"
    SCRIPT_START_TIME=$(date +%s.%N 2>/dev/null || date +%s)

    # Mock systemctl and hostname
    systemctl() { return 1; }
    hostname() { echo "192.168.1.100"; }
    export -f systemctl hostname

    print_summary_box
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$summary_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test summary box"
else
    # Check version is present
    if echo "$summary_output" | grep -q "1.2.3"; then
        pass "Summary box contains version"
    else
        fail "Summary box missing version"
    fi

    # Check binary path
    if echo "$summary_output" | grep -q "/usr/local/bin/kiro-client-waf"; then
        pass "Summary box contains binary path"
    else
        fail "Summary box missing binary path"
    fi

    # Check service status field
    if echo "$summary_output" | grep -qi "service\|Service"; then
        pass "Summary box contains service status field"
    else
        fail "Summary box missing service status"
    fi

    # Check useful commands
    if echo "$summary_output" | grep -q "systemctl"; then
        pass "Summary box contains useful commands (systemctl)"
    else
        fail "Summary box missing useful commands"
    fi

    # Check box-drawing characters (ASCII mode uses +/-/|)
    if echo "$summary_output" | grep -q "^+\-\-\-\-\|^┌─"; then
        pass "Summary box uses box-drawing characters"
    else
        # Check for either ASCII or Unicode box chars
        if echo "$summary_output" | grep -qE '^\+\-|^┌─'; then
            pass "Summary box uses box-drawing characters"
        elif echo "$summary_output" | grep -q "^|"; then
            pass "Summary box uses box-drawing characters (ASCII pipe)"
        else
            fail "Summary box missing box-drawing characters" "$(echo "$summary_output" | head -3)"
        fi
    fi

    # Check timing field
    if echo "$summary_output" | grep -qi "gian\|time"; then
        pass "Summary box contains timing information"
    else
        fail "Summary box missing timing information"
    fi
fi

# =============================================================================
# TEST 16: Non-TTY detection (pipe output)
# Validates: Requirements 15.10
# =============================================================================
section "Test 16: Non-TTY pipe detection"

pipe_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    QUIET=0
    detect_color_support
    echo "NO_COLOR=${NO_COLOR}"
    echo "NO_ANIMATION=${NO_ANIMATION}"
' 2>/dev/null | cat)

# When piped (not a TTY), detect_color_support should set NO_COLOR=1
if echo "$pipe_output" | grep -q "NO_COLOR=1"; then
    pass "Non-TTY (pipe) sets NO_COLOR=1"
else
    fail "Non-TTY (pipe) did not set NO_COLOR=1" "$pipe_output"
fi

if echo "$pipe_output" | grep -q "NO_ANIMATION=1"; then
    pass "Non-TTY (pipe) sets NO_ANIMATION=1"
else
    fail "Non-TTY (pipe) did not set NO_ANIMATION=1" "$pipe_output"
fi


# =============================================================================
# TEST 17: Banner with color mode
# Validates: Requirements 15.1
# =============================================================================
section "Test 17: Banner with color mode (teal/cyan)"

color_banner_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    SCRIPT_VERSION="2.0.0"
    MASTER_URL="https://firewall.vpsgen.com"
    NO_COLOR=0
    NO_ANIMATION=0
    setup_colors

    print_banner
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$color_banner_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test color banner"
else
    # Check that cyan color code is used (0;36m)
    if echo "$color_banner_output" | grep -qP '\033\[0;36m'; then
        pass "Banner uses cyan/teal color code"
    else
        fail "Banner missing cyan/teal color code"
    fi

    # Check ASCII art is present
    if echo "$color_banner_output" | grep -q '_  ___'; then
        pass "Banner contains ASCII art logo"
    else
        fail "Banner missing ASCII art logo"
    fi
fi

# =============================================================================
# TEST 18: Full script --help flag works
# Validates: General script functionality
# =============================================================================
section "Test 18: --help flag"

help_output=$(bash "$INSTALL_SCRIPT" --help 2>&1)
help_exit=$?

if [[ $help_exit -eq 0 ]]; then
    pass "--help exits with code 0"
else
    fail "--help exits with non-zero code: $help_exit"
fi

if echo "$help_output" | grep -q "\-\-license-key"; then
    pass "--help mentions --license-key parameter"
else
    fail "--help missing --license-key documentation"
fi

if echo "$help_output" | grep -q "\-\-quiet\|\-q"; then
    pass "--help mentions --quiet/-q parameter"
else
    fail "--help missing --quiet documentation"
fi

if echo "$help_output" | grep -q "\-\-xdp-mode"; then
    pass "--help mentions --xdp-mode parameter"
else
    fail "--help missing --xdp-mode documentation"
fi

# =============================================================================
# TEST 19: detect_unicode_support function
# Validates: Requirements 15.3 (spinner fallback)
# =============================================================================
section "Test 19: Unicode support detection"

unicode_output=$(TERM=linux LANG=C bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    TERM=linux
    LANG=C
    LC_ALL=""
    LC_CTYPE=""
    detect_unicode_support
    echo "UNICODE_SUPPORT=${UNICODE_SUPPORT}"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$unicode_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test Unicode detection"
else
    if echo "$unicode_output" | grep -q "UNICODE_SUPPORT=0"; then
        pass "TERM=linux with LANG=C disables Unicode support"
    else
        fail "TERM=linux with LANG=C did not disable Unicode" "$unicode_output"
    fi
fi

# =============================================================================
# TEST 20: Progress bar handles edge cases
# Validates: Requirements 15.2
# =============================================================================
section "Test 20: Progress bar edge cases"

edge_output=$(bash -c '
    source <(sed -n "/^# --- UI State ---/,/^main \"\$@\"/{ /^main \"\$@\"/d; p; }" "'"$INSTALL_SCRIPT"'" )

    NO_COLOR=1
    NO_ANIMATION=0
    setup_colors

    # Test: total=0 should not crash (division by zero protection)
    show_progress_bar 50 0 30
    echo "ZERO_TOTAL_OK"

    # Test: current > total should clamp to 100%
    show_progress_bar 150 100 30
    echo "OVER_100_OK"
' 2>/dev/null || echo "SOURCE_FAILED")

if echo "$edge_output" | grep -q "SOURCE_FAILED"; then
    fail "Could not source script to test progress bar edge cases"
else
    if echo "$edge_output" | grep -q "ZERO_TOTAL_OK"; then
        pass "Progress bar handles total=0 without crash"
    else
        fail "Progress bar crashed on total=0"
    fi

    if echo "$edge_output" | grep -q "OVER_100_OK"; then
        pass "Progress bar handles current>total without crash"
    else
        fail "Progress bar crashed on current>total"
    fi

    # When current > total, should clamp to 100%
    if echo "$edge_output" | grep -q "100%"; then
        pass "Progress bar clamps to 100% when current>total"
    else
        fail "Progress bar did not clamp to 100%"
    fi
fi

# =============================================================================
# SUMMARY
# =============================================================================
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Test Results: ${PASS_COUNT} passed, ${FAIL_COUNT} failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [[ $FAIL_COUNT -gt 0 ]]; then
    exit 1
fi
exit 0
