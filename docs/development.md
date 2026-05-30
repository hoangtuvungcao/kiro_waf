# Development

## Prerequisites

| Tool | Version | Mục đích |
|------|---------|----------|
| Go | 1.22+ | Build Go binaries |
| clang | 14+ | Compile XDP/eBPF C code |
| llvm | 14+ | eBPF toolchain |
| libbpf-dev | - | eBPF library headers |
| make | - | Build automation |
| git | - | Version control |

### Cài đặt trên Ubuntu

```bash
# Go
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# eBPF toolchain
apt install -y clang llvm libbpf-dev linux-headers-$(uname -r)

# Build tools
apt install -y make git curl jq
```

## Building from Source

### Build All

```bash
# Clone repository
git clone https://github.com/your-org/kiro_waf.git
cd kiro_waf

# Build everything (Go binaries + XDP)
make all
```

### Build Individual Components

```bash
# Chỉ build Go binaries
make build

# Chỉ build XDP filter
make build-xdp

# Build từng binary
make build/kiro-master
make build/kiro-client
make build/kiro-cli
```

### Build Output

```
build/
├── kiro-master      # Master server binary
├── kiro-client      # Client WAF binary
├── kiro-cli         # CLI tool binary
└── xdp_filter.o     # Compiled XDP/eBPF object
```

### Build Flags

Build tự động inject version info:

```bash
# Custom version
VERSION=1.0.0 make build

# Xem version info
./build/kiro-cli version
```

## Running Tests

```bash
# Chạy tất cả tests
make test

# Chạy tests với verbose
go test -v ./...

# Chạy tests cho package cụ thể
go test -v ./client-node/...
go test -v ./cmd/kiro-cli/...
go test -v ./internal/...

# Chạy test cụ thể
go test -v -run TestBanEngine ./client-node/ban/
go test -v -run TestRateLimiter ./client-node/ratelimit/

# Property-based tests
go test -v ./tests/property/...
go test -v -run TestPow ./client-node/challenge/

# Race detection
go test -race ./...

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Project Structure

```
kiro_waf/
├── cmd/                          # Application entry points
│   ├── kiro-master/              # Master server main
│   │   └── main.go
│   ├── kiro-client/              # Client WAF main
│   │   └── main.go
│   └── kiro-cli/                 # CLI tool main
│       ├── main.go
│       ├── status/               # Status subcommand
│       └── update/               # Update subcommand
│
├── client-node/                  # Client WAF core logic
│   ├── client_waf.go            # Main WAF client
│   ├── proxy.go                 # Reverse proxy logic
│   ├── loops.go                 # Background loops
│   ├── models.go                # Data models
│   ├── errors.go                # Error handling
│   ├── ban/                     # IP ban engine
│   │   ├── ban.go
│   │   └── engine.go
│   ├── challenge/               # Bot challenges
│   │   ├── challenge.go         # Challenge orchestrator
│   │   ├── pow.go               # Proof of Work
│   │   └── hold.go              # Hold page
│   ├── cookie/                  # Cookie management
│   │   ├── cookie.go
│   │   └── hmac.go              # HMAC signing
│   ├── ratelimit/               # Rate limiting
│   │   ├── ratelimit.go         # Rate limit logic
│   │   └── limiter.go           # Token bucket limiter
│   └── ua/                      # User-Agent detection
│       ├── ua.go
│       └── detector.go          # Bot/crawler detection
│
├── internal/                     # Internal packages
│   ├── client/                  # Client-specific internal
│   │   ├── ua/                  # UA detection internal
│   │   └── xdp/                 # XDP C source
│   │       └── xdp_filter.c
│   ├── master/                  # Master server internal
│   │   ├── handlers/            # HTTP handlers
│   │   └── models/              # Database models
│   └── shared/                  # Shared packages
│       ├── buildinfo/           # Version info
│       ├── config/              # Config loading
│       ├── diagnostics/         # Health/status/preflight
│       ├── installer/           # Install/uninstall logic
│       ├── machinefingerprint/  # Machine ID
│       ├── pilot/               # Pilot reports
│       └── support/             # Incident reports
│
├── configs/                      # Example configurations
│   ├── kiro.example.yaml        # Simple config
│   └── kiro.advanced.example.yaml # Full config
│
├── deployments/                  # Deployment configs
│   ├── systemd/                 # Service files
│   ├── nginx/                   # Nginx configs
│   ├── nftables/                # Firewall rules
│   ├── sysctl/                  # Kernel params
│   └── apparmor/                # AppArmor profiles
│
├── scripts/                      # Deploy & install scripts
│   ├── install-client.sh
│   ├── deploy_master.sh
│   ├── deploy-all-in-one.sh
│   └── build-xdp.sh
│
├── tests/                        # Integration & property tests
│   └── property/
│       └── pow_test.go
│
├── docs/                         # Documentation
├── build/                        # Build output (gitignored)
├── Makefile                      # Build system
├── go.mod                        # Go module
└── go.sum                        # Go dependencies
```

## Development Workflow

### 1. Feature Development

```bash
# Tạo branch
git checkout -b feature/my-feature

# Code...

# Test
go test ./...

# Build
make build

# Test locally
./build/kiro-cli version
./build/kiro-cli preflight --config configs/kiro.example.yaml --skip-command-checks
```

### 2. XDP Development

```bash
# Edit XDP source
vim internal/client/xdp/xdp_filter.c

# Compile
make build-xdp

# Test (cần root + test interface)
# ip link set dev lo xdpgeneric obj build/xdp_filter.o sec xdp
```

### 3. Testing Changes

```bash
# Unit tests
go test -v ./client-node/ban/
go test -v ./client-node/challenge/
go test -v ./client-node/ratelimit/

# Integration tests
go test -v ./cmd/kiro-cli/

# Property-based tests
go test -v ./tests/property/
```

## Code Style

- Tuân theo Go standard formatting (`gofmt`)
- Dùng `golangci-lint` cho static analysis
- Comments bằng tiếng Anh cho exported symbols
- Comments bằng tiếng Việt cho internal logic (optional)
- Error handling: wrap errors với context

```bash
# Format
gofmt -w .

# Lint (nếu có golangci-lint)
golangci-lint run ./...
```

## Contributing Guidelines

1. Fork repository
2. Tạo feature branch từ `main`
3. Viết tests cho code mới
4. Đảm bảo `make test` pass
5. Đảm bảo `make build` thành công
6. Tạo Pull Request với mô tả rõ ràng

### PR Template

```markdown
## Mô tả
[Mô tả thay đổi]

## Loại thay đổi
- [ ] Bug fix
- [ ] Feature mới
- [ ] Breaking change
- [ ] Documentation

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing done

## Checklist
- [ ] Code follows project style
- [ ] Tests added/updated
- [ ] Documentation updated
```

## Debugging

### Debug Client WAF

```bash
# Chạy với verbose logging
KIRO_LOG_LEVEL=debug ./build/kiro-client --config configs/kiro.example.yaml

# Xem logs
journalctl -u kiro-client-waf -f
tail -f /var/log/kiro/client.log
```

### Debug XDP

```bash
# Xem XDP stats
cat /sys/kernel/debug/tracing/trace_pipe

# Xem attached XDP programs
ip link show dev eth0
bpftool prog list
bpftool map list
```

### Debug Nginx

```bash
# Test config
nginx -t

# Xem error log
tail -f /var/log/nginx/error.log

# Xem access log
tail -f /var/log/nginx/access.log
```
