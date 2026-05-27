package tests

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestYAMLExamplesParse(t *testing.T) {
	files := []string{
		"configs/kiro.example.yaml",
		"configs/kiro.advanced.example.yaml",
		"configs/provider.example.yaml",
		"configs/tenant.server-only.example.yaml",
		"configs/tenant.full-cloudflare.example.yaml",
		"configs/tenant.full-strict.example.yaml",
		"rules/ddos.yaml",
		"rules/bot.yaml",
		"rules/overload.yaml",
		"examples/site-mapping.example.yaml",
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile("../" + file)
			if err != nil {
				t.Fatalf("read yaml: %v", err)
			}
			var value any
			if err := yaml.Unmarshal(raw, &value); err != nil {
				t.Fatalf("parse yaml: %v", err)
			}
			if value == nil {
				t.Fatal("expected non-empty yaml")
			}
		})
	}
}
