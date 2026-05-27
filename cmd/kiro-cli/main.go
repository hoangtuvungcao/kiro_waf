package main

import (
	"fmt"
	"os"

	"kiro_waf/internal/shared/buildinfo"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(buildinfo.Version)
		return
	}
	fmt.Fprintln(os.Stderr, "usage: kiro-cli version")
	os.Exit(2)
}
