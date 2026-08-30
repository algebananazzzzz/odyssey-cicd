package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Selection is one answer per wizard question.
type Selection struct {
	Environments string // single | dual
	Provider     string // empty: no infra
	Architecture string
	Stack        string
}

// Token values; Render substitutes every {{TOKEN}} it finds. ENV and ENV_LIST
// are derived, never supplied.
type Tokens map[string]string

// {{ENV}} in a deploy workflow means that workflow's environment.
var workflowEnv = map[string]string{
	"3-deploy.yml":         "prd",
	"3-deploy-preprod.yml": "preprod",
	"4-deploy-prd.yml":     "prd",
}

// An odyssey token is {{UPPER}} not preceded by $ — "${{ }}" belongs to GitHub.
var tokenRe = regexp.MustCompile(`(\$?)\{\{([A-Z0-9_]+)\}\}`)

type renderer struct {
	frag   fs.FS
	out    string
	envs   []string
	tokens Tokens
}

// Render writes the selected fragments to outDir. The fragment layout is the
// contract: every path the renderer touches means one thing.
func Render(frag fs.FS, sel Selection, envs []string, tokens Tokens, outDir string) error {
	r := &renderer{frag: frag, out: outDir, envs: envs, tokens: tokens}

	if err := r.copyDir("workflows/scripts", ".github/scripts"); err != nil {
		return err
	}
	if err := r.workflows(sel); err != nil {
		return err
	}
	if err := r.makefile(sel); err != nil {
		return err
	}
	if err := r.stack(sel.Stack); err != nil {
		return err
	}
	if err := r.architectureExtras(sel.Architecture); err != nil {
		return err
	}
	if sel.Provider != "" {
		if err := r.infra(sel); err != nil {
			return err
		}
	}
	return nil
}

// UnresolvedTokens lists the distinct {{TOKEN}} names under the fragment tree
// for this selection, so the caller can ask for exactly those.
func UnresolvedTokens(frag fs.FS, sel Selection, envs []string) ([]string, error) {
	dir, err := os.MkdirTemp("", "odyssey-scan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := Render(frag, sel, envs, nil, dir); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range tokenRe.FindAllStringSubmatch(string(data), -1) {
			if m[1] == "" {
				seen[m[2]] = true
			}
		}
		for _, m := range tokenRe.FindAllStringSubmatch(filepath.Base(path), -1) {
			seen[m[2]] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	delete(seen, "ENV")
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

func (r *renderer) subst(text, env string) string {
	text = strings.ReplaceAll(text, "{{ENV_LIST}}", `["`+strings.Join(r.envs, `", "`)+`"]`)
	if env != "" {
		text = strings.ReplaceAll(text, "{{ENV}}", env)
	}
	return tokenRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := tokenRe.FindStringSubmatch(m)
		if sub[1] == "$" {
			return m
		}
		if v, ok := r.tokens[sub[2]]; ok {
			return v
		}
		return m
	})
}

func (r *renderer) write(dst, text, env string) error {
	path := filepath.Join(r.out, dst)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(r.subst(text, env)), perm(dst))
}

func perm(path string) os.FileMode {
	if strings.HasSuffix(path, ".sh") {
		return 0o755
	}
	return 0o644
}

func (r *renderer) read(path string) (string, error) {
	data, err := fs.ReadFile(r.frag, path)
	return string(data), err
}

func (r *renderer) copyDir(src, dst string) error {
	return fs.WalkDir(r.frag, src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := r.read(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		return r.write(filepath.Join(dst, rel), text, "")
	})
}

func (r *renderer) appendFile(src, dst string) error {
	text, err := r.read(src)
	if err != nil {
		return err
	}
	path := filepath.Join(r.out, dst)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > 0 {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(r.subst(text, ""))
	return err
}

var markerRe = func(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)[ \t]*# >>> ` + name + `\n(.*?)[ \t]*# <<< ` + name + `\n`)
}

// fillMarker replaces the marker's body (empty body when nothing is selected,
// keep=true keeps the default) and strips the marker lines.
func fillMarker(text, name, body string, keep bool) string {
	re := markerRe(name)
	return re.ReplaceAllStringFunc(text, func(m string) string {
		if keep {
			return re.FindStringSubmatch(m)[1]
		}
		return body
	})
}

func (r *renderer) workflows(sel Selection) error {
	ciInfra, deployInfra := "", ""
	if sel.Provider != "" {
		var err error
		if ciInfra, err = r.read("workflows/ci/infra.yml"); err != nil {
			return err
		}
		if deployInfra, err = r.read("infra/providers/" + sel.Provider + "/workflows/deploy.yml"); err != nil {
			return err
		}
	}
	dirs := map[string]string{"workflows/ci": ciInfra, "workflows/deploy/" + sel.Environments: deployInfra}
	for dir, infraBody := range dirs {
		entries, err := fs.ReadDir(r.frag, dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") || e.Name() == "infra.yml" {
				continue
			}
			text, err := r.read(dir + "/" + e.Name())
			if err != nil {
				return err
			}
			text = fillMarker(text, "infra", infraBody, false)
			text = fillMarker(text, "deploy", "", true)
			if err := r.write(filepath.Join(".github/workflows", e.Name()), text, workflowEnv[e.Name()]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *renderer) makefile(sel Selection) error {
	parts := []string{"makefile/base.mk", "makefile/stack/" + sel.Stack + "/main.mk", "makefile/deploy/" + sel.Architecture + "/main.mk"}
	if sel.Provider != "" {
		parts = append(parts, "makefile/infra/terraform/main.mk")
	}
	for _, p := range parts {
		if err := r.appendFile(p, "Makefile"); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) stack(stack string) error {
	base := "stacks/" + stack
	if err := r.copyDir(base+"/.github", ".github"); err != nil {
		return err
	}
	if _, err := fs.Stat(r.frag, base+"/files"); err == nil {
		return r.copyDir(base+"/files", ".")
	}
	return nil
}

func (r *renderer) architectureExtras(arch string) error {
	base := "makefile/deploy/" + arch
	if _, err := fs.Stat(r.frag, base+"/scripts"); err == nil {
		if err := r.copyDir(base+"/scripts", "scripts"); err != nil {
			return err
		}
	}
	if _, err := fs.Stat(r.frag, base+"/files"); err == nil {
		return r.copyDir(base+"/files", ".")
	}
	return nil
}

func (r *renderer) infra(sel Selection) error {
	if err := r.appendFile("infra/.gitignore", "infra/.gitignore"); err != nil {
		return err
	}
	pdir := "infra/providers/" + sel.Provider
	err := fs.WalkDir(r.frag, pdir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(path, "/workflows/") {
			return err
		}
		rel, _ := filepath.Rel(pdir, path)
		text, rerr := r.read(path)
		if rerr != nil {
			return rerr
		}
		return r.writePerEnv(filepath.Join("infra", rel), text)
	})
	if err != nil {
		return err
	}

	adir := "infra/architecture/" + sel.Architecture
	if _, err := fs.Stat(r.frag, adir); err != nil {
		return nil
	}
	return fs.WalkDir(r.frag, adir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(adir, path)
		text, rerr := r.read(path)
		if rerr != nil {
			return rerr
		}
		switch {
		case rel == "variables.tf":
			return r.appendFile(path, "infra/variables.tf")
		case strings.HasPrefix(rel, "config/"):
			for _, env := range r.envs {
				dst := filepath.Join("infra", strings.ReplaceAll(rel, "{{ENV}}", env))
				if err := r.appendText(dst, r.subst(text, env)); err != nil {
					return err
				}
			}
			return nil
		default:
			return r.writePerEnv(filepath.Join("infra", rel), text)
		}
	})
}

// writePerEnv writes once, or once per environment when {{ENV}} is in the name.
func (r *renderer) writePerEnv(dst, text string) error {
	if !strings.Contains(dst, "{{ENV}}") {
		return r.write(dst, text, "")
	}
	for _, env := range r.envs {
		if err := r.write(strings.ReplaceAll(dst, "{{ENV}}", env), text, env); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) appendText(dst, text string) error {
	path := filepath.Join(r.out, dst)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > 0 {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(text)
	return err
}

func (m *Manifest) Validate(sel Selection) error {
	if _, ok := m.Environments[sel.Environments]; !ok {
		return fmt.Errorf("unknown environments %q", sel.Environments)
	}
	arch, ok := m.Architectures[sel.Architecture]
	if !ok {
		return fmt.Errorf("unknown architecture %q", sel.Architecture)
	}
	if sel.Provider == "" && arch.Infra != "optional" {
		return fmt.Errorf("architecture %q requires provider %q", sel.Architecture, arch.Provider)
	}
	if sel.Provider != "" && sel.Provider != arch.Provider {
		return fmt.Errorf("architecture %q runs on %q, not %q", sel.Architecture, arch.Provider, sel.Provider)
	}
	archs, ok := m.Stacks[sel.Stack]
	if !ok {
		return fmt.Errorf("unknown stack %q", sel.Stack)
	}
	for _, a := range archs {
		if a == sel.Architecture {
			return nil
		}
	}
	return fmt.Errorf("stack %q does not target %q", sel.Stack, sel.Architecture)
}
