# Cloudflare Free

In `full` mode, Cloudflare Free can reduce junk bots, cache static assets, and
hide the origin IP when DNS is configured correctly.

Origin lock is required:

```text
allow 80/443 from Cloudflare IP ranges
drop 80/443 from all other public sources
allow SSH only from admin IPs
```

Only trust `CF-Connecting-IP` when the remote address is a Cloudflare IP.

Cloudflare ranges are stored locally:

```text
rules/cloudflare/ips-v4.txt
rules/cloudflare/ips-v6.txt
```

