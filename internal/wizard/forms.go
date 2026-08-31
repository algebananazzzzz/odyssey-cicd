package wizard

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/algebananazzzzz/odyssey/internal/cli"
	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

func options[K ~string](keys []K) []huh.Option[string] {
	var out []huh.Option[string]
	for _, k := range keys {
		out = append(out, huh.NewOption(string(k), string(k)))
	}
	return out
}

func architectureForm(m *types.Manifest, shapes []string, a *render.Answers) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Key("provider").Title("Provider").
			Options(options(utils.Sorted(m.Providers))...).Value(&a.Provider),
		huh.NewSelect[string]().Key("architecture").Title("Architecture").
			Height(1+maxArchsPerProvider(m)).
			OptionsFunc(func() []huh.Option[string] {
				var archs []types.Architecture
				for _, arch := range utils.Sorted(m.Architectures) {
					if m.Architectures[arch].Provider == types.Provider(a.Provider) {
						archs = append(archs, arch)
					}
				}
				return options(archs)
			}, &a.Provider).Value(&a.Architecture),
		huh.NewSelect[string]().Key("stack").Title("Stack").
			Height(1+maxStacksPerArchitecture(m)).
			OptionsFunc(func() []huh.Option[string] {
				var stacks []types.Stack
				for _, s := range utils.Sorted(m.Stacks) {
					for _, arch := range m.Stacks[s].Architectures {
						if string(arch) == a.Architecture {
							stacks = append(stacks, s)
						}
					}
				}
				return options(stacks)
			}, &a.Architecture).Value(&a.Stack),
		huh.NewSelect[string]().Key("environments").Title("Environments").
			Options(options(shapes)...).Value(&a.Environments),
	)).WithWidth(formWidth).WithShowHelp(false).WithShowErrors(false)
}

func maxArchsPerProvider(m *types.Manifest) int {
	counts := map[types.Provider]int{}
	most := 1
	for _, spec := range m.Architectures {
		counts[spec.Provider]++
		most = max(most, counts[spec.Provider])
	}
	return most
}

func maxStacksPerArchitecture(m *types.Manifest) int {
	counts := map[types.Architecture]int{}
	most := 1
	for _, spec := range m.Stacks {
		for _, arch := range spec.Architectures {
			counts[arch]++
			most = max(most, counts[arch])
		}
	}
	return most
}

func projectForm(a *render.Answers) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Key("project").Title("Project code").
			Validate(cli.ValidProject).Value(&a.Project),
		huh.NewInput().Key("dir").Title("Directory").
			PlaceholderFunc(func() string { return "./" + a.Project }, &a.Project).
			Validate(func(dir string) error {
				if dir == "" {
					return nil
				}
				return render.TargetOK(dir)
			}).Value(&a.Dir),
	)).WithWidth(formWidth).WithShowHelp(false).WithShowErrors(false)
}

func variablesForm(asks []render.Ask, envs []string, screen int, vals map[string]map[string]*string) *huh.Form {
	var fields []huh.Field
	env := envs[screen]
	for _, ask := range asks {
		if screen > 0 && !ask.PerEnv {
			continue
		}
		title := ask.Name
		if ask.PerEnv {
			title += " (" + env + ")"
		}
		if ask.Optional {
			title += " · optional"
		}
		fields = append(fields, huh.NewInput().Key(ask.Name).Title(title).
			Validate(requiredUnless(ask.Optional)).Value(vals[env][ask.Name]))
	}
	return huh.NewForm(huh.NewGroup(fields...)).WithWidth(formWidth).WithShowHelp(false).WithShowErrors(false)
}

func confirmForm(n int, dir string, ok *bool) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Key("write").
			Title(fmt.Sprintf("Write %d files to %s?", n, dir)).
			Affirmative("Yes").Negative("No").Value(ok),
	)).WithWidth(formWidth).WithShowHelp(false).WithShowErrors(false)
}

func requiredUnless(optional bool) func(string) error {
	return func(s string) error {
		if !optional && s == "" {
			return errors.New("required")
		}
		return nil
	}
}
