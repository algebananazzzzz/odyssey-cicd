package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/types"
)

// An odyssey token is {{UPPER}} not preceded by $ — "${{ }}" belongs to GitHub.
var tokenRe = regexp.MustCompile(`(\$?)\{\{([A-Z][A-Z0-9_]*)\}\}`)

type Answers struct {
	Provider     string
	Architecture string
	Stack        string
	Environments string
	Project      string
	Dir          string
	Vars         map[string]map[string]string
}

type File struct {
	Path   string
	Body   string
	Mode   fs.FileMode
	Unit   string
	PerEnv bool
}

type Plan struct {
	Dir       string
	Envs      []string
	Files     []File
	Github    types.Github
	Bootstrap *types.Bootstrap
}

func Shapes(templates string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(templates, "fragments/workflows/deploy"))
	if err != nil {
		return nil, err
	}
	var shapes []string
	for _, e := range entries {
		if e.IsDir() {
			shapes = append(shapes, e.Name())
		}
	}
	sort.Strings(shapes)
	return shapes, nil
}

func Envs(frag fs.FS, shape string) ([]string, error) {
	entries, err := fs.ReadDir(frag, "workflows/deploy/"+shape)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yml") {
			seen[deployEnv(e.Name())] = true
		}
	}
	var envs []string
	for env := range seen {
		envs = append(envs, env)
	}
	sort.Slice(envs, func(i, j int) bool {
		if envs[i] == "preprod" {
			return true
		}
		if envs[j] == "preprod" {
			return false
		}
		return envs[i] < envs[j]
	})
	return envs, nil
}

func deployEnv(name string) string {
	if strings.Contains(name, "preprod") {
		return "preprod"
	}
	return "prd"
}

type renderer struct {
	frag  fs.FS
	envs  []string
	a     Answers
	files []File
}

func Build(templates string, m *types.Manifest, a Answers) (*Plan, error) {
	frag := os.DirFS(filepath.Join(templates, "fragments"))
	envs, err := Envs(frag, a.Environments)
	if err != nil {
		return nil, err
	}
	r := &renderer{frag: frag, envs: envs, a: a}
	if err := r.copyDir("workflows/scripts", ".github/scripts", "workflows"); err != nil {
		return nil, err
	}
	if err := r.workflows(); err != nil {
		return nil, err
	}
	if err := r.compose(m); err != nil {
		return nil, err
	}
	p := &Plan{Dir: a.Dir, Envs: envs, Files: r.files}
	p.Github, p.Bootstrap = metadata(m, a)
	return p, nil
}

func metadata(m *types.Manifest, a Answers) (types.Github, *types.Bootstrap) {
	var g types.Github
	for _, spec := range []types.Spec{
		m.Providers[types.Provider(a.Provider)],
		m.Architectures[types.Architecture(a.Architecture)],
		m.Stacks[types.Stack(a.Stack)],
	} {
		g.Variables = append(g.Variables, spec.Github.Variables...)
		g.Secrets = append(g.Secrets, spec.Github.Secrets...)
	}
	sort.Strings(g.Variables)
	sort.Strings(g.Secrets)
	return g, m.Stacks[types.Stack(a.Stack)].Bootstrap
}

func (r *renderer) subst(text, env string) string {
	text = strings.ReplaceAll(text, "{{ENV_LIST}}", `["`+strings.Join(r.envs, `", "`)+`"]`)
	if env != "" {
		text = strings.ReplaceAll(text, "{{ENV}}", env)
	}
	lookup := env
	if lookup == "" {
		lookup = r.envs[0]
	}
	vars := r.a.Vars[lookup]
	engine := map[string]string{"PROJECT": r.a.Project, "PREPROD_ENV": "preprod", "PRD_ENV": "prd"}
	return tokenRe.ReplaceAllStringFunc(text, func(tok string) string {
		sub := tokenRe.FindStringSubmatch(tok)
		if sub[1] == "$" {
			return tok
		}
		if v, ok := engine[sub[2]]; ok {
			return v
		}
		if v, ok := vars[sub[2]]; ok {
			return v
		}
		return tok
	})
}

func (r *renderer) emit(path, body, unit, env string, perEnv bool) {
	mode := fs.FileMode(0o644)
	if strings.HasSuffix(path, ".sh") {
		mode = 0o755
	}
	r.files = append(r.files, File{Path: path, Body: r.subst(body, env), Mode: mode, Unit: unit, PerEnv: perEnv})
}

func (r *renderer) appendTo(path, body, unit, env string, perEnv bool) {
	for i, f := range r.files {
		if f.Path == path {
			r.files[i].Body = f.Body + "\n" + r.subst(body, env)
			return
		}
	}
	r.emit(path, body, unit, env, perEnv)
}

func (r *renderer) read(path string) (string, error) {
	data, err := fs.ReadFile(r.frag, path)
	return string(data), err
}

func (r *renderer) copyDir(src, dst, unit string) error {
	return fs.WalkDir(r.frag, src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := r.read(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		r.emit(filepath.ToSlash(filepath.Join(dst, rel)), text, unit, "", false)
		return nil
	})
}

func markerRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)[ \t]*# >>> ` + name + `\n(.*?)[ \t]*# <<< ` + name + `\n`)
}

func fillMarker(text, name, body string, keep bool) string {
	re := markerRe(name)
	return re.ReplaceAllStringFunc(text, func(m string) string {
		if keep {
			return re.FindStringSubmatch(m)[1]
		}
		return body
	})
}

func (r *renderer) workflows() error {
	ciInfra, deployInfra := "", ""
	if r.a.Provider != "" {
		var err error
		if ciInfra, err = r.read("workflows/ci/infra.yml"); err != nil {
			return err
		}
		if deployInfra, err = r.read("infra/providers/" + r.a.Provider + "/workflows/deploy.yml"); err != nil {
			return err
		}
	}
	for dir, infraBody := range map[string]string{
		"workflows/ci":                            ciInfra,
		"workflows/deploy/" + r.a.Environments: deployInfra,
	} {
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
			env := ""
			if strings.HasPrefix(dir, "workflows/deploy/") {
				env = deployEnv(e.Name())
			}
			r.emit(".github/workflows/"+e.Name(), text, "workflows", env, false)
		}
	}
	return nil
}

func (r *renderer) compose(m *types.Manifest) error {
	return nil
}
