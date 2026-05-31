#!/bin/bash
# Kiro WAF - XDP Blocklist Sync
# Reads /var/lib/kiro/xdp-blocklist.txt and syncs to XDP BPF map
BLOCKLIST="${1:-/var/lib/kiro/xdp-blocklist.txt}"
BPFTOOL=$(which bpftool 2>/dev/null || echo "/usr/local/bin/bpftool")

if [[ ! -x "$BPFTOOL" ]]; then
    exit 0  # No bpftool, graceful skip
fi

if [[ ! -f "$BLOCKLIST" ]]; then
    exit 0
fi

# Read each IP/32 and add to XDP blocklist map
while IFS= read -r line; do
    line=$(echo "$line" | tr -d '[:space:]')
    [[ -z "$line" ]] && continue
    [[ "$line" == \#* ]] && continue
    
    # Parse IP from CIDR notation (e.g., 1.2.3.4/32)
    IP="${line%%/*}"
    
    # Convert IP to hex bytes for LPM trie key
    # LPM trie key format: prefix_len (4 bytes) + IP (4 bytes)
    IFS='.' read -r a b c d <<< "$IP"
    
    # key: 0x20 0x00 0x00 0x00 (prefix=32) + IP bytes
    # value: 0x01 (blocked)
    $BPFTOOL map update name ipv4_blocklist \
        key 0x20 0x00 0x00 0x00 $(printf '0x%02x 0x%02x 0x%02x 0x%02x' $a $b $c $d) \
        value 0x01 2>/dev/null || true
done < "$BLOCKLIST"

exit 0
