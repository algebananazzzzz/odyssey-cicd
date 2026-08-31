package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/algebananazzzzz/odyssey/internal/cli"
	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/validate"
	"github.com/algebananazzzzz/odyssey/internal/wizard"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "validate":
		runValidate(os.Args[2:])
	case "new":
		runNew(os.Args[2:])
	case "find":
		runFind(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: odyssey-cli validate|new|find [flags]")
	os.Exit(2)
}

func load(templates string) (*types.Manifest, []string) {
	m, err := validate.Manifest(templates + "/manifest.yml")
	if err != nil {
		fatal(err)
	}
	if err := validate.All(templates+"/fragments", m); err != nil {
		fatal(err)
	}
	shapes, err := render.Shapes(templates)
	if err != nil {
		fatal(err)
	}
	return m, shapes
}

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	templates := fs.String("templates", ".", "path to a cicd-templates checkout")
	fs.Parse(args)
	load(*templates)
	fmt.Println("manifest and fragments are valid")
}

type varFlags []string

func (v *varFlags) String() string     { return fmt.Sprint(*v) }
func (v *varFlags) Set(s string) error { *v = append(*v, s); return nil }

func runNew(args []string) {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	templates := fs.String("templates", ".", "path to a cicd-templates checkout")
	var a render.Answers
	fs.StringVar(&a.Provider, "provider", "", "cloud provider")
	fs.StringVar(&a.Architecture, "architecture", "", "deploy architecture")
	fs.StringVar(&a.Stack, "stack", "", "application stack")
	fs.StringVar(&a.Environments, "environments", "", "environment shape")
	fs.StringVar(&a.Project, "project", "", "project code")
	fs.StringVar(&a.Dir, "dir", "", "target directory (default ./<project>)")
	var vars varFlags
	fs.Var(&vars, "var", "NAME=VALUE or env:NAME=VALUE, repeatable")
	yes := fs.Bool("yes", false, "apply without confirmation")
	bootstrap := fs.Bool("bootstrap", false, "run the stack bootstrap after apply")
	fs.Parse(args)

	m, shapes := load(*templates)
	if interactive() {
		runTUI(*templates, m, shapes, a, vars, *yes, *bootstrap)
		return
	}
	runHeadless(*templates, m, shapes, a, vars, *yes, *bootstrap)
}

func interactive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

func runHeadless(templates string, m *types.Manifest, shapes []string, a render.Answers, vars varFlags, yes, bootstrap bool) {
	derived := map[string]bool{}
	before := a
	if err := cli.Infer(m, &a); err != nil {
		fatal(err)
	}
	derived["architecture"] = before.Architecture == "" && a.Architecture != ""
	derived["provider"] = before.Provider == "" && a.Provider != ""

	if a.Project != "" {
		if err := cli.ValidProject(a.Project); err != nil {
			fatal(err)
		}
	}
	if a.Environments != "" && !slices.Contains(shapes, a.Environments) {
		fatal(fmt.Errorf("unknown environments shape %q, valid: %s", a.Environments, strings.Join(shapes, ", ")))
	}

	missing := cli.Missing(m, shapes, a)
	if len(missing) > 0 {
		if a.Stack == "" && a.Architecture == "" && a.Provider == "" && a.Project == "" {
			fmt.Println("run `odyssey-cli find` to browse stacks, architectures and providers")
		}
		var asks []render.Ask
		if a.Provider != "" && a.Architecture != "" && a.Stack != "" && a.Environments == "" {
			asks = unionAsks(templates, m, a, shapes)
		}
		fmt.Print(cli.Report(a, derived, missing, asks))
		os.Exit(2)
	}
	scan, err := render.Build(templates, m, a)
	if err != nil {
		fatal(err)
	}
	parsed, err := cli.ParseVars(vars, scan.Envs)
	if err != nil {
		fatal(err)
	}
	a.Vars = parsed
	asks := render.Asks(m, scan)
	incomplete := false
	var pending []render.Ask
	for _, ask := range asks {
		missing := false
		for _, env := range scan.Envs {
			if _, ok := a.Vars[env][ask.Name]; !ok {
				missing = true
				if ask.Optional {
					a.Vars[env][ask.Name] = ""
				} else {
					incomplete = true
				}
			}
		}
		if missing {
			pending = append(pending, ask)
		}
	}
	if incomplete {
		fmt.Print(cli.Report(a, derived, nil, pending))
		os.Exit(2)
	}
	finish(templates, m, a, yes, bootstrap)
}

func unionAsks(templates string, m *types.Manifest, a render.Answers, shapes []string) []render.Ask {
	byName := map[string]render.Ask{}
	for _, shape := range shapes {
		a2 := a
		a2.Environments = shape
		scan, err := render.Build(templates, m, a2)
		if err != nil {
			continue
		}
		for _, ask := range render.Asks(m, scan) {
			existing, ok := byName[ask.Name]
			if !ok {
				byName[ask.Name] = ask
				continue
			}
			existing.PerEnv = existing.PerEnv || ask.PerEnv
			byName[ask.Name] = existing
		}
	}
	asks := make([]render.Ask, 0, len(byName))
	for _, ask := range byName {
		asks = append(asks, ask)
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Name < asks[j].Name })
	return asks
}

func finish(templates string, m *types.Manifest, a render.Answers, yes, bootstrap bool) {
	if a.Dir == "" {
		a.Dir = "./" + a.Project
	}
	if err := render.TargetOK(a.Dir); err != nil {
		fatal(err)
	}
	p, err := render.Build(templates, m, a)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Plan: %s → %s\n%s", a.Project, a.Dir, p.Tree())
	if !yes {
		fmt.Println("\nplan only; add --yes to apply")
		return
	}
	events := make(chan render.Step, len(render.Units))
	done := make(chan error, 1)
	go func() { done <- p.Apply(a.Dir, events); close(events) }()
	for s := range events {
		fmt.Printf("✓ %s (%d files)\n", s.Unit, s.Files)
	}
	if err := <-done; err != nil {
		fmt.Printf("✗ %v\n", err)
		fmt.Printf("the directory %s is incomplete; delete it before retrying\n", a.Dir)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Print(cli.Checklist(p))
	if bootstrap {
		if err := cli.RunBootstrap(p, a.Dir); err != nil {
			fatal(err)
		}
	} else {
		fmt.Print(cli.PrintBootstrap(p))
	}
}

func runTUI(templates string, m *types.Manifest, shapes []string, a render.Answers, vars varFlags, yes, bootstrap bool) {
	if err := cli.Infer(m, &a); err != nil {
		fatal(err)
	}
	if a.Environments != "" && len(vars) > 0 {
		scan, err := render.Build(templates, m, a)
		if err != nil {
			fatal(err)
		}
		parsed, err := cli.ParseVars(vars, scan.Envs)
		if err != nil {
			fatal(err)
		}
		a.Vars = parsed
	}
	p, err := wizard.Run(templates, m, shapes, a, yes)
	if err != nil {
		fatal(err)
	}
	fmt.Println()
	fmt.Print(cli.Checklist(p))
	if bootstrap {
		if err := cli.RunBootstrap(p, p.Dir); err != nil {
			fatal(err)
		}
	} else {
		fmt.Print(cli.PrintBootstrap(p))
	}
}

func runFind(args []string) {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	templates := fs.String("templates", ".", "path to a cicd-templates checkout")
	fs.Parse(args)
	m, shapes := load(*templates)
	rows, err := cli.Filter(cli.Rows(m), fs.Args())
	if err != nil {
		fatal(err)
	}
	switch len(rows) {
	case 0:
		fmt.Println("no rows match")
		os.Exit(1)
	case 1:
		card, err := cli.Card(*templates, m, shapes, rows[0])
		if err != nil {
			fatal(err)
		}
		fmt.Print(card)
	default:
		fmt.Print(cli.Table(rows))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "odyssey-cli:", err)
	os.Exit(1)
}
