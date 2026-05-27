#!/usr/bin/env bash
set -euo pipefail

CONFIG="${1:-configs/tenant.server-only.example.yaml}"
STATE_DIR="${KIRO_STATE_DIR:-/var/lib/kiro}"
SNAPSHOT_DIR="${KIRO_SNAPSHOT_DIR:-/var/lib/kiro/last-good-config}"
ROLLBACK_SECONDS="${KIRO_ROLLBACK_SECONDS:-60}"
ACK="KIRO_LAB_FIREWALL_APPLY"

if [[ "$(id -u)" != "0" ]]; then
  echo "Run as root in an Ubuntu 22.04 lab VM only." >&2
  exit 1
fi

command -v nft >/dev/null || {
  echo "nft command is required. Install with: apt-get install -y nftables" >&2
  exit 1
}

echo "== kiro_waf firewall lab apply =="
echo "Config: ${CONFIG}"
echo "State dir: ${STATE_DIR}"
echo "Snapshot dir: ${SNAPSHOT_DIR}"
echo "Rollback seconds: ${ROLLBACK_SECONDS}"
echo
echo "Preflight dry-run:"
go run ./cmd/kiro-agent --config "${CONFIG}" --firewall-dry-run >/tmp/kiro-waf-lab-firewall.nft
nft -c -f /tmp/kiro-waf-lab-firewall.nft
echo "nft syntax check passed."
echo
echo "Applying with pending rollback. Keep a second console open."
go run ./cmd/kiro-agent \
  --config "${CONFIG}" \
  --firewall-apply \
  --firewall-lab-ack "${ACK}" \
  --firewall-state-dir "${STATE_DIR}" \
  --firewall-snapshot-dir "${SNAPSHOT_DIR}" \
  --firewall-rollback-seconds "${ROLLBACK_SECONDS}"

echo
echo "If SSH/web checks are healthy, confirm before the rollback deadline:"
echo "go run ./cmd/kiro-agent --config ${CONFIG} --firewall-confirm --firewall-state-dir ${STATE_DIR}"
echo
echo "If anything is wrong, rollback immediately:"
echo "go run ./cmd/kiro-agent --config ${CONFIG} --firewall-rollback --firewall-lab-ack ${ACK} --firewall-state-dir ${STATE_DIR}"
