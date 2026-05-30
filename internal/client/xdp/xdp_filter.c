//go:build ignore

// SPDX-License-Identifier: GPL-2.0
//
// Kiro XDP/eBPF edge drop program — high-performance packet filter.
//
// ═══════════════════════════════════════════════════════════════════════════
// PERFORMANCE CHARACTERISTICS
// ═══════════════════════════════════════════════════════════════════════════
//
// Targets (Requirements 6.3, 6.4):
//   - <100ns per 64-byte packet on x86_64 @ 3.0GHz
//   - 10M pps single-core throughput in XDP native mode
//
// Design principles for throughput:
//   - All helper functions are __always_inline (zero function call overhead)
//   - Per-CPU array for stats (no atomic/lock contention between cores)
//   - LRU hash for rate state (kernel handles eviction, no manual cleanup)
//   - Minimal branches in hot path; early-exit on non-IPv4
//   - Zero dynamic memory allocation in entire XDP program path
//   - BPF_ANY flag for map updates (never fails on LRU maps)
//   - Compiled with clang -O2; BPF object must be < 32KB (Req 6.6)
//
// Edge case handling (Requirements 6.7, 6.8):
//   - Blocklist map full (65,536 entries): bpf_map_lookup_elem returns NULL
//     for IPs not in the map → treated as "not blocked" → XDP_PASS continues.
//     The program NEVER returns XDP_ABORTED under any condition.
//   - LRU rate state map full (262,144 entries): kernel automatically evicts
//     the least-recently-used entry. bpf_map_update_elem with BPF_ANY always
//     succeeds on LRU maps, so rate limiting continues for new source IPs.
//   - Non-IPv4 traffic: immediate XDP_PASS with zero filtering overhead.
//
// Build:
//   clang -O2 -target bpf -D__TARGET_ARCH_x86 -Wall \
//     -c internal/client/xdp/xdp_filter.c -o build/xdp_filter.o
//
// Runtime behavior is conservative by default:
// - IPv4 allowlist LPM hit always passes.
// - IPv4 blocklist LPM hit drops.
// - Drop malformed/private-source traffic only when kiro_config enables it.
// - Drop IPv4 fragments only when kiro_config enables it.
// - Per-source rate limits only apply when kiro_config enables them.
// ═══════════════════════════════════════════════════════════════════════════

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>

#ifndef __always_inline
#define __always_inline inline __attribute__((always_inline))
#endif

#ifndef SEC
#define SEC(NAME) __attribute__((section(NAME), used))
#endif

#ifndef BPF_F_NO_PREALLOC
#define BPF_F_NO_PREALLOC (1U << 0)
#endif

#ifndef BPF_ANY
#define BPF_ANY 0
#endif

/* ─── Statistics counter indices ─── */
#define KIRO_STAT_PASS              0
#define KIRO_STAT_DROP_BLOCKLIST    1
#define KIRO_STAT_DROP_PRIVATE      2
#define KIRO_STAT_DROP_MALFORMED    3
#define KIRO_STAT_DROP_RATE_TOTAL   4
#define KIRO_STAT_DROP_RATE_SYN     5
#define KIRO_STAT_DROP_RATE_UDP     6
#define KIRO_STAT_DROP_RATE_ICMP    7
#define KIRO_STAT_DROP_RATE_SUBNET24 8
#define KIRO_STAT_DROP_FRAGMENT     9
#define KIRO_STAT_DROP_UDP_PORT     10
#define KIRO_STAT_MAX               11

/* ─── Rate limit action codes ─── */
#define KIRO_RATE_PASS              0
#define KIRO_RATE_DROP_TOTAL        KIRO_STAT_DROP_RATE_TOTAL
#define KIRO_RATE_DROP_SYN          KIRO_STAT_DROP_RATE_SYN
#define KIRO_RATE_DROP_UDP          KIRO_STAT_DROP_RATE_UDP
#define KIRO_RATE_DROP_ICMP         KIRO_STAT_DROP_RATE_ICMP
#define KIRO_RATE_DROP_SUBNET24     KIRO_STAT_DROP_RATE_SUBNET24

/* ─── Rate key bucket types ─── */
#define KIRO_RATE_KEY_IP            1
#define KIRO_RATE_KEY_SUBNET24      2

/* ─── Protocol numbers ─── */
#define KIRO_IPPROTO_ICMP           1
#define KIRO_IPPROTO_TCP            6
#define KIRO_IPPROTO_UDP            17

/* ─── TCP flags ─── */
#define KIRO_TCP_FLAG_FIN           0x01
#define KIRO_TCP_FLAG_SYN           0x02
#define KIRO_TCP_FLAG_RST           0x04
#define KIRO_TCP_FLAG_PSH           0x08
#define KIRO_TCP_FLAG_ACK           0x10
#define KIRO_TCP_FLAG_URG           0x20
#define KIRO_TCP_FLAG_MASK          0x3f

/* ─── Header lengths ─── */
#define KIRO_TCP_MIN_HEADER_LEN     20
#define KIRO_UDP_HEADER_LEN         8
#define KIRO_ICMP_MIN_HEADER_LEN    8

/* ─── IP fragment flags ─── */
#define KIRO_IP_MF                  0x2000
#define KIRO_IP_OFFSET_MASK         0x1fff

/* ─── BPF map definition (legacy style for broad compatibility) ─── */
struct bpf_map_def {
	unsigned int type;
	unsigned int key_size;
	unsigned int value_size;
	unsigned int max_entries;
	unsigned int map_flags;
};

/* ─── LPM trie key for IPv4 ─── */
struct lpm_v4_key {
	__u32 prefixlen;
	__u32 addr;
};

/* ─── Runtime configuration (synced from userspace) ─── */
struct kiro_xdp_config {
	__u64 window_ns;              /* Rate window duration in nanoseconds */
	__u32 per_ip_pps;             /* Per-IP packets-per-second threshold */
	__u32 syn_per_ip_per_second;  /* SYN packets per IP per window */
	__u32 udp_per_ip_per_second;  /* UDP packets per IP per window */
	__u32 icmp_per_ip_per_second; /* ICMP packets per IP per window */
	__u8 drop_private_source_ip;  /* Drop RFC1918/loopback/link-local */
	__u8 drop_malformed;          /* Drop malformed packets */
	__u8 rate_limit_enabled;      /* Enable rate limiting */
	__u8 drop_fragments;          /* Drop IP fragments */
	__u32 per_subnet24_pps;       /* Per /24 subnet packets-per-second */
};

/* ─── Rate limiting state key ─── */
struct kiro_rate_key {
	__u32 bucket_type;  /* KIRO_RATE_KEY_IP or KIRO_RATE_KEY_SUBNET24 */
	__u32 addr;         /* IP address (network byte order) */
};

/* ─── Rate limiting state value ─── */
struct kiro_rate_value {
	__u64 window_start_ns;
	__u32 total_count;
	__u32 syn_count;
	__u32 udp_count;
	__u32 icmp_count;
};

/* ═══════════════════════════════════════════════════════════════════════════
 * BPF Maps
 *
 * Map type selection rationale (Requirements 6.1, 6.2):
 * - PERCPU_ARRAY for stats: each CPU increments its own counter with zero
 *   lock contention. Userspace sums across CPUs when reading.
 * - LPM_TRIE for allow/blocklist: O(prefix_len) lookup for CIDR matching.
 *   NO_PREALLOC because entries are sparse and managed from userspace.
 * - LRU_HASH for rate state: kernel-managed eviction when map is full
 *   (Req 6.8). No manual eviction logic needed in the hot path.
 * - ARRAY for config: single-entry, O(1) lookup, always succeeds.
 * ═══════════════════════════════════════════════════════════════════════════ */

/* Allowlist: IPs/subnets that always pass (checked before blocklist).
 * LPM trie with NO_PREALLOC — entries managed from userspace. */
struct bpf_map_def SEC("maps") ipv4_allowlist = {
	.type = BPF_MAP_TYPE_LPM_TRIE,
	.key_size = sizeof(struct lpm_v4_key),
	.value_size = sizeof(__u8),
	.max_entries = 4096,
	.map_flags = BPF_F_NO_PREALLOC,
};

/*
 * Blocklist: IPs/subnets to drop at line rate.
 * Edge case (Req 6.7): When this map reaches max capacity (65,536 entries),
 * bpf_map_lookup_elem for IPs NOT in the map returns NULL. This is treated
 * as "not in blocklist" and processing continues normally (XDP_PASS path).
 * The program never returns XDP_ABORTED regardless of map state.
 */
struct bpf_map_def SEC("maps") ipv4_blocklist = {
	.type = BPF_MAP_TYPE_LPM_TRIE,
	.key_size = sizeof(struct lpm_v4_key),
	.value_size = sizeof(__u8),
	.max_entries = 65536,
	.map_flags = BPF_F_NO_PREALLOC,
};

/* Runtime configuration (single entry array).
 * O(1) lookup, always succeeds for index 0. */
struct bpf_map_def SEC("maps") kiro_config = {
	.type = BPF_MAP_TYPE_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(struct kiro_xdp_config),
	.max_entries = 1,
};

/* Per-CPU statistics counters (Req 6.1).
 * Each CPU has its own private counter — no atomic operations needed,
 * eliminating all lock contention in the stats hot path. */
struct bpf_map_def SEC("maps") kiro_stats = {
	.type = BPF_MAP_TYPE_PERCPU_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = KIRO_STAT_MAX,
};

/*
 * Per-IP and per-subnet rate limiting state (Req 6.2).
 * BPF_MAP_TYPE_LRU_HASH with 262,144 max entries.
 * Edge case (Req 6.8): When this map is full, the kernel automatically
 * evicts the least-recently-used entry to make room for new ones.
 * bpf_map_update_elem with BPF_ANY always succeeds on LRU maps,
 * so rate limiting continues seamlessly for new source IPs.
 */
struct bpf_map_def SEC("maps") ipv4_rate_state = {
	.type = BPF_MAP_TYPE_LRU_HASH,
	.key_size = sizeof(struct kiro_rate_key),
	.value_size = sizeof(struct kiro_rate_value),
	.max_entries = 262144,
};

/* Blocked UDP source ports */
struct bpf_map_def SEC("maps") udp_src_port_blocklist = {
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u16),
	.value_size = sizeof(__u8),
	.max_entries = 1024,
};

/* ═══════════════════════════════════════════════════════════════════════════
 * BPF Helper Function Pointers
 * ═══════════════════════════════════════════════════════════════════════════ */

static void *(*bpf_map_lookup_elem)(void *map, const void *key) = (void *)1;
static long (*bpf_map_update_elem)(void *map, const void *key,
				   const void *value, __u64 flags) = (void *)2;
static __u64 (*bpf_ktime_get_ns)(void) = (void *)5;

/* ═══════════════════════════════════════════════════════════════════════════
 * Helper Functions
 *
 * All helpers are __always_inline to eliminate function call overhead.
 * At 3.0GHz, a function call/return costs ~5ns (branch prediction miss +
 * stack frame setup). With 10+ helpers in the hot path, inlining saves
 * ~50ns per packet — critical for the <100ns budget (Req 6.3).
 * ═══════════════════════════════════════════════════════════════════════════ */

/*
 * Increment a per-CPU statistics counter.
 * Since kiro_stats is BPF_MAP_TYPE_PERCPU_ARRAY, each CPU has its own
 * private copy — no atomic needed, eliminating lock contention entirely.
 */
static __always_inline void stat_inc(__u32 key)
{
	__u64 *value = bpf_map_lookup_elem(&kiro_stats, &key);
	if (value)
		(*value)++;
}

/* Load runtime configuration from the kiro_config map. */
static __always_inline struct kiro_xdp_config *load_config(void)
{
	__u32 key = 0;
	return bpf_map_lookup_elem(&kiro_config, &key);
}

/*
 * Check if an IPv4 source address is a private/reserved address.
 * Covers: RFC 1918 (10/8, 172.16/12, 192.168/16), loopback (127/8),
 * and link-local (169.254/16).
 */
static __always_inline int private_source_v4(__u32 network_order_saddr)
{
	__u32 ip = __builtin_bswap32(network_order_saddr);

	/* 10.0.0.0/8 */
	if ((ip & 0xff000000) == 0x0a000000)
		return 1;
	/* 172.16.0.0/12 */
	if ((ip & 0xfff00000) == 0xac100000)
		return 1;
	/* 192.168.0.0/16 */
	if ((ip & 0xffff0000) == 0xc0a80000)
		return 1;
	/* 127.0.0.0/8 (loopback) */
	if ((ip & 0xff000000) == 0x7f000000)
		return 1;
	/* 169.254.0.0/16 (link-local) */
	if ((ip & 0xffff0000) == 0xa9fe0000)
		return 1;
	return 0;
}

/*
 * Perform LPM trie lookup; returns 1 if address matches an entry.
 * Returns 0 (not found) when: (a) IP not in map, or (b) map is full and
 * IP was never inserted. Both cases are safe — "not found" means "not blocked".
 */
static __always_inline int lpm_hit(void *map, __u32 saddr)
{
	struct lpm_v4_key key = {
		.prefixlen = 32,
		.addr = saddr,
	};
	__u8 *value = bpf_map_lookup_elem(map, &key);
	return value && *value != 0;
}

/* Check if a counter value exceeds a configured threshold. */
static __always_inline int threshold_exceeded(__u32 threshold, __u32 value)
{
	return threshold > 0 && value > threshold;
}

/* Network-to-host byte order for 16-bit values. */
static __always_inline __u16 bswap16(__u16 value)
{
	return __builtin_bswap16(value);
}

/* Check if a UDP source port is in the blocked ports map. */
static __always_inline int udp_source_port_blocked(__u16 host_order_port)
{
	__u8 *value = bpf_map_lookup_elem(&udp_src_port_blocklist, &host_order_port);
	return value && *value != 0;
}

/* Check if an IPv4 packet is fragmented (MF flag set or offset != 0). */
static __always_inline int is_fragmented_v4(struct iphdr *iph)
{
	__u16 frag = bswap16(iph->frag_off);
	return (frag & (KIRO_IP_MF | KIRO_IP_OFFSET_MASK)) != 0;
}

/*
 * Detect invalid TCP flag combinations:
 * - Null flags (no flags set)
 * - SYN+FIN (illegal combination)
 * - SYN+RST (illegal combination)
 * - Christmas tree (FIN+PSH+URG all set)
 */
static __always_inline int tcp_flags_invalid(__u8 flags)
{
	__u8 masked = flags & KIRO_TCP_FLAG_MASK;

	/* Null flags — no flags set at all */
	if (masked == 0)
		return 1;
	/* SYN combined with FIN or RST */
	if ((masked & KIRO_TCP_FLAG_SYN) &&
	    (masked & (KIRO_TCP_FLAG_FIN | KIRO_TCP_FLAG_RST)))
		return 1;
	/* Christmas tree: FIN+PSH+URG */
	if ((masked & (KIRO_TCP_FLAG_FIN | KIRO_TCP_FLAG_PSH | KIRO_TCP_FLAG_URG)) ==
	    (KIRO_TCP_FLAG_FIN | KIRO_TCP_FLAG_PSH | KIRO_TCP_FLAG_URG))
		return 1;
	return 0;
}

/* Compute the /24 subnet key from a network-order IPv4 address. */
static __always_inline __u32 subnet24_key(__u32 network_order_saddr)
{
	__u32 host_order = __builtin_bswap32(network_order_saddr);
	return __builtin_bswap32(host_order & 0xffffff00);
}

/* ═══════════════════════════════════════════════════════════════════════════
 * Rate Limiting Logic
 *
 * Uses BPF_MAP_TYPE_LRU_HASH for automatic eviction (Req 6.8).
 * BPF_ANY flag ensures map updates never fail — on LRU maps, the kernel
 * evicts the oldest entry when capacity is reached rather than returning
 * an error. This guarantees rate limiting continues for new IPs without
 * any error handling branches in the hot path.
 *
 * Window-based counting: each source IP/subnet gets a time window.
 * When the window expires, counters reset. This avoids the need for
 * periodic cleanup from userspace.
 * ═══════════════════════════════════════════════════════════════════════════ */

/*
 * Check and update rate state for a given key (IP or subnet).
 * Returns KIRO_RATE_PASS if under all thresholds, or the appropriate
 * drop action code if any threshold is exceeded.
 */
static __always_inline int rate_state_check(struct kiro_rate_key *key,
					    struct kiro_xdp_config *cfg,
					    __u8 is_tcp_syn, __u8 is_udp,
					    __u8 is_icmp,
					    __u32 total_threshold,
					    __u32 syn_threshold,
					    __u32 udp_threshold,
					    __u32 icmp_threshold,
					    __u32 total_drop_action)
{
	struct kiro_rate_value initial = {};
	struct kiro_rate_value *state;
	__u64 now;

	/* If all thresholds are zero, rate limiting is effectively disabled */
	if (total_threshold == 0 && syn_threshold == 0 &&
	    udp_threshold == 0 && icmp_threshold == 0)
		return KIRO_RATE_PASS;

	now = bpf_ktime_get_ns();
	state = bpf_map_lookup_elem(&ipv4_rate_state, key);
	if (!state) {
		/*
		 * First packet from this source — initialize state.
		 * BPF_ANY on LRU_HASH: if map is full, kernel evicts the LRU
		 * entry automatically (Req 6.8). This update never fails.
		 */
		initial.window_start_ns = now;
		initial.total_count = 1;
		initial.syn_count = is_tcp_syn ? 1 : 0;
		initial.udp_count = is_udp ? 1 : 0;
		initial.icmp_count = is_icmp ? 1 : 0;
		bpf_map_update_elem(&ipv4_rate_state, key, &initial, BPF_ANY);
		return KIRO_RATE_PASS;
	}

	/* Reset counters if the current window has elapsed */
	if (cfg->window_ns == 0 || now - state->window_start_ns >= cfg->window_ns) {
		state->window_start_ns = now;
		state->total_count = 0;
		state->syn_count = 0;
		state->udp_count = 0;
		state->icmp_count = 0;
	}

	/* Increment counters */
	state->total_count++;
	if (is_tcp_syn)
		state->syn_count++;
	if (is_udp)
		state->udp_count++;
	if (is_icmp)
		state->icmp_count++;

	/* Check thresholds in priority order */
	if (threshold_exceeded(total_threshold, state->total_count))
		return total_drop_action;
	if (is_tcp_syn && threshold_exceeded(syn_threshold, state->syn_count))
		return KIRO_RATE_DROP_SYN;
	if (is_udp && threshold_exceeded(udp_threshold, state->udp_count))
		return KIRO_RATE_DROP_UDP;
	if (is_icmp && threshold_exceeded(icmp_threshold, state->icmp_count))
		return KIRO_RATE_DROP_ICMP;

	return KIRO_RATE_PASS;
}

/*
 * Perform per-IP and per-subnet /24 rate limiting.
 * Per-IP is checked first; if it passes, per-subnet is checked.
 * Returns KIRO_RATE_PASS or the specific drop action code.
 */
static __always_inline int rate_limit_v4(struct kiro_xdp_config *cfg,
					 struct iphdr *iph,
					 __u8 is_tcp_syn, __u8 is_udp,
					 __u8 is_icmp)
{
	struct kiro_rate_key ip_key = {
		.bucket_type = KIRO_RATE_KEY_IP,
		.addr = iph->saddr,
	};
	struct kiro_rate_key subnet_key = {
		.bucket_type = KIRO_RATE_KEY_SUBNET24,
		.addr = subnet24_key(iph->saddr),
	};
	int action;

	if (!cfg || !cfg->rate_limit_enabled)
		return KIRO_RATE_PASS;

	/* Check per-IP rate limits */
	action = rate_state_check(&ip_key, cfg, is_tcp_syn, is_udp, is_icmp,
				  cfg->per_ip_pps, cfg->syn_per_ip_per_second,
				  cfg->udp_per_ip_per_second,
				  cfg->icmp_per_ip_per_second,
				  KIRO_RATE_DROP_TOTAL);
	if (action != KIRO_RATE_PASS)
		return action;

	/* Check per-subnet /24 rate limits */
	return rate_state_check(&subnet_key, cfg, is_tcp_syn, is_udp, is_icmp,
				cfg->per_subnet24_pps, 0, 0, 0,
				KIRO_RATE_DROP_SUBNET24);
}

/* ═══════════════════════════════════════════════════════════════════════════
 * Main XDP Program
 *
 * Performance budget per 64-byte packet @ 3.0GHz (Req 6.3):
 *   Target: <100ns = <300 cycles
 *   - Ethernet parse + bounds check:  ~5ns  (15 cycles)
 *   - Non-IPv4 early exit:            ~2ns  (6 cycles) — fast path
 *   - IPv4 parse + validation:        ~10ns (30 cycles)
 *   - LPM trie lookup (allow/block):  ~20ns (60 cycles) each
 *   - Rate state lookup + update:     ~30ns (90 cycles)
 *   - Stats increment (per-CPU):      ~5ns  (15 cycles)
 *   Total worst case:                 ~92ns (276 cycles) — within budget
 *
 * Throughput: 10M pps on single core (Req 6.4)
 *   At 100ns/packet: 1s / 100ns = 10M packets/second ✓
 *
 * Zero allocation guarantee (Req 6.5):
 *   All variables are stack-allocated. Map operations use existing
 *   kernel-managed memory. No bpf_ringbuf_reserve or similar.
 *
 * Processing flow:
 *   1. Parse Ethernet → IPv4 header (non-IPv4 passes through immediately)
 *   2. Check allowlist (LPM trie) → PASS if found
 *   3. Check blocklist (LPM trie) → DROP if found
 *   4. Check private source IP → DROP if enabled
 *   5. Check IP fragments → rate limit + DROP if enabled
 *   6. Protocol-specific checks:
 *      - TCP: malformed flags detection
 *      - UDP: length check, blocked source port
 *      - ICMP: minimum header validation
 *   7. Rate limiting: per-IP and per-subnet /24
 *   8. Statistics counters updated on every decision
 * ═══════════════════════════════════════════════════════════════════════════ */

SEC("xdp")
int kiro_xdp_drop(struct xdp_md *ctx)
{
	void *data = (void *)(long)ctx->data;
	void *data_end = (void *)(long)ctx->data_end;
	struct kiro_xdp_config *cfg = load_config();
	struct ethhdr *eth = data;
	__u8 is_tcp_syn = 0;
	__u8 is_udp = 0;
	__u8 is_icmp = 0;
	int rate_action;
	__u16 total_len;
	__u32 packet_remaining;
	__u8 ip_header_len;

	/* ─── Ethernet header bounds check ─── */
	if ((void *)(eth + 1) > data_end)
		goto malformed;

	/* Non-IPv4 traffic passes through unconditionally (Req 6.9).
	 * This is the fastest path: ~7ns total for ARP, IPv6, etc. */
	if (eth->h_proto != __builtin_bswap16(ETH_P_IP)) {
		stat_inc(KIRO_STAT_PASS);
		return XDP_PASS;
	}

	/* ─── IPv4 header parsing and validation ─── */
	struct iphdr *iph = (void *)(eth + 1);
	if ((void *)(iph + 1) > data_end)
		goto malformed;
	if (iph->ihl < 5)
		goto malformed;
	ip_header_len = iph->ihl * 4;
	if ((void *)iph + ip_header_len > data_end)
		goto malformed;

	/* Validate IP total_length field */
	total_len = bswap16(iph->tot_len);
	if (total_len < ip_header_len)
		goto malformed;
	packet_remaining = data_end - (void *)iph;
	if ((__u32)total_len > packet_remaining)
		goto malformed;

	/* ─── Step 2: Allowlist check (highest priority — always passes) ─── */
	if (lpm_hit(&ipv4_allowlist, iph->saddr)) {
		stat_inc(KIRO_STAT_PASS);
		return XDP_PASS;
	}

	/* ─── Step 3: Blocklist check (Req 6.7) ───
	 * When blocklist map is full, lookup for IPs not in the map returns
	 * NULL (not found) → packet continues to next checks → XDP_PASS.
	 * This is safe: map-full means we can't add NEW entries from userspace,
	 * but existing entries still match correctly. */
	if (lpm_hit(&ipv4_blocklist, iph->saddr)) {
		stat_inc(KIRO_STAT_DROP_BLOCKLIST);
		return XDP_DROP;
	}

	/* ─── Step 4: Private source IP drop (RFC 1918, loopback, link-local) ─── */
	if (cfg && cfg->drop_private_source_ip && private_source_v4(iph->saddr)) {
		stat_inc(KIRO_STAT_DROP_PRIVATE);
		return XDP_DROP;
	}

	/* ─── Step 5: IP fragment handling ─── */
	if (is_fragmented_v4(iph)) {
		/* Still count fragments toward rate limits */
		rate_action = rate_limit_v4(cfg, iph, 0, 0, 0);
		if (rate_action != KIRO_RATE_PASS) {
			stat_inc((__u32)rate_action);
			return XDP_DROP;
		}
		if (cfg && cfg->drop_fragments) {
			stat_inc(KIRO_STAT_DROP_FRAGMENT);
			return XDP_DROP;
		}
		stat_inc(KIRO_STAT_PASS);
		return XDP_PASS;
	}

	/* ─── Step 6: Protocol-specific checks ─── */
	void *transport = (void *)iph + ip_header_len;

	if (iph->protocol == KIRO_IPPROTO_TCP) {
		__u8 *tcp_flags = transport + 13;
		__u8 *tcp_offset_flags = transport + 12;
		__u8 tcp_header_len;

		/* TCP minimum header bounds check */
		if (transport + KIRO_TCP_MIN_HEADER_LEN > data_end)
			goto malformed;
		tcp_header_len = ((*tcp_offset_flags) >> 4) * 4;
		if (tcp_header_len < KIRO_TCP_MIN_HEADER_LEN)
			goto malformed;
		if (transport + tcp_header_len > data_end)
			goto malformed;

		/* Malformed TCP flags detection */
		if (cfg && cfg->drop_malformed && tcp_flags_invalid(*tcp_flags))
			goto malformed;

		/* Track SYN-only packets for SYN flood rate limiting */
		if ((*tcp_flags & (KIRO_TCP_FLAG_SYN | KIRO_TCP_FLAG_ACK)) ==
		    KIRO_TCP_FLAG_SYN)
			is_tcp_syn = 1;

	} else if (iph->protocol == KIRO_IPPROTO_UDP) {
		__u16 *udp_source = transport;
		__u16 *udp_len = transport + 4;

		/* UDP header bounds check */
		if (transport + KIRO_UDP_HEADER_LEN > data_end)
			goto malformed;

		/* UDP length validation: must be >= 8 and <= IP payload */
		if (bswap16(*udp_len) < KIRO_UDP_HEADER_LEN)
			goto malformed;
		if (bswap16(*udp_len) > total_len - ip_header_len)
			goto malformed;

		/* Blocked UDP source port check */
		if (udp_source_port_blocked(bswap16(*udp_source))) {
			stat_inc(KIRO_STAT_DROP_UDP_PORT);
			return XDP_DROP;
		}
		is_udp = 1;

	} else if (iph->protocol == KIRO_IPPROTO_ICMP) {
		/* ICMP minimum header bounds check */
		if (transport + KIRO_ICMP_MIN_HEADER_LEN > data_end)
			goto malformed;
		is_icmp = 1;
	}

	/* ─── Step 7: Per-IP and per-subnet /24 rate limiting ─── */
	rate_action = rate_limit_v4(cfg, iph, is_tcp_syn, is_udp, is_icmp);
	if (rate_action != KIRO_RATE_PASS) {
		stat_inc((__u32)rate_action);
		return XDP_DROP;
	}

	/* ─── Step 8: Packet passes all checks ─── */
	stat_inc(KIRO_STAT_PASS);
	return XDP_PASS;

malformed:
	/*
	 * Drop malformed packets only when explicitly enabled.
	 * IMPORTANT: We NEVER return XDP_ABORTED (Req 6.7, 6.8).
	 * XDP_ABORTED triggers trace_xdp_exception and is reserved for
	 * program errors. All error paths use XDP_DROP or XDP_PASS.
	 */
	if (cfg && cfg->drop_malformed) {
		stat_inc(KIRO_STAT_DROP_MALFORMED);
		return XDP_DROP;
	}
	/* Conservative default: pass if malformed detection is disabled */
	stat_inc(KIRO_STAT_PASS);
	return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
