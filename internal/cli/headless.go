package cli

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

var varName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func ParseVars(pairs, envs []string) (map[string]map[string]string, error) {
	vars := map[string]map[string]string{}
	for _, env := range envs {
		vars[env] = map[string]string{}
	}
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("--var %q: want NAME=VALUE", pair)
		}
		targets := envs
		if env, rest, scoped := strings.Cut(name, ":"); scoped {
			if !slices.Contains(envs, env) {
				return nil, fmt.Errorf("--var %q: unknown environment %q", pair, env)
			}
			targets, name = []string{env}, rest
		}
		if !varName.MatchString(name) {
			return nil, fmt.Errorf("--var %q: invalid name %q", pair, name)
		}
		for _, env := range targets {
			vars[env][name] = value
		}
	}
	return vars, nil
}

func Infer(m *types.Manifest, a *render.Answers) error {
	if a.Stack != "" && a.Architecture == "" {
		spec, ok := m.Stacks[types.Stack(a.Stack)]
		if !ok {
			return fmt.Errorf("unknown stack %q", a.Stack)
		}
		archs := spec.Architectures
		if a.Provider != "" {
			archs = nil
			for _, arch := range spec.Architectures {
				if m.Architectures[arch].Provider == types.Provider(a.Provider) {
					archs = append(archs, arch)
				}
			}
			if len(archs) == 0 {
				return fmt.Errorf("stack %q has no architecture on provider %q", a.Stack, a.Provider)
			}
		}
		if len(archs) == 1 {
			a.Architecture = string(archs[0])
		}
	}
	if a.Architecture != "" {
		spec, ok := m.Architectures[types.Architecture(a.Architecture)]
		if !ok {
			return fmt.Errorf("unknown architecture %q", a.Architecture)
		}
		if a.Provider == "" {
			a.Provider = string(spec.Provider)
		} else if a.Provider != string(spec.Provider) {
			return fmt.Errorf("architecture %q runs on %q, not %q", a.Architecture, spec.Provider, a.Provider)
		}
		if a.Stack != "" && !slices.Contains(m.Stacks[types.Stack(a.Stack)].Architectures, types.Architecture(a.Architecture)) {
			return fmt.Errorf("stack %q does not target %q", a.Stack, a.Architecture)
		}
	}
	return nil
}

type Choice struct {
	Flag    string
	Options []string
}

func Missing(m *types.Manifest, shapes []string, a render.Answers) []Choice {
	var out []Choice
	if a.Provider == "" {
		out = append(out, Choice{"--provider", asStrings(utils.Sorted(m.Providers))})
	}
	if a.Architecture == "" {
		var opts []string
		for _, arch := range utils.Sorted(m.Architectures) {
			if a.Provider == "" || m.Architectures[arch].Provider == types.Provider(a.Provider) {
				opts = append(opts, string(arch))
			}
		}
		if a.Stack != "" {
			opts = nil
			for _, arch := range m.Stacks[types.Stack(a.Stack)].Architectures {
				opts = append(opts, string(arch))
			}
			sort.Strings(opts)
		}
		out = append(out, Choice{"--architecture", opts})
	}
	if a.Stack == "" {
		var opts []string
		for _, s := range utils.Sorted(m.Stacks) {
			if a.Architecture == "" || slices.Contains(m.Stacks[s].Architectures, types.Architecture(a.Architecture)) {
				opts = append(opts, string(s))
			}
		}
		out = append(out, Choice{"--stack", opts})
	}
	if a.Environments == "" {
		out = append(out, Choice{"--environments", shapes})
	}
	if a.Project == "" {
		out = append(out, Choice{"--project", []string{"<name>"}})
	}
	return out
}

func asStrings[K ~string](keys []K) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}

func Report(a render.Answers, derived map[string]bool, missing []Choice, asks []render.Ask) string {
	var b strings.Builder
	row := func(label, value, axis string) {
		if value == "" {
			return
		}
		tag := ""
		if derived[axis] {
			tag = "   (derived)"
		}
		fmt.Fprintf(&b, "%-13s %-17s%s\n", label, value, tag)
	}
	row("stack", a.Stack, "stack")
	row("architecture", a.Architecture, "architecture")
	row("provider", a.Provider, "provider")
	row("environments", a.Environments, "environments")
	row("project", a.Project, "project")
	if a.Project != "" && a.Dir == "" {
		fmt.Fprintf(&b, "%-13s ./%s          (default)\n", "dir", a.Project)
	}
	if len(missing) > 0 {
		b.WriteString("\nmissing\n")
		for _, c := range missing {
			fmt.Fprintf(&b, "  %-20s %s\n", c.Flag, strings.Join(c.Options, " | "))
		}
	}
	var optional, required []render.Ask
	for _, ask := range asks {
		if ask.Optional {
			optional = append(optional, ask)
		} else {
			required = append(required, ask)
		}
	}
	if len(required) > 0 {
		b.WriteString("\nrequired\n")
		for _, ask := range required {
			fmt.Fprintf(&b, "  --var %s=%s\n", ask.Name, perEnvHint(ask))
		}
	}
	if len(optional) > 0 {
		b.WriteString("\noptional (empty ok)\n")
		for _, ask := range optional {
			fmt.Fprintf(&b, "  --var %s=%s\n", ask.Name, perEnvHint(ask))
		}
	}
	b.WriteString("\nadd the missing flags; --yes applies without confirmation\n")
	return b.String()
}

func perEnvHint(ask render.Ask) string {
	if ask.PerEnv {
		return "          per-env: --var prd:" + ask.Name + "="
	}
	return ""
}
