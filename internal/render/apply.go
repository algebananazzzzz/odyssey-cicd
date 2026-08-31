package render

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Step struct {
	Unit  string
	Files int
}

var Units = []string{"workflows", "makefile", "stack", "infra", "docs"}

func TargetOK(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s exists and is not empty", dir)
	}
	return nil
}

func (p *Plan) Apply(dir string, events chan<- Step) error {
	if left := p.Unresolved(); len(left) > 0 {
		return fmt.Errorf("unresolved variables: %s", strings.Join(left, ", "))
	}
	for _, unit := range Units {
		n := 0
		for _, f := range p.sorted() {
			if f.Unit != unit {
				continue
			}
			path := filepath.Join(dir, f.Path)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(f.Body), f.Mode); err != nil {
				return err
			}
			n++
		}
		if events != nil {
			events <- Step{Unit: unit, Files: n}
		}
	}
	return nil
}

func (p *Plan) sorted() []File {
	files := append([]File(nil), p.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func (p *Plan) Tree() string {
	groups := map[string]int{}
	var singles []string
	for _, f := range p.Files {
		top, rest, nested := strings.Cut(f.Path, "/")
		if !nested {
			singles = append(singles, top)
			continue
		}
		if top == ".github" {
			if sub, _, deeper := strings.Cut(rest, "/"); deeper {
				top = top + "/" + sub
			}
		}
		groups[top+"/"]++
	}
	var keys []string
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sort.Strings(singles)
	var b strings.Builder
	fmt.Fprintf(&b, "%d files\n", len(p.Files))
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-22s %d files\n", k, groups[k])
	}
	for _, s := range singles {
		fmt.Fprintf(&b, "  %s\n", s)
	}
	return b.String()
}
