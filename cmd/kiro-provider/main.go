package main

import (
	"flag"
	"fmt"
	"os"

	"kiro_waf/internal/provider"
	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/licenseverify"
)

func main() {
	configPath := flag.String("config", "configs/provider.example.yaml", "path to provider config")
	check := flag.Bool("check", false, "validate provider config and exit")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.Version)
		return
	}
	if *check {
		res, err := config.CheckFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "provider config check failed: %v\n", err)
			os.Exit(1)
		}
		if res.Kind != config.KindProvider {
			fmt.Fprintf(os.Stderr, "provider config check failed: got %s config\n", res.Kind)
			os.Exit(1)
		}
		fmt.Printf("provider config ok: path=%s\n", res.Path)
		return
	}
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "gen-dev-keys":
			runGenDevKeys(*configPath, args[1:])
			return
		case "issue-test-license":
			runIssueTestLicense(*configPath, args[1:])
			return
		}
	}
	fmt.Fprintln(os.Stderr, "usage: kiro-provider --check | --version | gen-dev-keys | issue-test-license")
	os.Exit(2)
}

func runGenDevKeys(configPath string, args []string) {
	cmd := flag.NewFlagSet("kiro-provider gen-dev-keys", flag.ExitOnError)
	force := cmd.Bool("force", false, "overwrite existing provider key pair")
	if err := cmd.Parse(args); err != nil {
		os.Exit(2)
	}
	cfg := mustLoadProviderConfig(configPath)
	publicKey, err := provider.EnsureKeyPair(cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate provider keys failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("provider keys ok: private=%s public=%s public_key=%s\n",
		cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, licenseverify.EncodePublicKey(publicKey))
}

func runIssueTestLicense(configPath string, args []string) {
	cmd := flag.NewFlagSet("kiro-provider issue-test-license", flag.ExitOnError)
	licenseID := cmd.String("license-id", "lic_dev_000001", "license id")
	customerID := cmd.String("customer-id", "cus_dev_000001", "customer id")
	serverID := cmd.String("server-id", "srv_dev_000001", "server id")
	plan := cmd.String("plan", "school_smb", "license plan")
	fingerprintHash := cmd.String("fingerprint-hash", "", "server fingerprint hash")
	machineIDHash := cmd.String("machine-id-hash", "", "optional machine-id hash")
	primaryMACHash := cmd.String("primary-mac-hash", "", "optional primary MAC hash")
	validDays := cmd.Int("valid-days", 365, "license validity in days")
	updateChannel := cmd.String("update-channel", "stable", "update channel")
	policyVersion := cmd.String("policy-version", "dev", "policy bundle version")
	agentOutDir := cmd.String("agent-out-dir", "", "optional directory to export license.json and provider-public-key.pem for agent")
	if err := cmd.Parse(args); err != nil {
		os.Exit(2)
	}
	cfg := mustLoadProviderConfig(configPath)
	if _, err := provider.EnsureKeyPair(cfg.Provider.SigningKeyFile, cfg.Provider.PublicKeyFile, false); err != nil {
		fmt.Fprintf(os.Stderr, "provider key check failed: %v\n", err)
		os.Exit(1)
	}
	result, err := provider.IssueLicense(cfg, provider.IssueRequest{
		LicenseID:           *licenseID,
		CustomerID:          *customerID,
		ServerID:            *serverID,
		Plan:                *plan,
		MachineIDHash:       *machineIDHash,
		PrimaryMACHash:      *primaryMACHash,
		FingerprintHash:     *fingerprintHash,
		ValidDays:           *validDays,
		UpdateChannel:       *updateChannel,
		PolicyBundleVersion: *policyVersion,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "issue license failed: %v\n", err)
		os.Exit(1)
	}
	if *agentOutDir != "" {
		if err := provider.ExportAgentFiles(*agentOutDir, result.License, cfg.Provider.PublicKeyFile); err != nil {
			fmt.Fprintf(os.Stderr, "export agent files failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("license issued: license_id=%s path=%s agent_out=%s\n",
		result.License.Payload.LicenseID, result.LicensePath, *agentOutDir)
}

func mustLoadProviderConfig(path string) config.ProviderConfig {
	cfg, err := config.LoadProviderFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider config load failed: %v\n", err)
		os.Exit(1)
	}
	return cfg
}
