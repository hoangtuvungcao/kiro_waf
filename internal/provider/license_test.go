package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/licenseverify"
	"kiro_waf/internal/shared/storage"
)

func TestProviderIssuesLicenseAgentCanVerify(t *testing.T) {
	cfg := testProviderConfig(t.TempDir())
	publicKey, err := EnsureKeyPair(cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, false)
	if err != nil {
		t.Fatalf("ensure key pair: %v", err)
	}
	if len(publicKey) == 0 {
		t.Fatal("expected public key")
	}

	fingerprintHash := "sha256:test-fingerprint"
	result, err := IssueLicense(cfg, IssueRequest{
		LicenseID:           "lic_test_000001",
		CustomerID:          "cus_test_000001",
		ServerID:            "srv_test_000001",
		Plan:                "school_smb",
		MachineIDHash:       licenseverify.HashValue("machine-id"),
		PrimaryMACHash:      licenseverify.HashValue("00:11:22:33:44:55"),
		FingerprintHash:     fingerprintHash,
		ValidDays:           365,
		UpdateChannel:       "stable",
		PolicyBundleVersion: "2026.05.1",
		Now:                 time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("issue license: %v", err)
	}

	agentDir := filepath.Join(filepath.Dir(cfg.Storage.RootDir), "agent-fixture")
	if err := ExportAgentFiles(agentDir, result.License, cfg.Provider.PublicKeyFile); err != nil {
		t.Fatalf("export agent files: %v", err)
	}

	res, err := licenseverify.VerifyFile(
		filepath.Join(agentDir, "license.json"),
		filepath.Join(agentDir, "provider-public-key.pem"),
		licenseverify.Options{
			RequiredMode:           "full",
			RequiredFeatures:       []string{"waf", "bot_defense"},
			MachineFingerprintHash: fingerprintHash,
			Now:                    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatalf("agent verify provider license: %v", err)
	}
	if !res.Valid || res.LicenseID != "lic_test_000001" {
		t.Fatalf("unexpected verify result: %#v", res)
	}

	count, err := storage.CountJSONLLines(filepath.Join(cfg.Storage.RootDir, "activations", "2026-05.jsonl"))
	if err != nil {
		t.Fatalf("count activations: %v", err)
	}
	if count != 1 {
		t.Fatalf("activation records = %d, want 1", count)
	}
	assertAgentFixtureHasNoPrivateKey(t, agentDir)
}

func TestIssueLicenseRejectsUnknownPlan(t *testing.T) {
	cfg := testProviderConfig(t.TempDir())
	if _, err := EnsureKeyPair(cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, false); err != nil {
		t.Fatalf("ensure key pair: %v", err)
	}
	if _, err := IssueLicense(cfg, IssueRequest{
		LicenseID:       "lic_test",
		CustomerID:      "cus_test",
		ServerID:        "srv_test",
		Plan:            "missing",
		FingerprintHash: "sha256:test",
	}); err == nil {
		t.Fatal("expected unknown plan rejection")
	}
}

func TestEnsureKeyPairReusesExistingKey(t *testing.T) {
	cfg := testProviderConfig(t.TempDir())
	first, err := EnsureKeyPair(cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, false)
	if err != nil {
		t.Fatalf("ensure first key: %v", err)
	}
	second, err := EnsureKeyPair(cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, false)
	if err != nil {
		t.Fatalf("ensure second key: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("expected existing key pair to be reused")
	}
}

func testProviderConfig(dir string) config.ProviderConfig {
	return config.ProviderConfig{
		Provider: config.ProviderSection{
			Name:           "Test Provider",
			NodeRole:       "provider_license_server",
			SigningKeyFile: filepath.Join(dir, "keys", "ed25519-private.key"),
			PublicKeyFile:  filepath.Join(dir, "keys", "ed25519-public.key"),
		},
		Storage: config.StorageSection{
			Driver:  "file",
			RootDir: filepath.Join(dir, "provider-data"),
		},
		Licenses: config.LicenseSection{
			DefaultGraceDays: 7,
			Plans: map[string]config.LicensePlan{
				"school_smb": {
					AllowedModes: []string{"server", "full"},
					Features:     []string{"xdp", "nftables", "waf", "bot_defense"},
				},
			},
		},
	}
}

func assertAgentFixtureHasNoPrivateKey(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.Contains(name, "private") || strings.Contains(name, "signing") {
			t.Fatalf("agent fixture must not contain private key material: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk agent fixture: %v", err)
	}
}
