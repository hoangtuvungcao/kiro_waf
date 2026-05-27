# System Hardening

Recommended Ubuntu 22.04 LTS baseline:

- SSH keys and limited admin IPs.
- AppArmor.
- auditd or eBPF runtime monitor.
- nftables default drop.
- journald/logrotate limits.
- systemd hardening.

Runtime alerts:

- Web user executing shells or download tools.
- Web process opening unknown outbound connections.
- New executable files in webroot.
- Sensitive files read unexpectedly.
- sudoers, passwd, shadow, SSH config changes.
- New cron jobs or systemd units.

