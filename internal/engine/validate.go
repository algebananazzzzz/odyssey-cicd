package engine

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Combo struct {
	Selection
	Envs []string
}

// Combos enumerates every selection the manifest allows.
func (m *Manifest) Combos() []Combo {
	var out []Combo
	for layout, envs := range m.Environments {
		for stack, archs := range m.Stacks {
			for _, arch := range archs {
				spec := m.Architectures[arch]
				sels := []Selection{{layout, spec.Provider, arch, stack}}
				if spec.Infra == "optional" {
					sels = append(sels, Selection{layout, "", arch, stack})
				}
				for _, sel := range sels {
					out = append(out, Combo{sel, envs})
				}
			}
		}
	}
	return out
}

// ValidateAll renders every combo with derived token values and checks the
// result. Token values only need to exist to prove none survive, so they
// derive from the name: {{STATE_BUCKET}} -> "state-bucket".
func ValidateAll(frag fs.FS, m *Manifest) error {
	for _, c := range m.Combos() {
		if err := m.Validate(c.Selection); err != nil {
			return err
		}
		names, err := UnresolvedTokens(frag, c.Selection, c.Envs)
		if err != nil {
			return err
		}
		tokens := Tokens{}
		for _, n := range names {
			tokens[n] = strings.ToLower(strings.ReplaceAll(n, "_", "-"))
		}
		dir, err := os.MkdirTemp("", "odyssey-validate-")
		if err != nil {
			return err
		}
		if err := Render(frag, c.Selection, c.Envs, tokens, dir); err != nil {
			return err
		}
		if err := check(dir, c.Provider != ""); err != nil {
			return fmt.Errorf("%s/%s/%s: %w", c.Environments, c.Stack, c.Architecture, err)
		}
		os.RemoveAll(dir)
		provider := c.Provider
		if provider == "" {
			provider = "-"
		}
		fmt.Printf("ok  %-6s %-15s %-17s %s\n", c.Environments, c.Stack, c.Architecture, provider)
	}
	return nil
}

func check(dir string, infra bool) error {
	workflows, err := os.ReadDir(filepath.Join(dir, ".github/workflows"))
	if err != nil {
		return err
	}
	for _, wf := range workflows {
		data, err := os.ReadFile(filepath.Join(dir, ".github/workflows", wf.Name()))
		if err != nil {
			return err
		}
		var doc any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("%s: %w", wf.Name(), err)
		}
		text := string(data)
		if strings.Contains(text, "# >>>") {
			return fmt.Errorf("%s: unfilled marker", wf.Name())
		}
		if strings.Count(text, "make infra ENV") > 1 {
			return fmt.Errorf("%s: duplicated make infra", wf.Name())
		}
		if !infra && (strings.Contains(text, "terraform") || strings.Contains(text, "make infra")) {
			return fmt.Errorf("%s: infra residue", wf.Name())
		}
	}

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range tokenRe.FindAllStringSubmatch(string(data), -1) {
			if m[1] == "" && m[2] != "ENV" || strings.Contains(filepath.Base(path), "{{") {
				return fmt.Errorf("%s: unsubstituted {{%s}}", path, m[2])
			}
		}
		if strings.HasSuffix(path, ".sh") {
			return run(dir, "bash", "-n", path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := run(dir, "make", "-n", "ci"); err != nil {
		return err
	}
	if err := run(dir, "make", "-n", "deploy"); err != nil {
		return err
	}
	if infra {
		for _, cmd := range [][]string{
			{"make", "-n", "infra", "ENV=prd"},
			{"terraform", "-chdir=infra", "fmt", "-check", "-recursive"},
			{"terraform", "-chdir=infra", "init", "-backend=false", "-input=false"},
			{"terraform", "-chdir=infra", "validate"},
		} {
			if err := run(dir, cmd[0], cmd[1:]...); err != nil {
				return err
			}
		}
	}
	return nil
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}
