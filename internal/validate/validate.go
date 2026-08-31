package validate

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

// Uppercase-only, so it can never match a GitHub expression like
// ${{ vars.X }}, whose body starts with a space or a lowercase context.
var placeholder = regexp.MustCompile(`\{\{([A-Z][A-Z0-9_]*)\}\}`)

var githubRef = regexp.MustCompile(`\$\{\{\s*(vars|secrets)\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

var engineVars = map[string]bool{
	"PROJECT":     true,
	"ENV":         true,
	"ENV_LIST":    true,
	"PREPROD_ENV": true,
	"PRD_ENV":     true,
}

func Manifest(fsys fs.FS) (*types.Manifest, error) {
	data, err := fs.ReadFile(fsys, "manifest.yml")
	if err != nil {
		return nil, err
	}
	var m types.Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest.yml: %w", err)
	}
	return &m, nil
}

func All(root string, m *types.Manifest) error {
	if err := references(root, m); err != nil {
		return err
	}
	if err := contract(root, m); err != nil {
		return err
	}
	return syntax(root)
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

func contract(root string, m *types.Manifest) error {
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, t := range placeholder.FindAllStringSubmatch(d.Name(), -1) {
			if t[1] != "ENV" {
				return fmt.Errorf("%s: only {{ENV}} may appear in a file name", rel)
			}
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		owner, spec := ownerOf(rel, m)
		for _, t := range placeholder.FindAllStringSubmatch(string(data), -1) {
			name := t[1]
			if engineVars[name] {
				continue
			}
			if !allowsCustomVars(rel) {
				return fmt.Errorf("%s: {{%s}} outside terraform config and workflow files", rel, name)
			}
			if _, ok := spec.Inputs[name]; !ok {
				return fmt.Errorf("%s: {{%s}} not declared by %s", rel, name, owner)
			}
			used[owner+" input "+name] = true
		}
		if !strings.HasSuffix(rel, ".yml") {
			return nil
		}
		for _, r := range githubRef.FindAllStringSubmatch(string(data), -1) {
			kind, name := r[1], r[2]
			if name == "GITHUB_TOKEN" {
				continue
			}
			pool := spec.Github.Variables
			if kind == "secrets" {
				pool = spec.Github.Secrets
			}
			if !slices.Contains(pool, name) {
				return fmt.Errorf("%s: %s.%s not declared by %s", rel, kind, name, owner)
			}
			used[owner+" "+kind+" "+name] = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, o := range owners(m) {
		if err := declared(o.name, o.spec, used); err != nil {
			return err
		}
	}
	return nil
}

type owned struct {
	name string
	spec types.Spec
}

func owners(m *types.Manifest) []owned {
	list := []owned{{"base", baseSpec(m)}}
	for _, p := range utils.Sorted(m.Providers) {
		list = append(list, owned{"provider " + string(p), m.Providers[p]})
	}
	for _, a := range utils.Sorted(m.Architectures) {
		list = append(list, owned{"architecture " + string(a), m.Architectures[a]})
	}
	for _, s := range utils.Sorted(m.Stacks) {
		list = append(list, owned{"stack " + string(s), m.Stacks[s]})
	}
	return list
}

func declared(owner string, spec types.Spec, used map[string]bool) error {
	for _, name := range utils.Sorted(spec.Inputs) {
		if engineVars[name] {
			return fmt.Errorf("%s declares input %s, an engine variable", owner, name)
		}
		if !used[owner+" input "+name] {
			return fmt.Errorf("%s declares input %s but no fragment uses it", owner, name)
		}
	}
	for _, name := range spec.Github.Variables {
		if !used[owner+" vars "+name] {
			return fmt.Errorf("%s declares variable %s but no fragment references it", owner, name)
		}
	}
	for _, name := range spec.Github.Secrets {
		if !used[owner+" secrets "+name] {
			return fmt.Errorf("%s declares secret %s but no fragment references it", owner, name)
		}
	}
	return nil
}

func ownerOf(rel string, m *types.Manifest) (string, types.Spec) {
	seg := strings.Split(rel, "/")
	switch {
	case len(seg) > 2 && seg[0] == "infra" && seg[1] == "providers":
		return "provider " + seg[2], m.Providers[types.Provider(seg[2])]
	case len(seg) > 2 && seg[0] == "infra" && seg[1] == "architecture":
		return "architecture " + seg[2], m.Architectures[types.Architecture(seg[2])]
	case len(seg) > 2 && seg[0] == "makefile" && seg[1] == "deploy":
		return "architecture " + seg[2], m.Architectures[types.Architecture(seg[2])]
	case len(seg) > 2 && seg[0] == "makefile" && seg[1] == "stack":
		return "stack " + seg[2], m.Stacks[types.Stack(seg[2])]
	case len(seg) > 1 && seg[0] == "stacks":
		return "stack " + seg[1], m.Stacks[types.Stack(seg[1])]
	default:
		return "base", baseSpec(m)
	}
}

func baseSpec(m *types.Manifest) types.Spec {
	return types.Spec{Inputs: m.Inputs}
}

func allowsCustomVars(rel string) bool {
	if strings.HasPrefix(rel, "infra/") && strings.Contains(rel, "/config/") {
		return true
	}
	return strings.HasSuffix(rel, ".yml") &&
		(strings.HasPrefix(rel, "workflows/") || strings.Contains(rel, "/workflows/"))
}

func syntax(root string) error {
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
