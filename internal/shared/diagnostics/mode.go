package diagnostics

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ShowMode(path string) (string, error) {
	result, err := configMode(path)
	if err != nil {
		return "", err
	}
	return result, nil
}

func SetModeFile(path string, mode string) error {
	if mode != "server" && mode != "full" {
		return fmt.Errorf("mode must be server or full, got %q", mode)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return errors.New("config root must be a YAML mapping")
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "mode" {
			root.Content[i+1].Kind = yaml.ScalarNode
			root.Content[i+1].Tag = "!!str"
			root.Content[i+1].Value = mode
			out, err := yaml.Marshal(&doc)
			if err != nil {
				return err
			}
			return os.WriteFile(path, out, 0o644)
		}
	}
	root.Content = append([]*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mode"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: mode},
	}, root.Content...)
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func configMode(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var probe map[string]any
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return "", err
	}
	mode, _ := probe["mode"].(string)
	if mode == "" {
		return "", errors.New("mode is not set")
	}
	return mode, nil
}
