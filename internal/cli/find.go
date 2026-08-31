package cli

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

type Row struct {
	Stack, Architecture, Provider string
}

func Rows(m *types.Manifest) []Row {
	var rows []Row
	for _, stack := range utils.Sorted(m.Stacks) {
		archs := append([]types.Architecture(nil), m.Stacks[stack].Architectures...)
		sort.Slice(archs, func(i, j int) bool { return archs[i] < archs[j] })
		for _, arch := range archs {
			rows = append(rows, Row{
				Stack:        string(stack),
				Architecture: string(arch),
				Provider:     string(m.Architectures[arch].Provider),
			})
		}
	}
	return rows
}

func Filter(rows []Row, terms []string) ([]Row, error) {
	for _, term := range terms {
		axis, value, exact := strings.Cut(term, "=")
		var keep []Row
		for _, r := range rows {
			match := false
			if exact {
				switch axis {
				case "stack":
					match = r.Stack == value
				case "architecture":
					match = r.Architecture == value
				case "provider":
					match = r.Provider == value
				default:
					return nil, fmt.Errorf("unknown axis %q (stack, architecture, provider)", axis)
				}
			} else {
				match = strings.Contains(r.Stack, term) ||
					strings.Contains(r.Architecture, term) ||
					strings.Contains(r.Provider, term)
			}
			if match {
				keep = append(keep, r)
			}
		}
		rows = keep
	}
	return rows, nil
}

func Table(rows []Row) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-16s %-19s %s\n", "STACK", "ARCHITECTURE", "PROVIDER")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-16s %-19s %s\n", r.Stack, r.Architecture, r.Provider)
	}
	return b.String()
}

func Card(tfs fs.FS, m *types.Manifest, shapes []string, r Row) (string, error) {
	a := render.Answers{
		Provider: r.Provider, Architecture: r.Architecture,
		Stack: r.Stack, Environments: shapes[0], Project: "project",
	}
	scan, err := render.Build(tfs, m, a)
	if err != nil {
		return "", err
	}
	asks := render.Asks(m, scan)
	var inputs []string
	for _, ask := range asks {
		s := ask.Name
		if ask.Optional {
			s += "?"
		}
		if ask.PerEnv {
			s += " (per env)"
		}
		inputs = append(inputs, s)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-14s %s\n", "stack", r.Stack)
	fmt.Fprintf(&b, "%-14s %s\n", "architecture", r.Architecture)
	fmt.Fprintf(&b, "%-14s %s\n", "provider", r.Provider)
	fmt.Fprintf(&b, "%-14s %s\n", "environments", strings.Join(shapes, " | "))
	fmt.Fprintf(&b, "%-14s %s\n", "inputs", strings.Join(inputs, " "))
	fmt.Fprintf(&b, "%-14s %s\n", "github vars", strings.Join(scan.Github.Variables, " "))
	fmt.Fprintf(&b, "%-14s %s\n", "github secrets", strings.Join(scan.Github.Secrets, " "))
	fmt.Fprintf(&b, "\nodyssey-cli new --stack %s --environments %s --project <name>\n", r.Stack, shapes[0])
	return b.String(), nil
}
