# Enterprise Readiness

The current repository is a blueprint and configuration baseline, not yet a
validated production security product.

The architecture is feasible for single-server protection, schools, small
businesses, and small hosting providers. It is not yet ready to be sold as a
fully proven enterprise-grade anti-DDoS/WAF system until implementation,
benchmarks, safety checks, and operational procedures are completed.

Required before production:

- Real `kiro-agent`.
- Real XDP/eBPF programs.
- Safe nftables manager with rollback.
- Proxy config generator.
- WAF integration.
- Signed update flow.
- License activation and rebind workflow.
- Load/attack benchmarks.
- Support bundle.
- Privacy policy.
- Recovery guide.

Positioning should be honest:

```text
kiro_waf protects single servers, websites, and APIs from many common network,
bot, overload, and web attack patterns. It reduces risk and improves resilience.
It does not replace upstream bandwidth protection, global CDN scrubbing, or
secure application development.
```

