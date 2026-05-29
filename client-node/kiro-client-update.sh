#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${KIRO_CLIENT_ENV_FILE:-/etc/kiro-client/client.env}"
TARGET_BIN="${KIRO_CLIENT_TARGET_BIN:-/usr/local/bin/kiro-client-waf}"
SERVICE="${KIRO_CLIENT_SERVICE:-kiro-client-waf.service}"

if [ "$(id -u)" != "0" ]; then
  echo "kiro-client-update must run as root" >&2
  exit 2
fi
if [ ! -f "${ENV_FILE}" ]; then
  echo "missing ${ENV_FILE}" >&2
  exit 2
fi

set -a
# shellcheck disable=SC1090
. "${ENV_FILE}"
set +a

MASTER_URL="${KIRO_MASTER_URL:-https://firewall.vpsgen.com}"
COMPONENT="${KIRO_UPDATE_COMPONENT:-kiro-client-waf}"
CHANNEL="${KIRO_UPDATE_CHANNEL:-stable}"
CURRENT_VERSION="${KIRO_CLIENT_VERSION:-0.0.0}"
NODE_ID="${KIRO_NODE_ID:-$(hostname)}"
PUBLIC_IP="${KIRO_PUBLIC_IP:-}"
LICENSE_KEY="${KIRO_LICENSE_KEY:-}"

if [ -z "${LICENSE_KEY}" ]; then
  echo "KIRO_LICENSE_KEY is required" >&2
  exit 2
fi
fingerprint="$("${TARGET_BIN}" --print-fingerprint)"
tmpdir="$(mktemp -d)"
backup="${tmpdir}/kiro-client-waf.backup"
trap 'rm -rf "${tmpdir}"' EXIT

payload="$(python3 - "${LICENSE_KEY}" "${PUBLIC_IP}" "${fingerprint}" "${NODE_ID}" "${COMPONENT}" "${CHANNEL}" "${CURRENT_VERSION}" <<'PY'
import json
import sys
print(json.dumps({
    "license_key": sys.argv[1],
    "client_ip": sys.argv[2],
    "fingerprint_hash": sys.argv[3],
    "node_id": sys.argv[4],
    "component": sys.argv[5],
    "channel": sys.argv[6],
    "current_version": sys.argv[7],
}))
PY
)"

response="$(curl -fsS -X POST "${MASTER_URL%/}/api/v1/update/check" \
  -H 'Content-Type: application/json' \
  -d "${payload}")"

available="$(python3 -c 'import json,sys; print(str(json.load(sys.stdin).get("update_available", False)).lower())' <<<"${response}")"
if [ "${available}" != "true" ]; then
  reason="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("reason","up_to_date"))' <<<"${response}")"
  echo "no update available: ${reason}"
  exit 0
fi

version="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["release"]["version"])' <<<"${response}")"
artifact_url="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["release"]["artifact_url"])' <<<"${response}")"
expected_sha="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["release"]["sha256"])' <<<"${response}")"
download="${tmpdir}/artifact"

echo "downloading ${COMPONENT} ${version}"
curl -fL "${artifact_url}" -o "${download}"
actual_sha="$(sha256sum "${download}" | awk '{print $1}')"
if [ "${actual_sha}" != "${expected_sha}" ]; then
  echo "sha256 mismatch: expected ${expected_sha}, got ${actual_sha}" >&2
  exit 1
fi

candidate="${download}"
case "${artifact_url}" in
  *.tar.gz|*.tgz)
    mkdir -p "${tmpdir}/extract"
    tar -xzf "${download}" -C "${tmpdir}/extract"
    candidate="$(find "${tmpdir}/extract" -type f -name 'kiro-client-waf' -perm -111 | head -n 1)"
    if [ -z "${candidate}" ]; then
      echo "archive does not contain executable kiro-client-waf" >&2
      exit 1
    fi
    ;;
esac

cp -a "${TARGET_BIN}" "${backup}"
install -m 0755 "${candidate}" "${TARGET_BIN}.new"
mv "${TARGET_BIN}.new" "${TARGET_BIN}"

if ! systemctl restart "${SERVICE}"; then
  install -m 0755 "${backup}" "${TARGET_BIN}"
  systemctl restart "${SERVICE}" || true
  echo "service restart failed; rolled back" >&2
  exit 1
fi

sleep 1
if ! curl -fsS -A 'Mozilla/5.0 KiroUpdate' "http://127.0.0.1:8090/healthz" >/dev/null; then
  install -m 0755 "${backup}" "${TARGET_BIN}"
  systemctl restart "${SERVICE}" || true
  echo "health check failed; rolled back" >&2
  exit 1
fi

echo "updated ${COMPONENT} to ${version}"
