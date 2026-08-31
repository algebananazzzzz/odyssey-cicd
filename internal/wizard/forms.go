package wizard

import (
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
