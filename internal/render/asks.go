package render

import (
	"sort"

	"github.com/algebananazzzzz/odyssey/internal/types"
)

type Ask struct {
	Name     string
	Optional bool
	PerEnv   bool
}

func (p *Plan) Unresolved() []string {
	seen := map[string]bool{}
	for _, f := range p.Files {
		for _, sub := range tokenRe.FindAllStringSubmatch(f.Body, -1) {
			if sub[1] == "" {
				seen[sub[2]] = true
			}
		}
	}
	var names []string
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func Asks(m *types.Manifest, p *Plan) []Ask {
	perEnv := map[string]bool{}
	seen := map[string]bool{}
	for _, f := range p.Files {
		for _, sub := range tokenRe.FindAllStringSubmatch(f.Body, -1) {
			if sub[1] != "" {
				continue
			}
			seen[sub[2]] = true
			if f.PerEnv {
				perEnv[sub[2]] = true
			}
		}
	}
	optional := map[string]bool{}
	merge := func(inputs map[string]types.Input) {
		for name, in := range inputs {
			optional[name] = in.Optional
		}
	}
	merge(m.Inputs)
	merge(m.Providers[types.Provider(p.Answers.Provider)].Inputs)
	merge(m.Architectures[types.Architecture(p.Answers.Architecture)].Inputs)
	merge(m.Stacks[types.Stack(p.Answers.Stack)].Inputs)
	var asks []Ask
	for name := range seen {
		asks = append(asks, Ask{Name: name, Optional: optional[name], PerEnv: perEnv[name]})
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Name < asks[j].Name })
	return asks
}
