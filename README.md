[![CI](https://github.com/vantrong/kiro_waf/actions/workflows/ci.yml/badge.svg)](https://github.com/vantrong/kiro_waf/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Proprietary-blue)](./SECURITY.md)

# Kiro WAF

A production-grade Web Application Firewall combining XDP/eBPF packet filtering at 10M pps with a Go reverse proxy handling 100K rps. Designed for single-server protection with centralized management.

## Quick Start

```bash
# Build all binaries
make build

# Run tests
make test

# Deploy to VPS (as root)
sudo bash scripts/deploy_master.sh
```

## Architecture

```
Internet → [XDP/eBPF L3/L4 Filter] → [Go WAF Proxy L7] → Backend App
                                              ↕
                                    [Master Server API]
```

**Two-tier protection:**
- **Layer 3/4** — XDP/eBPF drops malicious packets at wire speed in the kernel
- **Layer 7** — Go reverse proxy performs rate limiting, bot challenges, WAF inspection, and request forwarding

**Operating modes:**
- `server` — Protects the server and network services only (no HTTP proxy)
- `full` — Full website/API protection with reverse proxy, WAF, bot defense, and Cloudflare integration

## Components

| Binary | Install Path | Purpose |
|--------|-------------|---------|
| `kiro-master` | `/usr/local/bin/kiro-master` | Control plane: admin UI, API, license management, update distribution |
| `kiro-client-waf` | `/usr/local/bin/kiro-client-waf` | Edge WAF: reverse proxy, rate limiting, bot challenges, XDP sync |
| `kiro-cli` | `/usr/local/bin/kiro-cli` | CLI tool: diagnostics, install management, OTA updates |

**Systemd services:** `kiro-master.service`, `kiro-client-waf.service`

**Master server:** https://firewall.vpsgen.com

## Build

Requires Go 1.21+ and make. Optional: clang/llvm for XDP.

```bash
make build       # Build Go binaries → build/kiro-master, build/kiro-client, build/kiro-cli
make build-xdp   # Compile XDP/eBPF object → build/xdp_filter.o
make test        # Run all Go tests
make clean       # Remove build artifacts
make all         # Build everything (Go + XDP)
```

Build output:
```
build/
├── kiro-master      # Master server binary
├── kiro-client      # Client WAF binary (installed as kiro-client-waf)
├── kiro-cli         # CLI tool binary
└── xdp_filter.o     # XDP/eBPF object (optional)
```

## Install on Client VPS

```bash
# Community plan (free, auto-registers)
curl -fsSL https://firewall.vpsgen.com/install.sh | bash

# With license key
curl -fsSL https://firewall.vpsgen.com/install.sh | bash -s -- --license-key KIRO-XXXX-XXXX
```

The install script handles OS detection, dependency installation, binary download with SHA-256 verification, systemd service setup, and auto-registration for Community plan.

## Deploy Master + Client (All-in-One)

```bash
git clone https://github.com/vantrong/kiro_waf.git /opt/kiro_waf
cd /opt/kiro_waf
sudo bash scripts/deploy_master.sh
```

This builds from source, installs binaries, configures Nginx, sets up systemd services, and runs health checks.

## Configuration

**Master** reads environment variables from `/etc/kiro-master/master.env`:
- `KIRO_MASTER_ADDR` — Listen address (default `:8080`)
- `KIRO_MASTER_DB` — SQLite database path
- `KIRO_MASTER_ADMIN_KEY` — Admin API key (required)
- `KIRO_MASTER_ADMIN_IPS` — IP allowlist (comma-separated)
- `KIRO_MASTER_SESSION_TTL` — Session TTL (default `12h`)

**Client WAF** reads environment variables from `/etc/kiro/client-waf.env`:
- `KIRO_CLIENT_LISTEN` — Listen address (default `:8090`)
- `KIRO_BACKEND_URL` — Backend to proxy to (required)
- `KIRO_MASTER_URL` — Master server URL (required)
- `KIRO_LICENSE_KEY` — License key (required)
- `KIRO_CLIENT_COOKIE_SECRET` — HMAC secret (required)

See [docs/configuration.md](docs/configuration.md) for the full reference.

## Project Structure

```
kiro_waf/
├── cmd/                    # Binary entry points
│   ├── kiro-master/        # Master server
│   ├── kiro-client/        # Client WAF node
│   └── kiro-cli/           # CLI tool
├── internal/               # Private packages
│   ├── master/             # Master server logic (handlers, db, plan)
│   └── client/             # Client node logic (proxy, ban, challenge, ratelimit)
├── configs/                # Example YAML configurations
├── deployments/            # Deployment configs (systemd, nginx, nftables, sysctl)
├── scripts/                # Install and deploy scripts
├── docs/                   # Documentation
└── tests/                  # Property-based tests
```

## Documentation

| Document | Description |
|----------|-------------|
| [docs/installation.md](docs/installation.md) | Installation guide (auto + manual) |
| [docs/cli-reference.md](docs/cli-reference.md) | CLI command reference |
| [docs/configuration.md](docs/configuration.md) | Configuration reference |
| [docs/deployment.md](docs/deployment.md) | Deployment guide |
| [docs/architecture.md](docs/architecture.md) | Architecture overview |
| [docs/security.md](docs/security.md) | Security design |
| [docs/troubleshooting.md](docs/troubleshooting.md) | Troubleshooting |

## Contributing

```bash
# Install prerequisites (Ubuntu/Debian)
sudo apt install golang-go clang llvm libbpf-dev make

# Build and test
make all
make test
```

1. Fork the repository
2. Create a feature branch from `main`
3. Ensure `make test` passes
4. Submit a pull request
