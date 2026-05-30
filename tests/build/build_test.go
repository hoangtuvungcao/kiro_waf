//go:build integration

// Package build contains build verification tests that ensure the project
// compiles correctly after directory restructuring.
// Validates: Requirements 8.8, 8.9
package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// projectRoot returns the absolute path to the project root directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	// The test file is at tests/build/build_test.go, so project root is two levels up.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// TestImportPathsResolve verifies that all Go import paths resolve correctly
// after the directory restructuring. Running `go build ./...` must succeed.
func TestImportPathsResolve(t *testing.T) {
	root := projectRoot(t)

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... failed: %v\nOutput:\n%s", err, string(output))
	}
}

// TestMakeBuildProducesBinaries verifies that `make build` exits with code 0
// and produces the expected binaries: kiro-master, kiro-client, kiro-cli.
func TestMakeBuildProducesBinaries(t *testing.T) {
	root := projectRoot(t)

	// Clean build directory first to ensure fresh build.
	cmd := exec.Command("make", "clean")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make clean failed: %v\nOutput:\n%s", err, string(output))
	}

	// Run make build.
	cmd = exec.Command("make", "build")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make build failed (exit code non-zero): %v\nOutput:\n%s", err, string(output))
	}

	// Verify each binary exists and is executable.
	binaries := []string{"kiro-master", "kiro-client", "kiro-cli"}
	buildDir := filepath.Join(root, "build")

	for _, name := range binaries {
		binPath := filepath.Join(buildDir, name)

		info, err := os.Stat(binPath)
		if err != nil {
			t.Errorf("binary %s not found at %s: %v", name, binPath, err)
			continue
		}

		if info.IsDir() {
			t.Errorf("expected %s to be a file, got directory", name)
			continue
		}

		if info.Size() == 0 {
			t.Errorf("binary %s has zero size", name)
			continue
		}

		// Check executable permission (Unix only).
		if runtime.GOOS != "windows" {
			mode := info.Mode()
			if mode&0111 == 0 {
				t.Errorf("binary %s is not executable (mode: %v)", name, mode)
			}
		}
	}
}

// TestMakeBuildXDP verifies that `make build-xdp` compiles the XDP object
// and the resulting file is under 32KB (32768 bytes).
func TestMakeBuildXDP(t *testing.T) {
	root := projectRoot(t)

	// Check if clang is available; skip if not installed.
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not found in PATH; skipping XDP build test")
	}

	// Run make build-xdp.
	cmd := exec.Command("make", "build-xdp")
	cmd.Dir = root
	cmd.Env = append(os.Environ())

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make build-xdp failed (exit code non-zero): %v\nOutput:\n%s", err, string(output))
	}

	// Verify XDP object exists and is under 32KB.
	xdpPath := filepath.Join(root, "build", "xdp_filter.o")
	info, err := os.Stat(xdpPath)
	if err != nil {
		t.Fatalf("XDP object not found at %s: %v", xdpPath, err)
	}

	const maxSize int64 = 32768 // 32KB
	if info.Size() >= maxSize {
		t.Errorf("XDP object size %d bytes exceeds limit of %d bytes (32KB)", info.Size(), maxSize)
	}

	if info.Size() == 0 {
		t.Error("XDP object has zero size")
	}
}
