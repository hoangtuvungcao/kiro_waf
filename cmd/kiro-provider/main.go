package main

import (
	"flag"
	"fmt"
	"os"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
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
	fmt.Fprintln(os.Stderr, "kiro-provider phase0 supports --check and --version")
	os.Exit(2)
}
