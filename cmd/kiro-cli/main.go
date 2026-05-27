package main

import (
	"flag"
	"fmt"
	"os"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/machinefingerprint"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(buildinfo.Version)
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "license" && os.Args[2] == "fingerprint" {
		fingerprintCmd := flag.NewFlagSet("kiro-cli license fingerprint", flag.ExitOnError)
		salt := fingerprintCmd.String("salt", "", "provider fingerprint salt id")
		if err := fingerprintCmd.Parse(os.Args[3:]); err != nil {
			os.Exit(2)
		}
		snapshot, err := machinefingerprint.Collect(machinefingerprint.Options{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "fingerprint failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(snapshot.FingerprintHash(*salt))
		return
	}
	fmt.Fprintln(os.Stderr, "usage: kiro-cli version | kiro-cli license fingerprint [--salt ID]")
	os.Exit(2)
}
