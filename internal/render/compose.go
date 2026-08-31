package render

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/types"
)

func (r *renderer) compose(m *types.Manifest) error {
	if err := r.makefile(); err != nil {
		return err
	}
	if err := r.stack(); err != nil {
		return err
	}
	if err := r.deployExtras(); err != nil {
		return err
	}
	if r.a.Provider != "" {
		if err := r.infra(); err != nil {
			return err
		}
	}
	return r.docs()
}

func (r *renderer) makefile() error {
	parts := []string{"makefile/base.mk", "makefile/stack/" + r.a.Stack + "/main.mk", "makefile/deploy/" + r.a.Architecture + "/main.mk"}
	if r.a.Provider != "" {
		parts = append(parts, "makefile/infra/terraform/main.mk")
	}
	for _, p := range parts {
		text, err := r.read(p)
		if err != nil {
			return err
		}
		r.appendTo("Makefile", text, "makefile", "", false)
	}
	return nil
}

func (r *renderer) stack() error {
	base := "stacks/" + r.a.Stack
	if err := r.copyDir(base+"/.github", ".github", "stack"); err != nil {
		return err
	}
	if _, err := fs.Stat(r.frag, base+"/files"); err == nil {
		return r.copyDir(base+"/files", ".", "stack")
	}
	return nil
}

func (r *renderer) deployExtras() error {
	base := "makefile/deploy/" + r.a.Architecture
	if _, err := fs.Stat(r.frag, base+"/scripts"); err == nil {
		if err := r.copyDir(base+"/scripts", "scripts", "makefile"); err != nil {
			return err
		}
	}
	if _, err := fs.Stat(r.frag, base+"/files"); err == nil {
		return r.copyDir(base+"/files", ".", "stack")
	}
	return nil
}

func (r *renderer) infra() error {
	gi, err := r.read("infra/.gitignore")
	if err != nil {
		return err
	}
	r.emit("infra/.gitignore", gi, "infra", "", false)

	pdir := "infra/providers/" + r.a.Provider
	err = fs.WalkDir(r.frag, pdir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(path, "/workflows/") {
			return err
		}
		rel, _ := filepath.Rel(pdir, path)
		text, rerr := r.read(path)
		if rerr != nil {
			return rerr
		}
		r.perEnv("infra/"+filepath.ToSlash(rel), text, false)
		return nil
	})
	if err != nil {
		return err
	}

	adir := "infra/architecture/" + r.a.Architecture
	if _, err := fs.Stat(r.frag, adir); err != nil {
		return nil
	}
	return fs.WalkDir(r.frag, adir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, adir+"/"))
		text, rerr := r.read(path)
		if rerr != nil {
			return rerr
		}
		switch {
		case rel == "variables.tf":
			r.appendTo("infra/variables.tf", text, "infra", "", false)
		case strings.HasPrefix(rel, "config/"):
			r.perEnv("infra/"+rel, text, true)
		default:
			r.perEnv("infra/"+rel, text, false)
		}
		return nil
	})
}

func (r *renderer) perEnv(dst, text string, appendMode bool) {
	if !strings.Contains(dst, "{{ENV}}") {
		if appendMode {
			r.appendTo(dst, text, "infra", "", false)
		} else {
			r.emit(dst, text, "infra", "", false)
		}
		return
	}
	for _, env := range r.envs {
		path := strings.ReplaceAll(dst, "{{ENV}}", env)
		if appendMode {
			r.appendTo(path, text, "infra", env, true)
		} else {
			r.emit(path, text, "infra", env, true)
		}
	}
}

func (r *renderer) docs() error {
	sources := []string{
		"stacks/" + r.a.Stack + "/context.md",
		"makefile/deploy/" + r.a.Architecture + "/context.md",
	}
	if r.a.Provider != "" {
		sources = append(sources, "infra/context.md")
	}
	var parts []string
	for _, src := range sources {
		if _, err := fs.Stat(r.frag, src); err != nil {
			continue
		}
		text, err := r.read(src)
		if err != nil {
			return err
		}
		parts = append(parts, strings.TrimRight(text, "\n"))
	}
	r.emit("AGENTS.md", strings.Join(parts, "\n\n")+"\n", "docs", "", false)
	r.emit("CLAUDE.md", "@AGENTS.md\n", "docs", "", false)
	return nil
}
