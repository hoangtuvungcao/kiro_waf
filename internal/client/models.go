// Package client định nghĩa các data models bổ sung cho Client_WAF.
// Bao gồm: BanEntry, XDPConfig.
package client

import "time"

// BanEntry đại diện cho một IP bị ban trong hệ thống.
type BanEntry struct {
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expires_at"`
	Reason    string    `json:"reason"`
}

// XDPConfig chứa cấu hình runtime cho XDP_Filter kernel program.
type XDPConfig struct {
	WindowNS            uint64 `json:"window_ns"`
	PerIPPPS            uint32 `json:"per_ip_pps"`
	SynPerIPPerSecond   uint32 `json:"syn_per_ip_per_second"`
	UDPPerIPPerSecond   uint32 `json:"udp_per_ip_per_second"`
	ICMPPerIPPerSecond  uint32 `json:"icmp_per_ip_per_second"`
	DropPrivateSourceIP uint8  `json:"drop_private_source_ip"`
	DropMalformed       uint8  `json:"drop_malformed"`
	RateLimitEnabled    uint8  `json:"rate_limit_enabled"`
	DropFragments       uint8  `json:"drop_fragments"`
	PerSubnet24PPS      uint32 `json:"per_subnet24_pps"`
}
