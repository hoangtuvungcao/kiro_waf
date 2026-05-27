package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"kiro_waf/internal/agent/firewall"
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
	firewallDryRun := flag.Bool("firewall-dry-run", false, "generate nftables rules and exit without applying")
	firewallSnapshotDir := flag.String("firewall-snapshot-dir", "", "optional directory for dry-run last-good snapshot")
	firewallApply := flag.Bool("firewall-apply", false, "LAB ONLY: apply generated nftables rules with pending rollback")
	firewallConfirm := flag.Bool("firewall-confirm", false, "confirm pending firewall apply and remove rollback state")
	firewallRollback := flag.Bool("firewall-rollback", false, "LAB ONLY: rollback pending firewall apply")
	firewallRollbackIfExpired := flag.Bool("firewall-rollback-if-expired", false, "LAB ONLY: rollback pending firewall apply if deadline has expired")
	firewallStateDir := flag.String("firewall-state-dir", "", "override firewall state directory")
	firewallRollbackSeconds := flag.Int("firewall-rollback-seconds", 0, "override firewall rollback timer in seconds")
	firewallLabAck := flag.String("firewall-lab-ack", "", "required value for firewall apply/rollback: KIRO_LAB_FIREWALL_APPLY")
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
	if *firewallDryRun {
		runtime, err := config.LoadRuntimeFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "runtime expansion failed: %v\n", err)
			os.Exit(1)
		}
		plan, err := firewall.GenerateNftables(runtime, firewall.Options{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "firewall dry-run failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(plan.Ruleset)
		if *firewallSnapshotDir != "" {
			path, err := firewall.WriteLastGoodSnapshot(*firewallSnapshotDir, runtime, plan, time.Time{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "firewall snapshot failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "firewall snapshot: %s sha256=%s\n", path, plan.SHA256)
		}
		return
	}
	if *firewallApply || *firewallConfirm || *firewallRollback || *firewallRollbackIfExpired {
		runtime, err := config.LoadRuntimeFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "runtime expansion failed: %v\n", err)
			os.Exit(1)
		}
		stateDir := firstNonEmpty(*firewallStateDir, runtime.Paths.StateDir)
		if *firewallConfirm {
			if err := firewall.ConfirmPendingApply(stateDir); err != nil {
				fmt.Fprintf(os.Stderr, "firewall confirm failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("firewall apply confirmed: state_dir=%s\n", stateDir)
			return
		}
		requireFirewallLabAck(*firewallLabAck)
		requireRootForFirewall()
		runner := firewall.NFTRunner{}
		if *firewallRollback {
			if err := firewall.RollbackPendingApply(stateDir, runner); err != nil {
				fmt.Fprintf(os.Stderr, "firewall rollback failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("firewall rollback applied: state_dir=%s\n", stateDir)
			return
		}
		if *firewallRollbackIfExpired {
			rolledBack, err := firewall.RollbackIfExpired(stateDir, runner, time.Time{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "firewall rollback check failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("firewall rollback expired=%t state_dir=%s\n", rolledBack, stateDir)
			return
		}
		plan, err := firewall.GenerateNftables(runtime, firewall.Options{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "firewall generate failed: %v\n", err)
			os.Exit(1)
		}
		result, err := firewall.ApplyNftables(runtime, plan, runner, firewall.ApplyOptions{
			StateDir:        stateDir,
			SnapshotDir:     firstNonEmpty(*firewallSnapshotDir, runtime.Paths.LastGoodConfigDir),
			RollbackSeconds: firstNonZero(*firewallRollbackSeconds, runtime.Safety.RollbackTimerSeconds),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "firewall apply failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("firewall applied: sha256=%s pending=%s rollback_deadline=%s snapshot=%s\n",
			result.AppliedRulesetSHA256, result.PendingPath, result.RollbackDeadline, result.SnapshotPath)
		return
	}
	fmt.Fprintln(os.Stderr, "usage: kiro-agent --check | --version | --firewall-dry-run | --firewall-apply | --firewall-confirm | --firewall-rollback")
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

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func requireFirewallLabAck(value string) {
	if value != "KIRO_LAB_FIREWALL_APPLY" {
		fmt.Fprintln(os.Stderr, "firewall apply/rollback is lab-only and requires --firewall-lab-ack KIRO_LAB_FIREWALL_APPLY")
		os.Exit(1)
	}
}

func requireRootForFirewall() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "firewall apply/rollback requires root")
		os.Exit(1)
	}
}
