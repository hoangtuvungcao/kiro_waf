package main

import (
	"flag"
	"fmt"
	"os"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
)

func main() {
	configPath := flag.String("config", "configs/kiro.example.yaml", "path to kiro config")
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
		return
	}
	fmt.Fprintln(os.Stderr, "kiro-agent phase0 supports --check and --version")
	os.Exit(2)
}
