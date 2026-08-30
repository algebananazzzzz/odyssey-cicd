package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func DirExists(root, rel string) error {
	info, err := os.Stat(filepath.Join(root, rel))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("directory %s missing", rel)
	}
	return nil
}

func ReadYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
