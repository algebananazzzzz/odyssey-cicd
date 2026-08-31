package wizard

import (
	"errors"
	"regexp"

	"github.com/charmbracelet/huh"

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
	)).WithTheme(theme())
}

var projectRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validProject(s string) error {
	if !projectRe.MatchString(s) {
		return errors.New("lowercase letters, digits and hyphens, starting with a letter")
	}
	return nil
}

func projectForm(a *render.Answers) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Key("project").Title("Project code").
			Validate(validProject).Value(&a.Project),
		huh.NewInput().Key("dir").Title("Directory").
			Placeholder("./<project code>").
			Validate(func(dir string) error {
				if dir == "" {
					return nil
				}
				return render.TargetOK(dir)
			}).Value(&a.Dir),
	)).WithTheme(theme())
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
	return huh.NewForm(huh.NewGroup(fields...)).WithTheme(theme())
}

func requiredUnless(optional bool) func(string) error {
	return func(s string) error {
		if !optional && s == "" {
			return errors.New("required")
		}
		return nil
	}
}
