# Kiro WAF - Build System
# ========================
# Targets:
#   build       - Build all Go binaries (kiro-master, kiro-client, kiro-cli)
#   build-xdp   - Compile XDP/eBPF C code (requires clang/llvm)
#   test        - Run all Go tests
#   clean       - Remove build artifacts
#   all         - Build everything (Go binaries + XDP)

# Build output directory
BUILD_DIR := build

# Module and version info
MODULE := kiro_waf
VERSION_PKG := $(MODULE)/pkg/version
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) \
           -X $(VERSION_PKG).Commit=$(COMMIT) \
           -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)
GOFLAGS := -trimpath
GO := go

# XDP build settings
XDP_SRC := internal/client/xdp/xdp_filter.c
XDP_OUT := $(BUILD_DIR)/xdp_filter.o
CLANG := clang
XDP_CFLAGS := -O2 -target bpf -D__TARGET_ARCH_x86 -Wall

# Binary targets
BINARIES := kiro-master kiro-client kiro-cli

.PHONY: all build build-xdp test clean

## all: Build Go binaries and XDP object
all: build build-xdp

## build: Compile all Go binaries into build/
build: $(addprefix $(BUILD_DIR)/,$(BINARIES))

$(BUILD_DIR)/kiro-master:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $@ ./cmd/kiro-master/

$(BUILD_DIR)/kiro-client:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $@ ./cmd/kiro-client/

$(BUILD_DIR)/kiro-cli:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $@ ./cmd/kiro-cli/

## build-xdp: Compile XDP/eBPF C code (requires clang)
build-xdp: $(XDP_OUT)

$(XDP_OUT): $(XDP_SRC)
	@mkdir -p $(BUILD_DIR)
	$(CLANG) $(XDP_CFLAGS) -c $< -o $@

## test: Run all Go tests
test:
	$(GO) test ./...

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
