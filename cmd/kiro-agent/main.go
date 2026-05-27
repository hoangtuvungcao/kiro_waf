package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/licenseverify"
	"kiro_waf/internal/shared/machinefingerprint"
)

func main() {
	configPath := flag.String("config", "configs/kiro.example.yaml", "path to kiro config")
	licensePath := flag.String("license-file", "", "override path to signed license file")
	publicKeyPath := flag.String("provider-public-key", "", "override path to provider public key")
	machineFingerprint := flag.String("machine-fingerprint", "", "override sha256 fingerprint hash for binding check")
	skipLicenseCheck := flag.Bool("skip-license-check", false, "validate config without reading license files")
	check := flag.Bool("check", false, "validate config and exit")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.Version)
		return
	}
	if *check {
		res, err := config.CheckFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "config check failed: %v\n", err)
			os.Exit(1)
		}
		runtime, err := config.LoadRuntimeFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "runtime expansion failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("config ok: kind=%s mode=%s plan=%s sites=%d backend_pools=%d path=%s\n",
			res.Kind, runtime.Mode, runtime.Plan, len(runtime.Sites), len(runtime.BackendPools), res.Path)
		effectiveLicensePath := firstNonEmpty(*licensePath, runtime.License.File)
		effectivePublicKeyPath := firstNonEmpty(*publicKeyPath, runtime.License.ProviderPublicKey)
		shouldVerifyLicense := !*skipLicenseCheck && (runtime.License.RequireValidLicense || effectiveLicensePath != "" || effectivePublicKeyPath != "")
		if shouldVerifyLicense {
			if effectiveLicensePath == "" || effectivePublicKeyPath == "" {
				fmt.Fprintln(os.Stderr, "license verification requires both --license-file and --provider-public-key")
				os.Exit(1)
			}
			fingerprintHash := strings.TrimSpace(*machineFingerprint)
			if fingerprintHash == "" {
				snapshot, err := machinefingerprint.Collect(machinefingerprint.Options{})
				if err != nil {
					fmt.Fprintf(os.Stderr, "machine fingerprint failed: %v\n", err)
					os.Exit(1)
				}
				fingerprintHash = snapshot.FingerprintHash(runtime.Identity.FingerprintSaltID)
			}
			licenseResult, err := licenseverify.VerifyFile(effectiveLicensePath, effectivePublicKeyPath, licenseverify.Options{
				RequiredMode:           runtime.Mode,
				MachineFingerprintHash: fingerprintHash,
				DisableGracePeriod:     runtime.License.RequireValidLicense && !runtime.License.AllowGracePeriod,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "license check failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("license ok: license_id=%s server_id=%s plan=%s expired=%t\n",
				licenseResult.LicenseID, licenseResult.ServerID, licenseResult.Plan, licenseResult.Expired)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "kiro-agent phase0 supports --check and --version")
	os.Exit(2)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
