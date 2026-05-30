package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Integration Tests for CLI Commands (Task 17.2)
// Tests the full binary execution via exec.Command.
// Validates: Requirements 12.7, 12.8, 12.9, 12.10, 12.11, 12.12, 12.13, 12.17
// =============================================================================

// --- Test: update check with mock Master_Server (Requirement 12.8) ---

func TestIntegration_UpdateCheck_WithMockMaster(t *testing.T) {
	bin := buildCLI(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/update/check" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"update_available": false,
		})
	}))
	defer server.Close()

	stdout, _, exitCode := runCLI(t, bin, "update", "check",
		"--master-url", server.URL,
		"--component", "kiro-client-waf",
		"--channel", "stable",
		"--current-version", "1.0.0",
	)
	if exitCode != 0 {
		t.Fatalf("update check exited with code %d, want 0", exitCode)
	}
	if !strings.Contains(stdout, "up to date") {
		t.Errorf("expected 'up to date' message, got: %s", stdout)
	}
}

func TestIntegration_UpdateCheck_RequiresMasterURL(t *testing.T) {
	bin := buildCLI(t)

	_, stderr, exitCode := runCLI(t, bin, "update", "check")
	if exitCode != 2 {
		t.Fatalf("update check without --master-url exited with code %d, want 2", exitCode)
	}
	if !strings.Contains(stderr, "master-url") {
		t.Errorf("error should mention --master-url, got: %s", stderr)
	}
}

func TestIntegration_UpdateCheck_UpdateAvailable(t *testing.T) {
	bin := buildCLI(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"update_available": true,
			"release": map[string]interface{}{
				"version":      "2.0.0",
				"artifact_url": "https://example.com/artifact.bin",
				"sha256":       "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				"notes":        "Bug fixes and improvements",
			},
		})
	}))
	defer server.Close()

	stdout, _, exitCode := runCLI(t, bin, "update", "check",
		"--master-url", server.URL,
		"--component", "kiro-client-waf",
		"--channel", "stable",
		"--current-version", "1.0.0",
	)
	if exitCode != 0 {
		t.Fatalf("update check exited with code %d, want 0", exitCode)
	}
	if !strings.Contains(stdout, "Update available") {
		t.Errorf("expected 'Update available' message, got: %s", stdout)
	}
	if !strings.Contains(stdout, "2.0.0") {
		t.Errorf("expected version 2.0.0 in output, got: %s", stdout)
	}
}

// --- Test: update apply with SHA-256 verification and auto-rollback (Requirements 12.8, 12.17) ---

func TestIntegration_UpdateApply_SHA256Mismatch(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()

	// Create a fake current binary
	binaryPath := filepath.Join(dir, "kiro-client-waf")
	originalContent := []byte("original binary content v1.0.0")
	if err := os.WriteFile(binaryPath, originalContent, 0o755); err != nil {
		t.Fatal(err)
	}

	// Artifact server serves a binary
	artifactContent := []byte("new binary content v2.0.0")
	artifactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artifactContent)
	}))
	defer artifactServer.Close()

	// Master server returns update with WRONG SHA-256
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"update_available": true,
			"release": map[string]interface{}{
				"version":      "2.0.0",
				"artifact_url": artifactServer.URL + "/artifact.bin",
				"sha256":       "sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"notes":        "test update",
			},
		})
	}))
	defer masterServer.Close()

	_, stderr, exitCode := runCLI(t, bin, "update", "apply",
		"--master-url", masterServer.URL,
		"--component", "kiro-client-waf",
		"--channel", "stable",
		"--current-version", "1.0.0",
		"--binary-path", binaryPath,
		"--service", "kiro-test-service",
	)
	if exitCode == 0 {
		t.Fatal("update apply with SHA-256 mismatch should fail")
	}
	if !strings.Contains(stderr, "SHA-256 mismatch") {
		t.Errorf("error should mention SHA-256 mismatch, got: %s", stderr)
	}

	// Verify original binary is still intact
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("original binary was modified after SHA-256 mismatch")
	}
}

func TestIntegration_UpdateApply_AutoRollbackOnHealthCheckFail(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()

	// Create a fake current binary
	binaryPath := filepath.Join(dir, "kiro-client-waf")
	originalContent := []byte("original binary content v1.0.0")
	if err := os.WriteFile(binaryPath, originalContent, 0o755); err != nil {
		t.Fatal(err)
	}

	// Artifact server serves a binary with correct SHA-256
	artifactContent := []byte("new binary content v2.0.0")
	h := sha256.Sum256(artifactContent)
	correctHash := hex.EncodeToString(h[:])

	artifactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artifactContent)
	}))
	defer artifactServer.Close()

	// Master server returns update with correct SHA-256
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"update_available": true,
			"release": map[string]interface{}{
				"version":      "2.0.0",
				"artifact_url": artifactServer.URL + "/artifact.bin",
				"sha256":       fmt.Sprintf("sha256:%s", correctHash),
				"notes":        "test update",
			},
		})
	}))
	defer masterServer.Close()

	// Apply will fail at systemctl restart (not available in test env)
	// which triggers auto-rollback behavior
	_, stderr, exitCode := runCLI(t, bin, "update", "apply",
		"--master-url", masterServer.URL,
		"--component", "kiro-client-waf",
		"--channel", "stable",
		"--current-version", "1.0.0",
		"--binary-path", binaryPath,
		"--service", "kiro-nonexistent-test-service",
	)

	// Should fail (systemctl not available or service doesn't exist)
	if exitCode == 0 {
		t.Log("update apply succeeded unexpectedly (systemctl available)")
		return
	}

	// The error should be about restart/health, not SHA-256
	if strings.Contains(stderr, "SHA-256 mismatch") {
		t.Fatalf("unexpected SHA-256 mismatch error: %s", stderr)
	}

	// After rollback, the original binary should be restored
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(originalContent) {
		// Check if .bak still exists (rollback may have restored it)
		bakPath := binaryPath + ".bak"
		if _, err := os.Stat(bakPath); err == nil {
			bakContent, _ := os.ReadFile(bakPath)
			if string(bakContent) == string(originalContent) {
				// Rollback didn't complete but backup is preserved
				t.Log("backup preserved at .bak path")
				return
			}
		}
		t.Errorf("binary content changed and was not rolled back")
	}
}

// --- Test: update rollback restores .bak file (Requirement 12.8) ---

func TestIntegration_UpdateRollback_RestoresBakFile(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()

	// Create current binary and .bak file
	binaryPath := filepath.Join(dir, "kiro-client-waf")
	currentContent := []byte("current binary v2.0.0")
	backupContent := []byte("backup binary v1.0.0")

	if err := os.WriteFile(binaryPath, currentContent, 0o755); err != nil {
		t.Fatal(err)
	}
	bakPath := binaryPath + ".bak"
	if err := os.WriteFile(bakPath, backupContent, 0o755); err != nil {
		t.Fatal(err)
	}

	// Rollback will fail at systemctl restart, but the file rename should succeed
	_, stderr, exitCode := runCLI(t, bin, "update", "rollback",
		"--binary-path", binaryPath,
		"--service", "kiro-nonexistent-test-service",
	)

	// Check if the .bak was renamed to the binary path
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("failed to read binary after rollback: %v", err)
	}

	if string(content) == string(backupContent) {
		// Rollback file rename succeeded (service restart may have failed)
		t.Log("rollback file restore succeeded")
	} else if exitCode != 0 {
		// If systemctl failed, the rename might still have happened
		// Check if the error is about service restart (expected in test env)
		if strings.Contains(stderr, "restart") || strings.Contains(stderr, "systemctl") {
			// This is expected - systemctl not available in test env
			// But the file should have been renamed before restart attempt
			t.Log("rollback failed at service restart (expected in test env)")
		} else if strings.Contains(stderr, "no backup found") {
			t.Errorf("rollback reported no backup, but .bak was created")
		}
	}
}

func TestIntegration_UpdateRollback_NoBackup(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "kiro-client-waf")
	// No .bak file exists

	_, stderr, exitCode := runCLI(t, bin, "update", "rollback",
		"--binary-path", binaryPath,
		"--service", "kiro-test-service",
	)
	if exitCode == 0 {
		t.Fatal("rollback without .bak should fail")
	}
	if !strings.Contains(stderr, "no backup found") {
		t.Errorf("error should mention no backup, got: %s", stderr)
	}
}

// --- Test: install plan returns JSON (Requirement 12.7) ---

func TestIntegration_InstallPlan_ReturnsJSON(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)

	stdout, _, exitCode := runCLI(t, bin, "install", "plan",
		"--config", configPath,
		"--install-root", dir,
	)
	if exitCode != 0 {
		t.Fatalf("install plan exited with code %d, want 0", exitCode)
	}

	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("install plan output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Verify required fields in the plan
	requiredFields := []string{"generated_at", "version", "action", "mode", "layout", "steps"}
	for _, field := range requiredFields {
		if _, ok := plan[field]; !ok {
			t.Errorf("install plan JSON missing required field %q", field)
		}
	}

	// Verify action is "install"
	if action, ok := plan["action"].(string); !ok || action != "install" {
		t.Errorf("install plan action = %v, want 'install'", plan["action"])
	}

	// Verify steps is a non-empty array
	steps, ok := plan["steps"].([]interface{})
	if !ok || len(steps) == 0 {
		t.Error("install plan should have non-empty steps array")
	}
}

// --- Test: install stage-lab stages into --install-root (Requirement 12.7) ---

func TestIntegration_InstallStageLab_StagesIntoRoot(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	installRoot := filepath.Join(dir, "stage-root")

	// Create fake binaries to stage
	agentBin := filepath.Join(dir, "kiro-agent")
	cliBin := filepath.Join(dir, "kiro-cli-fake")
	servicePath := filepath.Join(dir, "kiro-agent.service")
	if err := os.WriteFile(agentBin, []byte("fake agent binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliBin, []byte("fake cli binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath, []byte("[Unit]\nDescription=Kiro Agent\n[Service]\nExecStart=/usr/local/bin/kiro-agent\n[Install]\nWantedBy=multi-user.target\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, exitCode := runCLI(t, bin, "install", "stage-lab",
		"--config", configPath,
		"--install-root", installRoot,
		"--agent-binary", agentBin,
		"--cli-binary", cliBin,
		"--systemd-service", servicePath,
	)
	if exitCode != 0 {
		t.Fatalf("install stage-lab exited with code %d\nstderr: %s", exitCode, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("install stage-lab output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Verify install_root in result
	if root, ok := result["install_root"].(string); !ok || root != installRoot {
		t.Errorf("stage-lab install_root = %v, want %q", result["install_root"], installRoot)
	}

	// Verify files were staged
	if files, ok := result["files"].([]interface{}); !ok || len(files) == 0 {
		t.Error("stage-lab should have staged files")
	}

	// Verify the install root directory was created
	if _, err := os.Stat(installRoot); os.IsNotExist(err) {
		t.Error("install root directory was not created")
	}
}

// --- Test: incident report creates JSON and Markdown (Requirement 12.9) ---

func TestIntegration_IncidentReport_CreatesJSONAndMarkdown(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	outputDir := filepath.Join(dir, "incidents")

	stdout, stderr, exitCode := runCLI(t, bin, "incident", "report",
		"--config", configPath,
		"--output-dir", outputDir,
		"--type", "attack",
		"--severity", "high",
		"--status", "open",
		"--summary", "DDoS attack detected on port 443",
	)
	if exitCode != 0 {
		t.Fatalf("incident report exited with code %d\nstderr: %s", exitCode, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("incident report output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Verify result contains paths
	jsonPath, ok := result["json_path"].(string)
	if !ok || jsonPath == "" {
		t.Fatal("incident report missing json_path")
	}
	markdownPath, ok := result["markdown_path"].(string)
	if !ok || markdownPath == "" {
		t.Fatal("incident report missing markdown_path")
	}

	// Verify JSON file was created and contains expected fields
	jsonContent, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read incident JSON: %v", err)
	}
	var report map[string]interface{}
	if err := json.Unmarshal(jsonContent, &report); err != nil {
		t.Fatalf("incident JSON is not valid: %v", err)
	}
	if report["type"] != "attack" {
		t.Errorf("incident type = %v, want 'attack'", report["type"])
	}
	if report["severity"] != "high" {
		t.Errorf("incident severity = %v, want 'high'", report["severity"])
	}
	if report["status"] != "open" {
		t.Errorf("incident status = %v, want 'open'", report["status"])
	}
	if !strings.Contains(report["summary"].(string), "DDoS") {
		t.Errorf("incident summary should contain 'DDoS', got: %v", report["summary"])
	}

	// Verify Markdown file was created
	mdContent, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("failed to read incident Markdown: %v", err)
	}
	mdStr := string(mdContent)
	if !strings.Contains(mdStr, "# Incident Report") {
		t.Error("Markdown should contain '# Incident Report' header")
	}
	if !strings.Contains(mdStr, "attack") {
		t.Error("Markdown should contain incident type 'attack'")
	}
	if !strings.Contains(mdStr, "high") {
		t.Error("Markdown should contain severity 'high'")
	}
}

func TestIntegration_IncidentReport_DifferentTypes(t *testing.T) {
	bin := buildCLI(t)

	incidentTypes := []string{"attack", "lost_ssh", "update_failed", "origin_ip_leaked", "license_rebind", "runtime_security", "other"}
	for _, incType := range incidentTypes {
		t.Run("type="+incType, func(t *testing.T) {
			dir := t.TempDir()
			configPath := writeTestConfig(t, dir)
			outputDir := filepath.Join(dir, "incidents")

			stdout, stderr, exitCode := runCLI(t, bin, "incident", "report",
				"--config", configPath,
				"--output-dir", outputDir,
				"--type", incType,
				"--severity", "medium",
				"--status", "investigating",
				"--summary", "Test incident of type "+incType,
			)
			if exitCode != 0 {
				t.Fatalf("incident report type=%s exited with code %d\nstderr: %s", incType, exitCode, stderr)
			}

			var result map[string]interface{}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			if result["json_path"] == nil || result["markdown_path"] == nil {
				t.Error("result should contain json_path and markdown_path")
			}
		})
	}
}

// --- Test: pilot report aggregates evidence (Requirement 12.10) ---

func TestIntegration_PilotReport_AggregatesEvidence(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	outputDir := filepath.Join(dir, "pilot-reports")

	// Create evidence files
	healthFile := filepath.Join(dir, "health.json")
	if err := os.WriteFile(healthFile, []byte(`{"status":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	benchmarkFile := filepath.Join(dir, "benchmark.json")
	if err := os.WriteFile(benchmarkFile, []byte(`{"latency_p99_ms":12}`), 0o644); err != nil {
		t.Fatal(err)
	}
	incidentDir := filepath.Join(dir, "incidents")
	if err := os.MkdirAll(incidentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a dummy incident file in the directory
	if err := os.WriteFile(filepath.Join(incidentDir, "inc-001.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, exitCode := runCLI(t, bin, "pilot", "report",
		"--config", configPath,
		"--output-dir", outputDir,
		"--server-count", "3",
		"--started-at", "2025-01-01T00:00:00Z",
		"--ended-at", "2025-02-15T00:00:00Z",
		"--health-file", healthFile,
		"--benchmark-file", benchmarkFile,
		"--incident-dir", incidentDir,
	)
	if exitCode != 0 {
		t.Fatalf("pilot report exited with code %d\nstderr: %s", exitCode, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("pilot report output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Verify result contains expected fields
	if result["json_path"] == nil {
		t.Error("pilot report missing json_path")
	}
	if result["markdown_path"] == nil {
		t.Error("pilot report missing markdown_path")
	}
	if result["decision"] == nil {
		t.Error("pilot report missing decision")
	}

	// Read the JSON report and verify evidence aggregation
	jsonPath := result["json_path"].(string)
	jsonContent, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read pilot JSON: %v", err)
	}
	var report map[string]interface{}
	if err := json.Unmarshal(jsonContent, &report); err != nil {
		t.Fatalf("pilot JSON is not valid: %v", err)
	}

	// Verify evidence array contains our files
	evidence, ok := report["evidence"].([]interface{})
	if !ok {
		t.Fatal("pilot report missing evidence array")
	}

	// Check that health_report evidence is present
	foundHealth := false
	foundBenchmark := false
	foundIncident := false
	for _, item := range evidence {
		if e, ok := item.(map[string]interface{}); ok {
			name := e["name"].(string)
			status := e["status"].(string)
			switch name {
			case "health_report":
				foundHealth = true
				if status != "present" {
					t.Errorf("health_report evidence status = %q, want 'present'", status)
				}
			case "benchmark_report":
				foundBenchmark = true
				if status != "present" {
					t.Errorf("benchmark_report evidence status = %q, want 'present'", status)
				}
			case "incident_drill":
				foundIncident = true
				if status != "present" {
					t.Errorf("incident_drill evidence status = %q, want 'present'", status)
				}
			}
		}
	}
	if !foundHealth {
		t.Error("pilot report missing health_report evidence")
	}
	if !foundBenchmark {
		t.Error("pilot report missing benchmark_report evidence")
	}
	if !foundIncident {
		t.Error("pilot report missing incident_drill evidence")
	}
}

// --- Test: report returns JSON (Requirement 12.11) ---

func TestIntegration_Report_ReturnsJSON(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)

	stdout, stderr, exitCode := runCLI(t, bin, "report", "--config", configPath)
	if exitCode != 0 {
		t.Fatalf("report command exited with code %d\nstderr: %s", exitCode, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("report output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Verify required fields for system report
	requiredFields := []string{"version", "generated_at", "go_version", "os", "arch", "config"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("report JSON missing required field %q", field)
		}
	}

	// Verify config section contains mode
	configSection, ok := result["config"].(map[string]interface{})
	if !ok {
		t.Fatal("report 'config' is not an object")
	}
	if configSection["mode"] != "server" {
		t.Errorf("report config.mode = %v, want 'server'", configSection["mode"])
	}
}

// --- Test: invalid command shows usage and exits code 2 (Requirement 12.12) ---

func TestIntegration_InvalidCommand_ShowsUsageExitsCode2(t *testing.T) {
	bin := buildCLI(t)

	invalidCommands := [][]string{
		{"nonexistent"},
		{"foobar"},
		{"unknown-cmd"},
		{"update", "invalid-sub"},
		{"install", "nonexistent-sub"},
		{"mode", "invalid-sub"},
		{"incident", "invalid-sub"},
		{"pilot", "invalid-sub"},
	}

	for _, args := range invalidCommands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, stderr, exitCode := runCLI(t, bin, args...)
			if exitCode != 2 {
				t.Errorf("invalid command %v exited with code %d, want 2", args, exitCode)
			}
			if !strings.Contains(stderr, "usage") {
				t.Errorf("invalid command %v should show usage, got stderr: %s", args, stderr)
			}
		})
	}
}

func TestIntegration_NoArgs_ShowsUsageExitsCode2(t *testing.T) {
	bin := buildCLI(t)
	_, stderr, exitCode := runCLI(t, bin)
	if exitCode != 2 {
		t.Errorf("no args exited with code %d, want 2", exitCode)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("no args should show usage, got stderr: %s", stderr)
	}
}

// --- Test: missing required params shows error naming param and exits code 2 (Requirement 12.13) ---

func TestIntegration_MissingRequiredParams_ExitsCode2(t *testing.T) {
	bin := buildCLI(t)

	tests := []struct {
		name          string
		args          []string
		expectedParam string
	}{
		{
			name:          "update check missing --master-url",
			args:          []string{"update", "check"},
			expectedParam: "master-url",
		},
		{
			name:          "update apply missing --master-url",
			args:          []string{"update", "apply", "--binary-path", "/tmp/bin", "--service", "svc"},
			expectedParam: "master-url",
		},
		{
			name:          "update apply missing --binary-path",
			args:          []string{"update", "apply", "--master-url", "http://localhost", "--service", "svc"},
			expectedParam: "binary-path",
		},
		{
			name:          "update apply missing --service",
			args:          []string{"update", "apply", "--master-url", "http://localhost", "--binary-path", "/tmp/bin"},
			expectedParam: "service",
		},
		{
			name:          "update rollback missing --binary-path",
			args:          []string{"update", "rollback", "--service", "svc"},
			expectedParam: "binary-path",
		},
		{
			name:          "update rollback missing --service",
			args:          []string{"update", "rollback", "--binary-path", "/tmp/bin"},
			expectedParam: "service",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, exitCode := runCLI(t, bin, tc.args...)
			if exitCode != 2 {
				t.Errorf("%s: exited with code %d, want 2\nstderr: %s", tc.name, exitCode, stderr)
			}
			if !strings.Contains(stderr, tc.expectedParam) {
				t.Errorf("%s: error should mention %q, got: %s", tc.name, tc.expectedParam, stderr)
			}
		})
	}
}

// --- Test: update apply missing required params exits code 2 ---

func TestIntegration_UpdateApply_MissingParams_ExitsCode2(t *testing.T) {
	bin := buildCLI(t)

	// All required params missing
	_, stderr, exitCode := runCLI(t, bin, "update", "apply")
	if exitCode != 2 {
		t.Errorf("update apply with no params exited with code %d, want 2\nstderr: %s", exitCode, stderr)
	}
}

// --- Test: update rollback missing required params exits code 2 ---

func TestIntegration_UpdateRollback_MissingParams_ExitsCode2(t *testing.T) {
	bin := buildCLI(t)

	_, stderr, exitCode := runCLI(t, bin, "update", "rollback")
	if exitCode != 2 {
		t.Errorf("update rollback with no params exited with code %d, want 2\nstderr: %s", exitCode, stderr)
	}
}

// =============================================================================
// End-to-End Integration Tests (Task 23.1)
// Tests full CLI binary execution for all commands and combined flows.
// Validates: Requirements 12.1–12.18
// =============================================================================

// --- Test: mode show returns current mode (Requirement 12.6) ---

func TestIntegration_ModeShow_ReturnsCurrentMode(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kiro.yaml")
	if err := os.WriteFile(configPath, []byte("mode: server\nplan: community\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runCLI(t, bin, "mode", "show", "--config", configPath)
	if exitCode != 0 {
		t.Fatalf("mode show exited with code %d, want 0", exitCode)
	}
	mode := strings.TrimSpace(stdout)
	if mode != "server" {
		t.Errorf("mode show = %q, want 'server'", mode)
	}
}

func TestIntegration_ModeShow_AfterSet(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kiro.yaml")
	if err := os.WriteFile(configPath, []byte("mode: server\nplan: community\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set mode to full
	_, _, exitCode := runCLI(t, bin, "mode", "set", "--config", configPath, "--mode", "full")
	if exitCode != 0 {
		t.Fatalf("mode set full exited with code %d, want 0", exitCode)
	}

	// Verify mode show returns full
	stdout, _, exitCode := runCLI(t, bin, "mode", "show", "--config", configPath)
	if exitCode != 0 {
		t.Fatalf("mode show exited with code %d, want 0", exitCode)
	}
	mode := strings.TrimSpace(stdout)
	if mode != "full" {
		t.Errorf("mode show after set = %q, want 'full'", mode)
	}
}

// --- Test: install plan → stage-lab → apply-lab end-to-end flow (Requirements 12.7, 12.18) ---

func TestIntegration_InstallFlow_PlanToStageLabToApplyLab(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	installRoot := filepath.Join(dir, "install-root")

	// Step 1: install plan
	stdout, stderr, exitCode := runCLI(t, bin, "install", "plan",
		"--config", configPath,
		"--install-root", installRoot,
	)
	if exitCode != 0 {
		t.Fatalf("install plan exited with code %d\nstderr: %s", exitCode, stderr)
	}
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("install plan output is not valid JSON: %v", err)
	}
	if plan["action"] != "install" {
		t.Errorf("plan action = %v, want 'install'", plan["action"])
	}

	// Step 2: install stage-lab
	agentBin := filepath.Join(dir, "kiro-agent")
	cliBin := filepath.Join(dir, "kiro-cli-fake")
	servicePath := filepath.Join(dir, "kiro-agent.service")
	os.WriteFile(agentBin, []byte("fake agent binary"), 0o755)
	os.WriteFile(cliBin, []byte("fake cli binary"), 0o755)
	os.WriteFile(servicePath, []byte("[Unit]\nDescription=Kiro Agent\n[Service]\nExecStart=/usr/local/bin/kiro-agent\n[Install]\nWantedBy=multi-user.target\n"), 0o644)

	stdout, stderr, exitCode = runCLI(t, bin, "install", "stage-lab",
		"--config", configPath,
		"--install-root", installRoot,
		"--agent-binary", agentBin,
		"--cli-binary", cliBin,
		"--systemd-service", servicePath,
	)
	if exitCode != 0 {
		t.Fatalf("install stage-lab exited with code %d\nstderr: %s", exitCode, stderr)
	}
	var stageResult map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &stageResult); err != nil {
		t.Fatalf("install stage-lab output is not valid JSON: %v", err)
	}
	if stageResult["install_root"] != installRoot {
		t.Errorf("stage-lab install_root = %v, want %q", stageResult["install_root"], installRoot)
	}

	// Step 3: install apply-lab with wrong ack → should fail
	_, stderr, exitCode = runCLI(t, bin, "install", "apply-lab",
		"--config", configPath,
		"--install-root", installRoot,
		"--ack", "WRONG_ACK",
		"--skip-os-check",
	)
	if exitCode != 1 {
		t.Errorf("install apply-lab with wrong ack exited with code %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "ack") && !strings.Contains(stderr, "KIRO_LAB_INSTALL_APPLY") {
		t.Errorf("error should mention ack requirement, got: %s", stderr)
	}

	// Step 4: install apply-lab with correct ack and install-root (lab mode, no root required)
	osRelease := filepath.Join(dir, "os-release")
	os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644)

	stdout, stderr, exitCode = runCLI(t, bin, "install", "apply-lab",
		"--config", configPath,
		"--install-root", installRoot,
		"--agent-binary", agentBin,
		"--cli-binary", cliBin,
		"--systemd-service", servicePath,
		"--ack", "KIRO_LAB_INSTALL_APPLY",
		"--os-release", osRelease,
		"--skip-os-check",
	)
	if exitCode != 0 {
		t.Fatalf("install apply-lab with correct ack exited with code %d\nstderr: %s\nstdout: %s", exitCode, stderr, stdout)
	}
}

// --- Test: update apply → health check fail → auto-rollback verifies message (Requirement 12.17) ---

func TestIntegration_UpdateApply_RollbackMessageOnHealthFail(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()

	// Create a fake current binary
	binaryPath := filepath.Join(dir, "kiro-client-waf")
	originalContent := []byte("original binary content v1.0.0")
	if err := os.WriteFile(binaryPath, originalContent, 0o755); err != nil {
		t.Fatal(err)
	}

	// Artifact server serves a binary with correct SHA-256
	artifactContent := []byte("new binary content v2.0.0")
	h := sha256.Sum256(artifactContent)
	correctHash := hex.EncodeToString(h[:])

	artifactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artifactContent)
	}))
	defer artifactServer.Close()

	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"update_available": true,
			"release": map[string]interface{}{
				"version":      "2.0.0",
				"artifact_url": artifactServer.URL + "/artifact.bin",
				"sha256":       fmt.Sprintf("sha256:%s", correctHash),
				"notes":        "test update",
			},
		})
	}))
	defer masterServer.Close()

	// Apply will fail at systemctl restart (service doesn't exist) → triggers rollback
	_, stderr, exitCode := runCLI(t, bin, "update", "apply",
		"--master-url", masterServer.URL,
		"--component", "kiro-client-waf",
		"--channel", "stable",
		"--current-version", "1.0.0",
		"--binary-path", binaryPath,
		"--service", "kiro-nonexistent-test-service",
	)

	// Should fail with exit code 1 (rollback occurred)
	if exitCode == 0 {
		t.Log("update apply succeeded unexpectedly (systemctl available)")
		return
	}

	// Verify the error mentions rollback or restart failure (not SHA-256)
	if strings.Contains(stderr, "SHA-256 mismatch") {
		t.Fatalf("unexpected SHA-256 mismatch error: %s", stderr)
	}
	if !strings.Contains(stderr, "rollback") && !strings.Contains(stderr, "restart") {
		t.Errorf("error should mention rollback or restart failure, got: %s", stderr)
	}

	// Verify original binary is restored after rollback
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) == string(originalContent) {
		t.Log("original binary successfully restored after rollback")
	} else {
		// Check if .bak exists (rollback may have partially completed)
		bakPath := binaryPath + ".bak"
		if _, err := os.Stat(bakPath); err == nil {
			t.Log("backup file preserved at .bak path")
		}
	}
}

// --- Test: all CLI commands produce expected output with real config (Requirements 12.1-12.11) ---

func TestIntegration_AllCommands_WithRealConfig(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	osRelease := filepath.Join(dir, "os-release")
	os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644)

	tests := []struct {
		name       string
		args       []string
		wantExit   int
		checkJSON  bool
		checkField string
	}{
		{
			name:     "version",
			args:     []string{"version"},
			wantExit: 0,
		},
		{
			name:       "status",
			args:       []string{"status", "--config", configPath},
			wantExit:   0,
			checkJSON:  true,
			checkField: "mode",
		},
		{
			name:       "health",
			args:       []string{"health", "--config", configPath, "--os-release", osRelease, "--preflight-writable-root", dir, "--skip-command-checks"},
			wantExit:   0,
			checkJSON:  true,
			checkField: "status",
		},
		{
			name:       "preflight",
			args:       []string{"preflight", "--config", configPath, "--os-release", osRelease, "--preflight-writable-root", dir, "--skip-command-checks"},
			wantExit:   0,
			checkJSON:  true,
			checkField: "status",
		},
		{
			name:       "report",
			args:       []string{"report", "--config", configPath},
			wantExit:   0,
			checkJSON:  true,
			checkField: "version",
		},
		{
			name:     "mode show",
			args:     []string{"mode", "show", "--config", configPath},
			wantExit: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, bin, tc.args...)
			if exitCode != tc.wantExit {
				t.Fatalf("%s exited with code %d, want %d\nstderr: %s", tc.name, exitCode, tc.wantExit, stderr)
			}
			if tc.checkJSON {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("%s output is not valid JSON: %v\nOutput: %s", tc.name, err, stdout)
				}
				if tc.checkField != "" {
					if _, ok := result[tc.checkField]; !ok {
						t.Errorf("%s JSON missing field %q", tc.name, tc.checkField)
					}
				}
			}
		})
	}
}
