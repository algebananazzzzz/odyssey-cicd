package validate

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

func Manifest(path string) (*types.Manifest, error) {
	var m types.Manifest
	if err := utils.ReadYAML(path, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func All(root string, m *types.Manifest) error {
	if err := references(root, m); err != nil {
		return err
	}
	return files(root)
}

func references(root string, m *types.Manifest) error {
	for provider := range m.Providers {
		if err := utils.DirExists(root, "infra/providers/"+string(provider)); err != nil {
			return err
		}
	}
	for architecture, spec := range m.Architectures {
		if _, ok := m.Providers[spec.Provider]; !ok {
			return fmt.Errorf("architecture %q: provider %q not declared", architecture, spec.Provider)
		}
		if err := utils.DirExists(root, "makefile/deploy/"+string(architecture)); err != nil {
			return err
		}
	}
	for stack, spec := range m.Stacks {
		for _, architecture := range spec.Architectures {
			if _, ok := m.Architectures[architecture]; !ok {
				return fmt.Errorf("stack %q: architecture %q not declared", stack, architecture)
			}
		}
		if err := utils.DirExists(root, "stacks/"+string(stack)); err != nil {
			return err
		}
		if err := utils.DirExists(root, "makefile/stack/"+string(stack)); err != nil {
			return err
		}
	}
	return nil
}

func files(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch {
		case strings.HasSuffix(path, ".sh"):
			return run("bash", "-n", path)
		case strings.HasSuffix(path, ".yml"):
			var doc any
			return utils.ReadYAML(path, &doc)
		}
		return nil
	})
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}
