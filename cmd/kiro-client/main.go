package main

import (
	"os"

	client "kiro_waf/internal/client"
)

func main() {
	os.Exit(client.Run())
}
