package tests

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAgentDoesNotImportProvider(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./cmd/kiro-agent")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list agent deps: %v\n%s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.Contains(dep, "/internal/provider") {
			t.Fatalf("kiro-agent must not import provider package, found %s", dep)
		}
	}
}

func TestProviderDoesNotImportAgentRuntime(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./cmd/kiro-provider")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list provider deps: %v\n%s", err, out)
	}
	blocked := []string{
		"/internal/agent/firewall",
		"/internal/agent/ebpf",
		"/internal/agent/proxy",
	}
	for _, dep := range strings.Fields(string(out)) {
		for _, prefix := range blocked {
			if strings.Contains(dep, prefix) {
				t.Fatalf("kiro-provider must not import agent runtime package, found %s", dep)
			}
		}
	}
}
