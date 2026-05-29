#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

source_path="${1:-ebpf/xdp/kiro_xdp_drop.c}"
output_path="${2:-build/ebpf/kiro_xdp_drop.o}"
target_arch="${KIRO_XDP_TARGET_ARCH:-x86}"
clang_bin="${CLANG:-clang}"
include_args=()

case "$(uname -m)" in
  x86_64)
    if [ -d /usr/include/x86_64-linux-gnu ]; then
      include_args+=("-I" "/usr/include/x86_64-linux-gnu")
    fi
    ;;
  aarch64|arm64)
    if [ -d /usr/include/aarch64-linux-gnu ]; then
      include_args+=("-I" "/usr/include/aarch64-linux-gnu")
    fi
    ;;
esac

mkdir -p "$(dirname "${output_path}")"

"${clang_bin}" \
  -O2 \
  -g \
  -target bpf \
  -D"__TARGET_ARCH_${target_arch}" \
  "${include_args[@]}" \
  -Wall \
  -Werror \
  -c "${source_path}" \
  -o "${output_path}"

printf 'xdp object built: %s\n' "${output_path}"
