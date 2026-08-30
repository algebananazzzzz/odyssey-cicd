package engine

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Architecture struct {
	Provider string `yaml:"provider"`
	Infra    string `yaml:"infra"`
}

type Manifest struct {
	SchemaVersion int                     `yaml:"schema_version"`
	Environments  map[string][]string     `yaml:"environments"`
	Providers     []string                `yaml:"providers"`
	Architectures map[string]Architecture `yaml:"architectures"`
	Stacks        map[string][]string     `yaml:"stacks"`
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("manifest schema_version %d: upgrade odyssey-cli", m.SchemaVersion)
	}
	return &m, nil
}
