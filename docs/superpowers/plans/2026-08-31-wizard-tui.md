# Wizard TUI and Headless CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full `odyssey-cli new` flow: render engine, headless mode with inference and teaching output, `find` discovery, and the five-page huh-in-bubbletea TUI.

**Architecture:** A pure in-memory render engine (`internal/render`) produces a `Plan` from an `Answers` value and applies it while streaming steps; `internal/cli` builds headless behavior (inference, teaching output, find, checklist) on top of it; `internal/wizard` is rewritten as a page-machine Bubble Tea model embedding one huh form per page over the same engine calls.

**Tech Stack:** Go 1.25, std `flag`, charmbracelet bubbletea v1 + lipgloss (present), huh + bubbles (added), yaml.v3.

**Spec:** `docs/superpowers/specs/2026-08-31-wizard-tui-design.md`. A prior engine implementation exists in git history and is the reference for render mechanics: view it with `git show b4acdd0:internal/engine/render.go`. It was written against an older manifest schema; the code in this plan is already adapted to the current one, so copy from the plan, consult history only to understand intent.

## Global Constraints

- Naming: the spec's `render.Plan(...)` function collides with the `Plan` type in Go, so the constructor is `render.Build(templates string, a Answers) (*Plan, error)`. Everything else keeps spec names.
- No new code comments. Rationale goes in ROADMAP.md or the spec (user rule, recorded in project memory). Code below is intentionally comment-free except two pre-existing regex explanations carried over from the old engine.
- Engine variables are exactly `PROJECT`, `ENV`, `ENV_LIST`, `PREPROD_ENV`, `PRD_ENV`. Environment names are fixed: `preprod` and `prd`. A deploy workflow renders with `ENV=preprod` when its filename contains `preprod`, else `prd`.
- Token regex: `(\$?)\{\{([A-Z][A-Z0-9_]*)\}\}`; a `$` prefix means a GitHub expression, never substituted.
- Marker blocks: `# >>> <name>` ... `# <<< <name>`; markers are `infra` and `deploy`. Infra body replaces, deploy keeps its default, marker lines always stripped.
- Exit codes: 0 success (including headless plan-only), 1 failure or abort, 2 incomplete headless answers.
- Std `flag` with one `flag.FlagSet` per subcommand, no cobra.
- Commits: imperative subject, conventional prefix (`feat(cli):`, `docs:`), no footers.
- Run `gofmt -l` cleanliness implicitly via `go vet ./...` + `go test ./...` before every commit (CI runs both).

## File Structure

| File | Responsibility |
|---|---|
| `internal/render/render.go` | `Answers`, `File`, `Plan` types; `Shapes`, `Envs`; substitution, markers; workflows + scripts rendering |
| `internal/render/compose.go` | Makefile concatenation, stack copy, deploy-axis extras, infra per-env, AGENTS.md + CLAUDE.md |
| `internal/render/asks.go` | `Ask` discovery from an unsubstituted plan + manifest; `Plan.Unresolved` |
| `internal/render/apply.go` | `Step`, `Plan.Apply`, `TargetOK`, `Plan.Tree` |
| `internal/cli/headless.go` | var-flag parsing, axis inference, missing report (teaching output) |
| `internal/cli/output.go` | checklist and bootstrap printing |
| `internal/cli/find.go` | rows, filtering, table and detail card |
| `internal/wizard/wizard.go` | parent model, page machine, Run |
| `internal/wizard/forms.go` | one huh.Form builder per page |
| `internal/wizard/theme.go` | huh theme, lipgloss styles, status panel |
| `internal/types/types.go` | add `Bootstrap` |
| `cmd/odyssey-cli/main.go` | subcommand dispatch, TTY detection, closing output (moved from `cmd/main.go`) |

Tests sit beside each package (`*_test.go`). Engine tests run against the real `fragments/` tree and `manifest.yml` at the repo root, resolved as `../..` from the package dir.

---

### Task 1: Render engine core: types, substitution, workflows

**Files:**
- Create: `internal/render/render.go`
- Test: `internal/render/render_test.go`

**Interfaces:**
- Consumes: `types.Manifest` (existing), the fragment tree layout.
- Produces: `type Answers struct { Provider, Architecture, Stack, Environments, Project, Dir string; Vars map[string]map[string]string }` (Vars is env → name → value); `type File struct { Path, Body string; Mode fs.FileMode; Unit string; PerEnv bool }`; `type Plan struct { Dir string; Envs []string; Files []File; Github types.Github; Bootstrap *types.Bootstrap }`; `func Shapes(templates string) ([]string, error)`; `func Envs(frag fs.FS, shape string) ([]string, error)`; `func Build(templates string, m *types.Manifest, a Answers) (*Plan, error)`. Units emitted here: `workflows`.

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"strings"
	"testing"

	"github.com/algebananazzzzz/odyssey/internal/validate"
)

const templates = "../.."

func answers() Answers {
	return Answers{
		Provider: "cloudflare", Architecture: "cloudflare-worker",
		Stack: "nextjs", Environments: "dual", Project: "acme-web", Dir: "./acme-web",
		Vars: map[string]map[string]string{
			"preprod": {"PREPROD_URL": "https://pre.example.com", "PRD_URL": "https://example.com", "CUSTOM_DOMAIN": "pre.example.com"},
			"prd":     {"PREPROD_URL": "https://pre.example.com", "PRD_URL": "https://example.com", "CUSTOM_DOMAIN": "example.com"},
		},
	}
}

func testBuild(t *testing.T, a Answers) *Plan {
	t.Helper()
	m, err := validate.Manifest(templates + "/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(templates, m, a)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func file(t *testing.T, p *Plan, path string) File {
	t.Helper()
	for _, f := range p.Files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("plan has no %s; have %v", path, paths(p))
	return File{}
}

func paths(p *Plan) []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, f.Path)
	}
	return out
}

func TestShapesAndEnvs(t *testing.T) {
	shapes, err := Shapes(templates)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"dual", "single"}; !equal(shapes, want) {
		t.Fatalf("shapes = %v, want %v", shapes, want)
	}
	p := testBuild(t, answers())
	if want := []string{"preprod", "prd"}; !equal(p.Envs, want) {
		t.Fatalf("envs = %v, want %v", p.Envs, want)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWorkflowsRendered(t *testing.T) {
	p := testBuild(t, answers())
	for _, path := range []string{
		".github/workflows/1-ci-branch.yml",
		".github/workflows/2-ci-merge.yml",
		".github/workflows/3-deploy-preprod.yml",
		".github/workflows/4-deploy-prd.yml",
		".github/scripts/semver-tag.sh",
		".github/scripts/promote-tag.sh",
	} {
		f := file(t, p, path)
		if f.Unit != "workflows" {
			t.Fatalf("%s unit = %q, want workflows", path, f.Unit)
		}
	}
	merge := file(t, p, ".github/workflows/2-ci-merge.yml").Body
	if strings.Contains(merge, "# >>>") || strings.Contains(merge, "# <<<") {
		t.Fatalf("marker lines survived in 2-ci-merge.yml")
	}
	if !strings.Contains(merge, "infra-check") {
		t.Fatalf("ci infra marker body not injected:\n%s", merge)
	}
	pre := file(t, p, ".github/workflows/3-deploy-preprod.yml").Body
	if !strings.Contains(pre, `name: "preprod"`) {
		t.Fatalf("PREPROD_ENV not substituted in preprod deploy")
	}
	if strings.Contains(pre, "{{") && !strings.Contains(pre, "${{") {
		t.Fatalf("unsubstituted odyssey token in preprod deploy")
	}
	if !strings.Contains(pre, "${{") {
		t.Fatalf("github expressions must survive substitution")
	}
	if f := file(t, p, ".github/scripts/semver-tag.sh"); f.Mode != 0o755 {
		t.Fatalf("script mode = %o, want 755", f.Mode)
	}
}

func TestNoProviderDeletesInfraMarkers(t *testing.T) {
	a := answers()
	a.Provider = ""
	p := testBuild(t, a)
	merge := file(t, p, ".github/workflows/2-ci-merge.yml").Body
	if strings.Contains(merge, "infra-check") || strings.Contains(merge, "# >>>") {
		t.Fatalf("no-provider render kept infra content:\n%s", merge)
	}
}
```

Note on `TestWorkflowsRendered`'s marker-body assertion: confirm `fragments/workflows/ci/infra.yml` actually contains `infra-check` before finalizing the string (`grep infra-check fragments/workflows/ci/infra.yml`); if it differs, assert on a stable line from that file instead.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run 'TestShapes|TestWorkflows|TestNoProvider' -v`
Expected: FAIL to build, `undefined: Build` etc.

- [ ] **Step 3: Write the implementation**

`internal/render/render.go`:

```go
package render

import (
	"fmt"
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
	sort.Strings(envs)
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
```

Add a temporary stub so the package compiles before Task 2:

```go
func (r *renderer) compose(m *types.Manifest) error {
	return nil
}
```

And in `internal/types/types.go` add (this is Task 3's `Bootstrap` consumer arriving early so `Plan` compiles; the yaml wiring is complete already):

```go
type Bootstrap struct {
	Intent   string   `yaml:"intent"`
	Commands []string `yaml:"commands"`
}
```

and add to `Spec`:

```go
	Bootstrap *Bootstrap `yaml:"bootstrap,omitempty"`
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
git add internal/render internal/types/types.go
git commit -m "feat(render): in-memory plan engine, workflows and scripts"
```

---

### Task 2: Render engine composition: makefile, stack, infra, docs

**Files:**
- Modify: `internal/render/render.go` (delete the `compose` stub)
- Create: `internal/render/compose.go`
- Test: `internal/render/compose_test.go`

**Interfaces:**
- Consumes: `renderer` internals from Task 1 (`emit`, `appendTo`, `read`, `copyDir`, `subst`).
- Produces: plan files for units `makefile`, `stack`, `infra`, `docs`. `Makefile` at root; `scripts/`, root files from makefile axes; `infra/` per-env; `AGENTS.md` and `CLAUDE.md` (body exactly `@AGENTS.md\n`).

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"strings"
	"testing"
)

func TestMakefileOrder(t *testing.T) {
	p := testBuild(t, answers())
	mk := file(t, p, "Makefile")
	if mk.Unit != "makefile" {
		t.Fatalf("Makefile unit = %q", mk.Unit)
	}
	iBase := strings.Index(mk.Body, "task runner")
	iStack := strings.Index(mk.Body, "next build")
	iDeploy := strings.Index(mk.Body, "wrangler")
	iInfra := strings.Index(mk.Body, "infra-init")
	if iBase == -1 || iStack == -1 || iDeploy == -1 || iInfra == -1 {
		t.Fatalf("missing sections: base=%d stack=%d deploy=%d infra=%d\n%s", iBase, iStack, iDeploy, iInfra, mk.Body)
	}
	if !(iBase < iStack && iStack < iDeploy && iDeploy < iInfra) {
		t.Fatalf("order wrong: base=%d stack=%d deploy=%d infra=%d", iBase, iStack, iDeploy, iInfra)
	}
	if strings.Contains(mk.Body, "{{PROJECT}}") {
		t.Fatalf("PROJECT not substituted in Makefile")
	}
}

func TestInfraPerEnv(t *testing.T) {
	p := testBuild(t, answers())
	for _, path := range []string{
		"infra/backend.tf", "infra/providers.tf", "infra/variables.tf", "infra/.gitignore",
		"infra/config/preprod.tfvars", "infra/config/prd.tfvars",
		"infra/config/preprod.tfbackend", "infra/config/prd.tfbackend",
		"infra/cache.tf", "infra/domain.tf",
	} {
		if f := file(t, p, path); f.Unit != "infra" {
			t.Fatalf("%s unit = %q", path, f.Unit)
		}
	}
	tv := file(t, p, "infra/config/preprod.tfvars")
	if !tv.PerEnv {
		t.Fatalf("env-expanded file not marked PerEnv")
	}
	if !strings.Contains(tv.Body, `env`) || !strings.Contains(tv.Body, `"preprod"`) {
		t.Fatalf("env not substituted:\n%s", tv.Body)
	}
	if !strings.Contains(tv.Body, "pre.example.com") {
		t.Fatalf("per-env CUSTOM_DOMAIN missing in preprod tfvars:\n%s", tv.Body)
	}
	if prd := file(t, p, "infra/config/prd.tfvars"); !strings.Contains(prd.Body, "worker_name") {
		t.Fatalf("architecture tfvars not appended to provider tfvars:\n%s", prd.Body)
	}
	vars := file(t, p, "infra/variables.tf")
	if !strings.Contains(vars.Body, "project_code") || !strings.Contains(vars.Body, "custom_domain") {
		t.Fatalf("architecture variables.tf not appended:\n%s", vars.Body)
	}
	if !strings.Contains(vars.Body, `["preprod", "prd"]`) {
		t.Fatalf("ENV_LIST not expanded:\n%s", vars.Body)
	}
}

func TestStackAndDocs(t *testing.T) {
	p := testBuild(t, answers())
	if f := file(t, p, ".github/actions/setup/action.yml"); f.Unit != "stack" {
		t.Fatalf("setup action unit = %q", f.Unit)
	}
	agents := file(t, p, "AGENTS.md")
	if agents.Unit != "docs" {
		t.Fatalf("AGENTS.md unit = %q", agents.Unit)
	}
	if !strings.Contains(agents.Body, "infra") {
		t.Fatalf("AGENTS.md missing infra context:\n%s", agents.Body)
	}
	if strings.Contains(agents.Body, "{{PROJECT}}") {
		t.Fatalf("AGENTS.md tokens not substituted")
	}
	if claude := file(t, p, "CLAUDE.md"); claude.Body != "@AGENTS.md\n" {
		t.Fatalf("CLAUDE.md body = %q", claude.Body)
	}
}

func TestAwsStackFiles(t *testing.T) {
	a := answers()
	a.Provider, a.Architecture, a.Stack = "aws", "aws-ecs", "go-service"
	a.Vars = nil
	p := testBuild(t, a)
	if f := file(t, p, "Dockerfile"); f.Unit != "stack" {
		t.Fatalf("Dockerfile unit = %q", f.Unit)
	}
	file(t, p, "taskdef.json")
	if f := file(t, p, "scripts/deploy-ecs.sh"); f.Mode != 0o755 {
		t.Fatalf("deploy-ecs.sh mode = %o", f.Mode)
	}
}
```

The `iStack`/`iDeploy` needles assume `makefile/stack/nextjs/main.mk` contains `next build` and `makefile/deploy/cloudflare-worker/main.mk` contains `wrangler`; verify with grep and adjust the needle to a real line before running.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run 'TestMakefile|TestInfra|TestStack|TestAws' -v`
Expected: FAIL, `plan has no Makefile`.

- [ ] **Step 3: Write the implementation**

Delete the `compose` stub from `render.go`. Create `internal/render/compose.go`:

```go
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
```

One subtlety carried from the old engine: `appendTo` on a `{{ENV}}`-named architecture config must land on the provider's already-emitted per-env file (`infra/config/preprod.tfvars`), which it does because `perEnv` expands the name before calling `appendTo`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): makefile, stack, infra and docs composition"
```

---

### Task 3: Ask discovery and the all-combinations invariant

**Files:**
- Create: `internal/render/asks.go`
- Test: `internal/render/asks_test.go`

**Interfaces:**
- Consumes: `Build`, `Plan`, `tokenRe`, `types.Manifest`.
- Produces: `type Ask struct { Name string; Optional, PerEnv bool }`; `func Asks(m *types.Manifest, p *Plan) []Ask` (sorted by name); `func (p *Plan) Unresolved() []string` (sorted distinct non-`$` token names left in bodies).

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"testing"

	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/validate"
)

func TestAsksForNextjs(t *testing.T) {
	m, err := validate.Manifest(templates + "/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	a := answers()
	a.Vars = nil
	p := testBuild(t, a)
	asks := Asks(m, p)
	want := []Ask{
		{Name: "CUSTOM_DOMAIN", Optional: true, PerEnv: true},
		{Name: "PRD_URL", Optional: true},
		{Name: "PREPROD_URL", Optional: true},
	}
	if len(asks) != len(want) {
		t.Fatalf("asks = %+v, want %+v", asks, want)
	}
	for i := range want {
		if asks[i] != want[i] {
			t.Fatalf("asks[%d] = %+v, want %+v", i, asks[i], want[i])
		}
	}
}

func TestUnresolvedEmptyWhenAnswered(t *testing.T) {
	if left := testBuild(t, answers()).Unresolved(); len(left) != 0 {
		t.Fatalf("unresolved after full answers: %v", left)
	}
}

func TestEveryCombinationRenders(t *testing.T) {
	m, err := validate.Manifest(templates + "/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := Shapes(templates)
	if err != nil {
		t.Fatal(err)
	}
	for stack, spec := range m.Stacks {
		for _, arch := range spec.Architectures {
			for _, shape := range shapes {
				a := Answers{
					Provider:     string(m.Architectures[arch].Provider),
					Architecture: string(arch),
					Stack:        string(stack),
					Environments: shape,
					Project:      "proj",
					Dir:          "./proj",
				}
				scan := testBuild(t, a)
				a.Vars = map[string]map[string]string{}
				for _, env := range scan.Envs {
					a.Vars[env] = map[string]string{}
					for _, ask := range Asks(m, scan) {
						a.Vars[env][ask.Name] = "x"
					}
				}
				p := testBuild(t, a)
				if left := p.Unresolved(); len(left) != 0 {
					t.Fatalf("%s/%s/%s: unresolved %v", stack, arch, shape, left)
				}
				for _, f := range p.Files {
					if strings.Contains(f.Body, "# >>> ") || strings.Contains(f.Body, "# <<< ") {
						t.Fatalf("%s/%s/%s: marker survived in %s", stack, arch, shape, f.Path)
					}
				}
				file(t, p, "Makefile")
				file(t, p, "AGENTS.md")
				file(t, p, ".github/workflows/1-ci-branch.yml")
			}
		}
	}
}
```

Imports for this test file: `strings`, `testing`, and `validate`; the manifest values need no explicit `types` reference.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run 'TestAsks|TestUnresolved|TestEvery' -v`
Expected: FAIL to build, `undefined: Asks`.

- [ ] **Step 3: Write the implementation**

`internal/render/asks.go`:

```go
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
	for name, in := range m.Inputs {
		optional[name] = in.Optional
	}
	for _, spec := range []types.Spec{
		m.Providers[types.Provider(p.answersProvider)],
	} {
		_ = spec
	}
	var asks []Ask
	for name := range seen {
		asks = append(asks, Ask{Name: name, Optional: optional[name], PerEnv: perEnv[name]})
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Name < asks[j].Name })
	return asks
}
```

The `optional` map above is incomplete as sketched: it must merge inputs from all four owners. `Plan` does not carry the answers, so add the field. In `render.go`, give `Plan` the source answers:

```go
type Plan struct {
	Dir       string
	Envs      []string
	Answers   Answers
	Files     []File
	Github    types.Github
	Bootstrap *types.Bootstrap
}
```

set `Answers: a` in `Build`, and write the real optional merge in `asks.go`:

```go
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
```

(Use the second version only; the first sketch shows the trap to avoid.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -v`
Expected: PASS, including the full combination matrix.

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): ask discovery and all-combination invariants"
```

---

### Task 4: Apply with step events, TargetOK, plan tree

**Files:**
- Create: `internal/render/apply.go`
- Test: `internal/render/apply_test.go`

**Interfaces:**
- Consumes: `Plan`, `File`.
- Produces: `type Step struct { Unit string; Files int }`; `var Units = []string{"workflows", "makefile", "stack", "infra", "docs"}`; `func (p *Plan) Apply(dir string, events chan<- Step) error`; `func TargetOK(dir string) error`; `func (p *Plan) Tree() string`.

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyWritesAndStreams(t *testing.T) {
	p := testBuild(t, answers())
	dir := t.TempDir()
	events := make(chan Step, 16)
	if err := p.Apply(dir, events); err != nil {
		t.Fatal(err)
	}
	close(events)
	var units []string
	total := 0
	for s := range events {
		units = append(units, s.Unit)
		total += s.Files
	}
	if want := []string{"workflows", "makefile", "stack", "infra", "docs"}; !equal(units, want) {
		t.Fatalf("units = %v, want %v", units, want)
	}
	if total != len(p.Files) {
		t.Fatalf("streamed %d files, plan has %d", total, len(p.Files))
	}
	data, err := os.ReadFile(filepath.Join(dir, "infra/config/prd.tfvars"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"prd"`) {
		t.Fatalf("prd tfvars content wrong:\n%s", data)
	}
	info, err := os.Stat(filepath.Join(dir, ".github/scripts/semver-tag.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("script perm = %o", info.Mode().Perm())
	}
}

func TestApplyRefusesUnresolved(t *testing.T) {
	a := answers()
	a.Vars = nil
	p := testBuild(t, a)
	if err := p.Apply(t.TempDir(), nil); err == nil {
		t.Fatal("apply accepted a plan with unresolved tokens")
	}
}

func TestTargetOK(t *testing.T) {
	dir := t.TempDir()
	if err := TargetOK(filepath.Join(dir, "absent")); err != nil {
		t.Fatalf("absent dir rejected: %v", err)
	}
	if err := TargetOK(dir); err != nil {
		t.Fatalf("empty dir rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TargetOK(dir); err == nil {
		t.Fatal("occupied dir accepted")
	}
}

func TestTree(t *testing.T) {
	tree := testBuild(t, answers()).Tree()
	for _, want := range []string{".github/workflows/", "infra/", "Makefile", "AGENTS.md", "files"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("tree missing %q:\n%s", want, tree)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run 'TestApply|TestTarget|TestTree' -v`
Expected: FAIL to build, `undefined: Step`.

- [ ] **Step 3: Write the implementation**

`internal/render/apply.go`:

```go
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
```

Grouping detail: `Tree` groups by first path segment (`infra/`, `scripts/`), except under `.github/` where it splits one level deeper (`.github/workflows/`, `.github/scripts/`, `.github/actions/`), matching the spec's plan-page example.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): apply with step events, target check, plan tree"
```

---

### Task 5: Headless answers: var flags, inference, teaching report

**Files:**
- Create: `internal/cli/headless.go`
- Test: `internal/cli/headless_test.go`

**Interfaces:**
- Consumes: `render.Answers`, `render.Ask`, `types.Manifest`.
- Produces: `func ParseVars(pairs, envs []string) (map[string]map[string]string, error)`; `func Infer(m *types.Manifest, a *render.Answers) error`; `func Missing(m *types.Manifest, shapes []string, a render.Answers) []Choice` with `type Choice struct { Flag string; Options []string }`; `func Report(a render.Answers, derived map[string]bool, missing []Choice, asks []render.Ask) string`. `derived` marks axes filled by `Infer`.

- [ ] **Step 1: Write the failing test**

Use the real manifest throughout; the test file in full:

```go
package cli

import (
	"strings"
	"testing"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/validate"
)

func manifest(t *testing.T) *types.Manifest {
	t.Helper()
	m, err := validate.Manifest("../../manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestInferFromStack(t *testing.T) {
	a := render.Answers{Stack: "nextjs"}
	if err := Infer(manifest(t), &a); err != nil {
		t.Fatal(err)
	}
	if a.Architecture != "cloudflare-worker" || a.Provider != "cloudflare" {
		t.Fatalf("inferred %q/%q", a.Architecture, a.Provider)
	}
}

func TestInferConflict(t *testing.T) {
	a := render.Answers{Stack: "nextjs", Provider: "aws"}
	if err := Infer(manifest(t), &a); err == nil {
		t.Fatal("conflicting provider accepted")
	}
}

func TestInferLeavesAmbiguity(t *testing.T) {
	m := manifest(t)
	m.Stacks["nextjs"] = types.Spec{Architectures: []types.Architecture{"cloudflare-worker", "aws-ecs"}}
	a := render.Answers{Stack: "nextjs"}
	if err := Infer(m, &a); err != nil {
		t.Fatal(err)
	}
	if a.Architecture != "" {
		t.Fatalf("ambiguous architecture guessed: %q", a.Architecture)
	}
}

func TestParseVars(t *testing.T) {
	vars, err := ParseVars([]string{"CUSTOM_DOMAIN=x.com", "prd:CUSTOM_DOMAIN=y.com"}, []string{"preprod", "prd"})
	if err != nil {
		t.Fatal(err)
	}
	if vars["preprod"]["CUSTOM_DOMAIN"] != "x.com" || vars["prd"]["CUSTOM_DOMAIN"] != "y.com" {
		t.Fatalf("vars = %v", vars)
	}
	if _, err := ParseVars([]string{"staging:X=1"}, []string{"prd"}); err == nil {
		t.Fatal("unknown env accepted")
	}
	if _, err := ParseVars([]string{"lower=1"}, []string{"prd"}); err == nil {
		t.Fatal("bad name accepted")
	}
}

func TestReport(t *testing.T) {
	m := manifest(t)
	a := render.Answers{Stack: "nextjs", Project: "acme-web"}
	if err := Infer(m, &a); err != nil {
		t.Fatal(err)
	}
	missing := Missing(m, []string{"dual", "single"}, a)
	asks := []render.Ask{
		{Name: "CUSTOM_DOMAIN", Optional: true, PerEnv: true},
		{Name: "PRD_URL", Optional: true},
		{Name: "PREPROD_URL", Optional: true},
	}
	out := Report(a, map[string]bool{"architecture": true, "provider": true}, missing, asks)
	for _, want := range []string{
		"stack         nextjs",
		"architecture  cloudflare-worker   (derived)",
		"provider      cloudflare          (derived)",
		"--environments",
		"dual | single",
		"optional (empty ok)",
		"per-env: --var prd:CUSTOM_DOMAIN=",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -v`
Expected: FAIL to build, `undefined: Infer`.

- [ ] **Step 3: Write the implementation**

`internal/cli/headless.go`:

```go
package cli

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

var varName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func ParseVars(pairs, envs []string) (map[string]map[string]string, error) {
	vars := map[string]map[string]string{}
	for _, env := range envs {
		vars[env] = map[string]string{}
	}
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("--var %q: want NAME=VALUE", pair)
		}
		targets := envs
		if env, rest, scoped := strings.Cut(name, ":"); scoped {
			if !slices.Contains(envs, env) {
				return nil, fmt.Errorf("--var %q: unknown environment %q", pair, env)
			}
			targets, name = []string{env}, rest
		}
		if !varName.MatchString(name) {
			return nil, fmt.Errorf("--var %q: invalid name %q", pair, name)
		}
		for _, env := range targets {
			vars[env][name] = value
		}
	}
	return vars, nil
}

func Infer(m *types.Manifest, a *render.Answers) error {
	if a.Stack != "" && a.Architecture == "" {
		spec, ok := m.Stacks[types.Stack(a.Stack)]
		if !ok {
			return fmt.Errorf("unknown stack %q", a.Stack)
		}
		archs := spec.Architectures
		if a.Provider != "" {
			archs = nil
			for _, arch := range spec.Architectures {
				if m.Architectures[arch].Provider == types.Provider(a.Provider) {
					archs = append(archs, arch)
				}
			}
			if len(archs) == 0 {
				return fmt.Errorf("stack %q has no architecture on provider %q", a.Stack, a.Provider)
			}
		}
		if len(archs) == 1 {
			a.Architecture = string(archs[0])
		}
	}
	if a.Architecture != "" {
		spec, ok := m.Architectures[types.Architecture(a.Architecture)]
		if !ok {
			return fmt.Errorf("unknown architecture %q", a.Architecture)
		}
		if a.Provider == "" {
			a.Provider = string(spec.Provider)
		} else if a.Provider != string(spec.Provider) {
			return fmt.Errorf("architecture %q runs on %q, not %q", a.Architecture, spec.Provider, a.Provider)
		}
		if a.Stack != "" && !slices.Contains(m.Stacks[types.Stack(a.Stack)].Architectures, types.Architecture(a.Architecture)) {
			return fmt.Errorf("stack %q does not target %q", a.Stack, a.Architecture)
		}
	}
	return nil
}

type Choice struct {
	Flag    string
	Options []string
}

func Missing(m *types.Manifest, shapes []string, a render.Answers) []Choice {
	var out []Choice
	if a.Provider == "" {
		out = append(out, Choice{"--provider", asStrings(utils.Sorted(m.Providers))})
	}
	if a.Architecture == "" {
		var opts []string
		for _, arch := range utils.Sorted(m.Architectures) {
			if a.Provider == "" || m.Architectures[arch].Provider == types.Provider(a.Provider) {
				opts = append(opts, string(arch))
			}
		}
		if a.Stack != "" {
			opts = nil
			for _, arch := range m.Stacks[types.Stack(a.Stack)].Architectures {
				opts = append(opts, string(arch))
			}
			sort.Strings(opts)
		}
		out = append(out, Choice{"--architecture", opts})
	}
	if a.Stack == "" {
		var opts []string
		for _, s := range utils.Sorted(m.Stacks) {
			if a.Architecture == "" || slices.Contains(m.Stacks[s].Architectures, types.Architecture(a.Architecture)) {
				opts = append(opts, string(s))
			}
		}
		out = append(out, Choice{"--stack", opts})
	}
	if a.Environments == "" {
		out = append(out, Choice{"--environments", shapes})
	}
	if a.Project == "" {
		out = append(out, Choice{"--project", []string{"<name>"}})
	}
	return out
}

func asStrings[K ~string](keys []K) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}

func Report(a render.Answers, derived map[string]bool, missing []Choice, asks []render.Ask) string {
	var b strings.Builder
	row := func(label, value, axis string) {
		if value == "" {
			return
		}
		tag := ""
		if derived[axis] {
			tag = "   (derived)"
		}
		fmt.Fprintf(&b, "%-13s %-17s%s\n", label, value, tag)
	}
	row("stack", a.Stack, "stack")
	row("architecture", a.Architecture, "architecture")
	row("provider", a.Provider, "provider")
	row("environments", a.Environments, "environments")
	row("project", a.Project, "project")
	if a.Project != "" && a.Dir == "" {
		fmt.Fprintf(&b, "%-13s ./%s          (default)\n", "dir", a.Project)
	}
	if len(missing) > 0 {
		b.WriteString("\nmissing\n")
		for _, c := range missing {
			fmt.Fprintf(&b, "  %-20s %s\n", c.Flag, strings.Join(c.Options, " | "))
		}
	}
	var optional, required []render.Ask
	for _, ask := range asks {
		if ask.Optional {
			optional = append(optional, ask)
		} else {
			required = append(required, ask)
		}
	}
	if len(required) > 0 {
		b.WriteString("\nrequired\n")
		for _, ask := range required {
			fmt.Fprintf(&b, "  --var %s=%s\n", ask.Name, perEnvHint(ask))
		}
	}
	if len(optional) > 0 {
		b.WriteString("\noptional (empty ok)\n")
		for _, ask := range optional {
			fmt.Fprintf(&b, "  --var %s=%s\n", ask.Name, perEnvHint(ask))
		}
	}
	b.WriteString("\nadd the missing flags; --yes applies without confirmation\n")
	return b.String()
}

func perEnvHint(ask render.Ask) string {
	if ask.PerEnv {
		return "          per-env: --var prd:" + ask.Name + "="
	}
	return ""
}
```

The `Report` alignment (`%-13s %-17s`) must reproduce the spec's example spacing; adjust widths until the Task 5 test's literal strings match, and treat the test as the source of truth for format.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): var parsing, axis inference, teaching report"
```

---

### Task 6: Command move and headless `new`

**Files:**
- Create: `cmd/odyssey-cli/main.go` (content below), `internal/cli/output.go`
- Delete: `cmd/main.go`
- Modify: `.github/workflows/validate.yml` (the `go run ./cmd validate` line becomes `go run ./cmd/odyssey-cli validate`)
- Test: `internal/cli/output_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1-5, `wizard.Run`/`wizard.Shapes` (existing PoC, still wired for the TTY path until Task 10).
- Produces: `func Checklist(p *render.Plan) string`; `func PrintBootstrap(p *render.Plan) string`; `func RunBootstrap(p *render.Plan, dir string) error`; the `odyssey-cli` binary with `validate`, `new`, `find` (stub) subcommands.

- [ ] **Step 1: Write the failing test for output helpers**

```go
package cli

import (
	"strings"
	"testing"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
)

func TestChecklist(t *testing.T) {
	p := &render.Plan{Github: types.Github{
		Variables: []string{"STATE_BUCKET", "ZONE_NAME"},
		Secrets:   []string{"CLOUDFLARE_API_TOKEN"},
	}}
	out := Checklist(p)
	for _, want := range []string{
		"gh variable set STATE_BUCKET --body <value>",
		"gh variable set ZONE_NAME --body <value>",
		"gh secret set CLOUDFLARE_API_TOKEN",
		"environment scope",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("checklist missing %q:\n%s", want, out)
		}
	}
	if Checklist(&render.Plan{}) != "" {
		t.Fatal("empty github block should produce no checklist")
	}
}

func TestPrintBootstrap(t *testing.T) {
	p := &render.Plan{Bootstrap: &types.Bootstrap{
		Intent:   "install deps and run dev",
		Commands: []string{"npm install", "npm run dev"},
	}}
	out := PrintBootstrap(p)
	for _, want := range []string{"install deps and run dev", "$ npm install", "$ npm run dev"} {
		if !strings.Contains(out, want) {
			t.Fatalf("bootstrap missing %q:\n%s", want, out)
		}
	}
	if PrintBootstrap(&render.Plan{}) != "" {
		t.Fatal("nil bootstrap should print nothing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestChecklist|TestPrintBootstrap' -v`
Expected: FAIL to build, `undefined: Checklist`.

- [ ] **Step 3: Write `internal/cli/output.go`**

```go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/render"
)

func Checklist(p *render.Plan) string {
	if len(p.Github.Variables) == 0 && len(p.Github.Secrets) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("configure github before the first deploy (environment scope where prd differs):\n")
	for _, name := range p.Github.Variables {
		fmt.Fprintf(&b, "  gh variable set %s --body <value>\n", name)
	}
	for _, name := range p.Github.Secrets {
		fmt.Fprintf(&b, "  gh secret set %s\n", name)
	}
	return b.String()
}

func PrintBootstrap(p *render.Plan) string {
	if p.Bootstrap == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "bootstrap (%s):\n", p.Bootstrap.Intent)
	for _, cmd := range p.Bootstrap.Commands {
		fmt.Fprintf(&b, "  $ %s\n", cmd)
	}
	return b.String()
}

func RunBootstrap(p *render.Plan, dir string) error {
	if p.Bootstrap == nil {
		return nil
	}
	for _, line := range p.Bootstrap.Commands {
		cmd := exec.Command("sh", "-c", line)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("bootstrap %q: %w", line, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Write `cmd/odyssey-cli/main.go`**

```go
package main

import (
	"flag"
	"fmt"
	"os"

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

	missing := cli.Missing(m, shapes, a)
	if len(missing) > 0 {
		if a.Stack == "" && a.Architecture == "" && a.Provider == "" && a.Project == "" {
			fmt.Println("run `odyssey-cli find` to browse stacks, architectures and providers")
		}
		fmt.Print(cli.Report(a, derived, missing, nil))
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
	var pending []render.Ask
	incomplete := false
	for _, ask := range asks {
		if _, ok := parsed[scan.Envs[0]][ask.Name]; !ok {
			pending = append(pending, ask)
			if !ask.Optional {
				incomplete = true
			}
		}
	}
	if incomplete {
		fmt.Print(cli.Report(a, derived, nil, pending))
		os.Exit(2)
	}
	for _, env := range scan.Envs {
		for _, ask := range asks {
			if _, ok := a.Vars[env][ask.Name]; !ok {
				a.Vars[env][ask.Name] = ""
			}
		}
	}
	finish(templates, m, a, yes, bootstrap)
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
	s, err := wizard.Run(m, shapes)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("environments: %s\nprovider: %s\narchitecture: %s\nstack: %s\n",
		s.Environments, s.Provider, s.Architecture, s.Stack)
}

func runFind(args []string) {
	fatal(fmt.Errorf("find: not implemented yet"))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "odyssey-cli:", err)
	os.Exit(1)
}
```

`runTUI` keeps the PoC wizard alive so the build stays green; Task 10 replaces its body. Delete `cmd/main.go` in the same change and run `go get github.com/mattn/go-isatty` (it is already an indirect dependency, this promotes it).

- [ ] **Step 5: Run everything and exercise headless by hand**

Run: `go vet ./... && go test ./...`
Expected: PASS.

Run: `go run ./cmd/odyssey-cli new --stack nextjs --project acme-web < /dev/null`
Expected: teaching report with derived architecture and provider, missing `--environments`, exit code 2 (`echo $?`).

Run: `go run ./cmd/odyssey-cli new --stack nextjs --environments dual --project acme-web < /dev/null`
Expected: plan tree then `plan only; add --yes to apply`, exit 0, nothing written.

Run: `go run ./cmd/odyssey-cli new --stack nextjs --environments dual --project acme-web --dir /tmp/acme-web --yes < /dev/null && ls /tmp/acme-web && rm -rf /tmp/acme-web`
Expected: `✓` line per unit, the gh checklist, files on disk.

- [ ] **Step 6: Update CI and commit**

Edit `.github/workflows/validate.yml`: `go run ./cmd validate` → `go run ./cmd/odyssey-cli validate`.

```bash
git add cmd internal/cli .github/workflows/validate.yml go.mod go.sum
git rm cmd/main.go
git commit -m "feat(cli): headless new with inference, teaching report and apply"
```

---

### Task 7: find

**Files:**
- Create: `internal/cli/find.go`
- Modify: `cmd/odyssey-cli/main.go` (replace the `runFind` stub)
- Test: `internal/cli/find_test.go`

**Interfaces:**
- Consumes: `types.Manifest`, `render.Build`, `render.Asks`, `render.Shapes`, `render.Envs`.
- Produces: `type Row struct { Stack, Architecture, Provider string }`; `func Rows(m *types.Manifest) []Row` (sorted by stack then architecture); `func Filter(rows []Row, terms []string) ([]Row, error)`; `func Table(rows []Row) string`; `func Card(templates string, m *types.Manifest, shapes []string, r Row) (string, error)`.

- [ ] **Step 1: Write the failing test**

```go
package cli

import (
	"strings"
	"testing"
)

func TestRowsAndFilter(t *testing.T) {
	rows := Rows(manifest(t))
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	aws, err := Filter(rows, []string{"provider=aws"})
	if err != nil {
		t.Fatal(err)
	}
	if len(aws) != 2 {
		t.Fatalf("aws rows = %v", aws)
	}
	sub, err := Filter(rows, []string{"next"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 1 || sub[0].Stack != "nextjs" {
		t.Fatalf("substring rows = %v", sub)
	}
	if _, err := Filter(rows, []string{"color=red"}); err == nil {
		t.Fatal("unknown axis accepted")
	}
}

func TestTable(t *testing.T) {
	out := Table(Rows(manifest(t)))
	if !strings.Contains(out, "STACK") || !strings.Contains(out, "node-puppeteer") {
		t.Fatalf("table:\n%s", out)
	}
}

func TestCard(t *testing.T) {
	m := manifest(t)
	rows, _ := Filter(Rows(m), []string{"nextjs"})
	out, err := Card("../..", m, []string{"dual", "single"}, rows[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"stack          nextjs",
		"architecture   cloudflare-worker",
		"provider       cloudflare",
		"dual | single",
		"CUSTOM_DOMAIN? (per env)",
		"STATE_BUCKET",
		"CLOUDFLARE_API_TOKEN",
		"odyssey-cli new --stack nextjs",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("card missing %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestRows|TestTable|TestCard' -v`
Expected: FAIL to build, `undefined: Rows`.

- [ ] **Step 3: Write the implementation**

`internal/cli/find.go`:

```go
package cli

import (
	"fmt"
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

func Card(templates string, m *types.Manifest, shapes []string, r Row) (string, error) {
	a := render.Answers{
		Provider: r.Provider, Architecture: r.Architecture,
		Stack: r.Stack, Environments: shapes[0], Project: "project",
	}
	scan, err := render.Build(templates, m, a)
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
```

Shape order is the sorted `Shapes` output everywhere (`dual | single`); the spec's `single | dual` example was illustrative, consistency wins.

Replace `runFind` in `cmd/odyssey-cli/main.go`:

```go
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
```

- [ ] **Step 4: Run tests and exercise by hand**

Run: `go test ./internal/cli/ -v`
Expected: PASS.

Run: `go run ./cmd/odyssey-cli find && go run ./cmd/odyssey-cli find provider=aws && go run ./cmd/odyssey-cli find nextjs`
Expected: 4-row table, 2-row table, detail card.

- [ ] **Step 5: Commit**

```bash
git add internal/cli cmd/odyssey-cli/main.go
git commit -m "feat(cli): find with axis filters, table and detail card"
```

---

### Task 8: TUI foundation: deps, page machine, architecture page

**Files:**
- Modify: `go.mod`/`go.sum` (add huh, bubbles), `internal/wizard/wizard.go` (full rewrite), `internal/wizard/wizard_test.go` (full rewrite)
- Create: `internal/wizard/forms.go`, `internal/wizard/theme.go`

**Interfaces:**
- Consumes: `render.Answers`, `render.Shapes` output, `types.Manifest`.
- Produces: `type Model struct` with exported `Answers render.Answers`; `type page int` with `pageArchitecture, pageProject, pageVariables, pagePlan, pageApply, pageDone`; `func New(templates string, m *types.Manifest, shapes []string, a render.Answers, yes bool) *Model`; `Model` implements `tea.Model`. `ErrAborted` kept. Form builder `architectureForm(m *types.Manifest, shapes []string, a *render.Answers) *huh.Form` with select order provider, architecture, stack, environments.

- [ ] **Step 1: Add dependencies**

Run: `go get github.com/charmbracelet/huh@latest github.com/charmbracelet/bubbles@latest && go mod tidy`
Expected: resolves against bubbletea v1.3.x without replacing it with v2; check `go.mod` afterwards and if huh forces a bubbletea major bump, pin huh to the latest version that keeps bubbletea v1.

- [ ] **Step 2: Write the OptionsFunc spike test (the gating risk)**

This test settles whether huh re-filters a downstream select when an upstream binding changes, driven purely through `Update`. Write it first in the new `internal/wizard/wizard_test.go`:

```go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
)

func testManifest() *types.Manifest {
	return &types.Manifest{
		Providers: map[types.Provider]types.Spec{"aws": {}, "cloudflare": {}},
		Architectures: map[types.Architecture]types.Spec{
			"aws-ecs":           {Provider: "aws"},
			"cloudflare-pages":  {Provider: "cloudflare"},
			"cloudflare-worker": {Provider: "cloudflare"},
		},
		Stacks: map[types.Stack]types.Spec{
			"astro":          {Architectures: []types.Architecture{"cloudflare-pages"}},
			"go-service":     {Architectures: []types.Architecture{"aws-ecs"}},
			"nextjs":         {Architectures: []types.Architecture{"cloudflare-worker"}},
			"node-puppeteer": {Architectures: []types.Architecture{"aws-ecs"}},
		},
	}
}

var (
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	down  = tea.KeyMsg{Type: tea.KeyDown}
	esc   = tea.KeyMsg{Type: tea.KeyEsc}
	ctrlC = tea.KeyMsg{Type: tea.KeyCtrlC}
)

func drive(t *testing.T, m tea.Model, msgs ...tea.Msg) tea.Model {
	t.Helper()
	queue := append([]tea.Msg(nil), msgs...)
	for len(queue) > 0 {
		msg := queue[0]
		queue = queue[1:]
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		queue = append(queue, collect(cmd)...)
	}
	return m
}

func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	if _, ok := msg.(tea.QuitMsg); ok {
		return nil
	}
	return []tea.Msg{msg}
}

func start(t *testing.T) tea.Model {
	t.Helper()
	m := New("../..", testManifest(), []string{"dual", "single"}, render.Answers{}, false)
	var model tea.Model = m
	queue := collect(m.Init())
	model = drive(t, model, queue...)
	return drive(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})
}

func TestArchitectureFiltersByProvider(t *testing.T) {
	m := start(t)
	m = drive(t, m, down, enter)
	view := m.View()
	if strings.Contains(view, "aws-ecs") {
		t.Fatalf("architecture list not filtered for cloudflare:\n%s", view)
	}
	if !strings.Contains(view, "cloudflare-pages") || !strings.Contains(view, "cloudflare-worker") {
		t.Fatalf("cloudflare architectures missing:\n%s", view)
	}
}

func TestFullSelection(t *testing.T) {
	m := drive(t, start(t), down, enter, down, enter, enter, enter)
	w := m.(*Model)
	if w.Answers.Provider != "cloudflare" || w.Answers.Architecture != "cloudflare-worker" ||
		w.Answers.Stack != "nextjs" || w.Answers.Environments != "dual" {
		t.Fatalf("answers = %+v", w.Answers)
	}
	if w.Page() == pageArchitecture {
		t.Fatal("page did not advance after completing selection")
	}
}

func TestStatusPanelTracksAnswers(t *testing.T) {
	m := drive(t, start(t), down, enter)
	view := m.View()
	for _, want := range []string{"odyssey", "cloudflare"} {
		if !strings.Contains(view, want) {
			t.Fatalf("status panel missing %q:\n%s", want, view)
		}
	}
}

func TestCtrlCAborts(t *testing.T) {
	m := drive(t, start(t), ctrlC)
	if !m.(*Model).Aborted() {
		t.Fatal("ctrl+c did not abort")
	}
}
```

Key-sequence assumption to verify while making this pass: in huh, `enter` on a select commits it and moves to the next field, so the sequence `down, enter` picks the second option of the provider select. If huh's default keymap differs, adjust sequences, not the design.

- [ ] **Step 3: Run the spike test to verify it fails**

Run: `go test ./internal/wizard/ -run TestArchitecture -v`
Expected: FAIL to build (`undefined: New`), then after Step 4 the real spike outcome.

- [ ] **Step 4: Write the foundation**

`internal/wizard/theme.go`:

```go
package wizard

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/render"
)

const minPanelWidth = 60

var (
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			Padding(0, 1).MarginLeft(3).Width(24)
	panelTitle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	appStyle   = lipgloss.NewStyle().Padding(1, 2)
)

func theme() *huh.Theme {
	return huh.ThemeCharm()
}

func statusPanel(a render.Answers) string {
	var b strings.Builder
	b.WriteString(panelTitle.Render("odyssey") + "\n\n")
	row := func(label, value string) {
		if value == "" {
			value = dimStyle.Render("—")
		}
		b.WriteString(lipgloss.NewStyle().Width(6).Faint(true).Render(label) + " " + value + "\n")
	}
	row("cloud", a.Provider)
	row("arch", a.Architecture)
	row("stack", a.Stack)
	row("envs", a.Environments)
	row("proj", a.Project)
	return panelStyle.Render(strings.TrimRight(b.String(), "\n"))
}
```

`internal/wizard/forms.go`:

```go
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
```

`internal/wizard/wizard.go`:

```go
package wizard

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
)

var ErrAborted = errors.New("aborted")

type page int

const (
	pageArchitecture page = iota
	pageProject
	pageVariables
	pagePlan
	pageApply
	pageDone
)

type Model struct {
	templates string
	manifest  *types.Manifest
	shapes    []string
	yes       bool

	page    page
	form    *huh.Form
	Answers render.Answers
	aborted bool
	width   int
	height  int
}

func New(templates string, m *types.Manifest, shapes []string, a render.Answers, yes bool) *Model {
	w := &Model{templates: templates, manifest: m, shapes: shapes, Answers: a, yes: yes}
	w.form = architectureForm(m, shapes, &w.Answers)
	return w
}

func (w *Model) Page() page    { return w.page }
func (w *Model) Aborted() bool { return w.aborted }

func (w *Model) Init() tea.Cmd {
	return w.form.Init()
}

func (w *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width, w.height = msg.Width, msg.Height
		return w, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			w.aborted = true
			return w, tea.Quit
		case tea.KeyEsc:
			return w.back()
		}
	}
	return w.route(msg)
}

func (w *Model) route(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w.form == nil {
		return w, nil
	}
	next, cmd := w.form.Update(msg)
	if f, ok := next.(*huh.Form); ok {
		w.form = f
	}
	if w.form.State == huh.StateCompleted {
		return w.advance()
	}
	return w, cmd
}

func (w *Model) advance() (tea.Model, tea.Cmd) {
	switch w.page {
	case pageArchitecture:
		w.page = pageDone
		return w, tea.Quit
	}
	return w, nil
}

func (w *Model) back() (tea.Model, tea.Cmd) {
	if w.page == pageArchitecture {
		w.aborted = true
		return w, tea.Quit
	}
	return w, nil
}

func (w *Model) View() string {
	if w.page == pageDone || w.aborted {
		return ""
	}
	body := ""
	if w.form != nil {
		body = w.form.View()
	}
	if w.width >= minPanelWidth {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, statusPanel(w.Answers))
	}
	return appStyle.Render(body)
}
```

`advance` and `back` grow the remaining pages in Tasks 9 and 10; here page one completes and quits so the spike is meaningful.

- [ ] **Step 5: Run the spike and full test file**

Run: `go test ./internal/wizard/ -v`
Expected: PASS. If `TestArchitectureFiltersByProvider` fails because huh caches or defers OptionsFunc through the message loop, this is the documented fallback, not a redesign: drop `OptionsFunc`, watch `w.Answers.Provider`/`w.Answers.Architecture` in `route` after forwarding, and when one changed rebuild the page-1 form in place (preserving already-picked values) before returning. The page machine already owns form construction, so the fallback touches only `route`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/wizard
git commit -m "feat(wizard): huh-in-bubbletea page machine and architecture page"
```

---

### Task 9: Project and variables pages

**Files:**
- Modify: `internal/wizard/forms.go`, `internal/wizard/wizard.go`
- Test: `internal/wizard/wizard_test.go` (extend)

**Interfaces:**
- Consumes: `render.Asks`, `render.Build`, `render.TargetOK`, page machine from Task 8.
- Produces: `projectForm(a *render.Answers) *huh.Form` (inputs keyed `project`, `dir`); `variablesForm(asks []render.Ask, envs []string, screen int, vals map[string]map[string]*string) *huh.Form`; `Model` fields `asks []render.Ask`, `envs []string`, `varScreen int`, `varVals map[string]map[string]*string`.

- [ ] **Step 1: Write the failing tests**

Append to `wizard_test.go`:

```go
func typeString(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return msgs
}

func selectAll(t *testing.T) tea.Model {
	return drive(t, start(t), down, enter, down, enter, enter, enter)
}

func TestProjectPage(t *testing.T) {
	m := selectAll(t)
	if m.(*Model).Page() != pageProject {
		t.Fatalf("page = %v, want pageProject", m.(*Model).Page())
	}
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter, enter)
	w := m.(*Model)
	if w.Answers.Project != "acme-web" {
		t.Fatalf("project = %q", w.Answers.Project)
	}
	if w.Answers.Dir != "./acme-web" {
		t.Fatalf("dir = %q", w.Answers.Dir)
	}
}

func TestProjectCodeValidation(t *testing.T) {
	if err := validProject("Acme Web"); err == nil {
		t.Fatal("bad project code accepted")
	}
	if err := validProject("acme-web2"); err != nil {
		t.Fatalf("good project code rejected: %v", err)
	}
}

func TestVariablesScreens(t *testing.T) {
	m := selectAll(t)
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter, enter)
	w := m.(*Model)
	if w.Page() != pageVariables {
		t.Fatalf("page = %v, want pageVariables", w.Page())
	}
	view := m.View()
	for _, want := range []string{"CUSTOM_DOMAIN", "PREPROD_URL", "PRD_URL"} {
		if !strings.Contains(view, want) {
			t.Fatalf("variables screen missing %q:\n%s", want, view)
		}
	}
	m = drive(t, m, typeString("pre.example.com")...)
	m = drive(t, m, enter, enter, enter)
	w = m.(*Model)
	if w.Page() != pageVariables || w.varScreen != 1 {
		t.Fatalf("expected second env screen, page=%v screen=%d", w.Page(), w.varScreen)
	}
	if got := *w.varVals["prd"]["CUSTOM_DOMAIN"]; got != "pre.example.com" {
		t.Fatalf("prd not prefilled from preprod, got %q", got)
	}
	m = drive(t, m, enter)
	w = m.(*Model)
	if w.Page() != pagePlan {
		t.Fatalf("page after variables = %v, want pagePlan", w.Page())
	}
	if w.Answers.Vars["preprod"]["CUSTOM_DOMAIN"] != "pre.example.com" {
		t.Fatalf("vars not harvested: %v", w.Answers.Vars)
	}
}
```

The first variables screen for nextjs/dual holds `CUSTOM_DOMAIN` (preprod value), `PRD_URL`, `PREPROD_URL` in ask order (sorted by name); the key-count assumptions (`enter` per field) follow from that order. The `TestVariablesScreens` drive into `pagePlan` requires `render.Build` to succeed with templates `../..`, which it does since Task 3's matrix test proves every combination.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/wizard/ -run 'TestProject|TestVariables' -v`
Expected: FAIL, page stays `pageDone` after selection (Task 8's `advance` quits).

- [ ] **Step 3: Implement the pages**

In `forms.go` add:

```go
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
```

In `wizard.go`, extend `Model` with `asks []render.Ask`, `envs []string`, `varScreen int`, `varVals map[string]map[string]*string`, and rewrite `advance`/`back`:

```go
func (w *Model) advance() (tea.Model, tea.Cmd) {
	switch w.page {
	case pageArchitecture:
		w.page = pageProject
		w.form = projectForm(&w.Answers)
		return w, w.form.Init()
	case pageProject:
		if w.Answers.Dir == "" {
			w.Answers.Dir = "./" + w.Answers.Project
		}
		return w.enterVariables()
	case pageVariables:
		w.harvestScreen()
		if w.varScreen+1 < len(w.envs) && w.hasPerEnv() {
			w.varScreen++
			w.prefill(w.varScreen)
			w.form = variablesForm(w.asks, w.envs, w.varScreen, w.varVals)
			return w, w.form.Init()
		}
		return w.enterPlan()
	}
	return w, nil
}

func (w *Model) enterVariables() (tea.Model, tea.Cmd) {
	scan, err := render.Build(w.templates, w.manifest, w.Answers)
	if err != nil {
		w.fail(err)
		return w, tea.Quit
	}
	w.envs = scan.Envs
	w.asks = render.Asks(w.manifest, scan)
	if len(w.asks) == 0 {
		return w.enterPlan()
	}
	w.varVals = map[string]map[string]*string{}
	for _, env := range w.envs {
		w.varVals[env] = map[string]*string{}
		for _, ask := range w.asks {
			s := ""
			if w.Answers.Vars[env] != nil {
				s = w.Answers.Vars[env][ask.Name]
			}
			w.varVals[env][ask.Name] = &s
		}
	}
	w.page = pageVariables
	w.varScreen = 0
	w.form = variablesForm(w.asks, w.envs, 0, w.varVals)
	return w, w.form.Init()
}

func (w *Model) hasPerEnv() bool {
	for _, ask := range w.asks {
		if ask.PerEnv {
			return true
		}
	}
	return false
}

func (w *Model) prefill(screen int) {
	first := w.envs[0]
	env := w.envs[screen]
	for _, ask := range w.asks {
		if ask.PerEnv && *w.varVals[env][ask.Name] == "" {
			v := *w.varVals[first][ask.Name]
			w.varVals[env][ask.Name] = &v
		}
	}
}

func (w *Model) harvestScreen() {
	if w.Answers.Vars == nil {
		w.Answers.Vars = map[string]map[string]string{}
	}
	for _, env := range w.envs {
		if w.Answers.Vars[env] == nil {
			w.Answers.Vars[env] = map[string]string{}
		}
	}
	env := w.envs[w.varScreen]
	for _, ask := range w.asks {
		if w.varScreen > 0 && !ask.PerEnv {
			continue
		}
		v := *w.varVals[env][ask.Name]
		if ask.PerEnv {
			w.Answers.Vars[env][ask.Name] = v
		} else {
			for _, e := range w.envs {
				w.Answers.Vars[e][ask.Name] = v
			}
		}
	}
}

func (w *Model) back() (tea.Model, tea.Cmd) {
	switch w.page {
	case pageArchitecture:
		w.aborted = true
		return w, tea.Quit
	case pageProject:
		w.page = pageArchitecture
		w.form = architectureForm(w.manifest, w.shapes, &w.Answers)
		return w, w.form.Init()
	case pageVariables:
		if w.varScreen > 0 {
			w.varScreen--
			w.form = variablesForm(w.asks, w.envs, w.varScreen, w.varVals)
			return w, w.form.Init()
		}
		w.page = pageProject
		w.form = projectForm(&w.Answers)
		return w, w.form.Init()
	}
	return w, nil
}
```

`enterPlan` and `fail` arrive in Task 10; for this task stub them:

```go
func (w *Model) enterPlan() (tea.Model, tea.Cmd) {
	w.page = pagePlan
	w.form = nil
	return w, nil
}

func (w *Model) fail(err error) {
	w.err = err
	w.aborted = true
}
```

with an `err error` field on `Model`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/wizard/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wizard
git commit -m "feat(wizard): project and per-env variables pages"
```

---

### Task 10: Plan and apply pages, Run, interactive wiring

**Files:**
- Modify: `internal/wizard/wizard.go`, `internal/wizard/forms.go`, `cmd/odyssey-cli/main.go` (replace `runTUI`)
- Test: `internal/wizard/wizard_test.go` (extend)

**Interfaces:**
- Consumes: `render.Build`, `Plan.Tree`, `Plan.Apply`, `render.TargetOK`, `cli.Checklist`, `cli.PrintBootstrap`.
- Produces: `func Run(templates string, m *types.Manifest, shapes []string, a render.Answers, yes bool) (*render.Plan, error)` returning the applied plan, `ErrAborted` on abort; flag pre-seeding skips completed pages; `esc` skips back over flag-completed pages.

- [ ] **Step 1: Write the failing tests**

```go
func TestPlanPageAndConfirm(t *testing.T) {
	m := selectAll(t)
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter, enter)
	m = drive(t, m, typeString("pre.example.com")...)
	m = drive(t, m, enter, enter, enter, enter)
	w := m.(*Model)
	if w.Page() != pagePlan {
		t.Fatalf("page = %v", w.Page())
	}
	view := m.View()
	if !strings.Contains(view, "files") || !strings.Contains(view, "Write") {
		t.Fatalf("plan view:\n%s", view)
	}
	m = drive(t, m, down, enter)
	if !m.(*Model).Aborted() {
		t.Fatal("answering No did not abort")
	}
}

func TestFlagSeededSkipsPages(t *testing.T) {
	a := render.Answers{
		Provider: "cloudflare", Architecture: "cloudflare-worker",
		Stack: "nextjs", Environments: "dual", Project: "acme-web", Dir: "./x-does-not-exist",
	}
	w := New("../..", testManifest(), []string{"dual", "single"}, a, false)
	var m tea.Model = w
	m = drive(t, m, collect(w.Init())...)
	m = drive(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := m.(*Model).Page(); got != pageVariables {
		t.Fatalf("flag-seeded start page = %v, want pageVariables", got)
	}
	m = drive(t, m, esc)
	if !m.(*Model).Aborted() {
		t.Fatal("esc on the first live page should abort, flag-completed pages are not revisited")
	}
}
```

`esc` on the first page a user actually sees aborts, exactly as it does on the architecture page in a flagless run; pages completed by flags are never navigation targets.

The confirm sequence `down, enter` assumes huh's Confirm toggles to the negative with down/right; verify against huh's keymap while implementing and adjust (left/right arrows are the common binding, `h`/`l` also work).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/wizard/ -run 'TestPlanPage|TestFlagSeeded' -v`
Expected: FAIL, plan page has no view/confirm yet.

- [ ] **Step 3: Implement plan page, apply page, Run**

In `forms.go`:

```go
func confirmForm(n int, dir string, ok *bool) *huh.Form {
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Key("write").
			Title(fmt.Sprintf("Write %d files to %s?", n, dir)).
			Affirmative("Yes").Negative("No").Value(ok),
	)).WithTheme(theme())
}
```

In `wizard.go`, add fields `plan *render.Plan`, `confirm bool`, `spin spinner.Model`, `applying bool`, `applied []render.Step`, `events chan render.Step`, `applyErr chan error`, `flagPages map[page]bool`, and:

```go
func (w *Model) enterPlan() (tea.Model, tea.Cmd) {
	if err := render.TargetOK(w.Answers.Dir); err != nil {
		w.fail(err)
		return w, tea.Quit
	}
	p, err := render.Build(w.templates, w.manifest, w.Answers)
	if err != nil {
		w.fail(err)
		return w, tea.Quit
	}
	if left := p.Unresolved(); len(left) > 0 {
		w.fail(fmt.Errorf("unresolved variables: %s", strings.Join(left, ", ")))
		return w, tea.Quit
	}
	w.plan = p
	if w.yes {
		return w.enterApply()
	}
	w.page = pagePlan
	w.confirm = true
	w.form = confirmForm(len(p.Files), w.Answers.Dir, &w.confirm)
	return w, w.form.Init()
}

func (w *Model) enterApply() (tea.Model, tea.Cmd) {
	w.page = pageApply
	w.form = nil
	w.applying = true
	w.spin = spinner.New(spinner.WithSpinner(spinner.Dot))
	w.events = make(chan render.Step, len(render.Units))
	w.applyErr = make(chan error, 1)
	plan, dir, events, errc := w.plan, w.Answers.Dir, w.events, w.applyErr
	go func() { errc <- plan.Apply(dir, events); close(events) }()
	return w, tea.Batch(w.spin.Tick, w.nextStep())
}

type stepMsg struct {
	step render.Step
	ok   bool
}

func (w *Model) nextStep() tea.Cmd {
	return func() tea.Msg {
		step, ok := <-w.events
		return stepMsg{step, ok}
	}
}
```

and extend `Update`'s message switch (before key handling):

```go
	case stepMsg:
		if msg.ok {
			w.applied = append(w.applied, msg.step)
			return w, tea.Batch(
				tea.Printf("✓ %s (%d files)", msg.step.Unit, msg.step.Files),
				w.nextStep(),
			)
		}
		if err := <-w.applyErr; err != nil {
			w.fail(err)
		}
		w.page = pageDone
		return w, tea.Quit
	case spinner.TickMsg:
		if w.applying {
			var cmd tea.Cmd
			w.spin, cmd = w.spin.Update(msg)
			return w, cmd
		}
		return w, nil
```

`advance` gains the plan case:

```go
	case pagePlan:
		if !w.confirm {
			w.aborted = true
			return w, tea.Quit
		}
		return w.enterApply()
```

`back` gains:

```go
	case pagePlan:
		return w.enterVariables()
```

and every `back` transition target checks `w.flagPages`: skip over pages the flags completed (walk backwards until a page not in `flagPages`, aborting past `pageArchitecture`).

`View` becomes:

```go
func (w *Model) View() string {
	if w.page == pageDone || w.aborted {
		return ""
	}
	if w.page == pageApply {
		return appStyle.Render(w.spin.View() + " applying " + w.Answers.Dir)
	}
	body := ""
	if w.form != nil {
		body = w.form.View()
	}
	if w.page == pagePlan {
		body = "Plan: " + w.Answers.Dir + "\n" + w.plan.Tree() + "\n" + body
	}
	if w.width >= minPanelWidth {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, statusPanel(w.Answers))
	}
	return appStyle.Render(body)
}
```

`New` gains flag pre-seeding, replacing the unconditional architecture form:

```go
func New(templates string, m *types.Manifest, shapes []string, a render.Answers, yes bool) *Model {
	w := &Model{templates: templates, manifest: m, shapes: shapes, Answers: a, yes: yes, flagPages: map[page]bool{}}
	if err := cliInfer(m, &w.Answers); err == nil &&
		a.Provider != "" && a.Architecture != "" && a.Stack != "" && a.Environments != "" {
		w.flagPages[pageArchitecture] = true
	}
	if a.Project != "" && validProject(a.Project) == nil {
		w.flagPages[pageProject] = true
	}
	switch {
	case !w.flagPages[pageArchitecture]:
		w.form = architectureForm(m, shapes, &w.Answers)
	case !w.flagPages[pageProject]:
		w.page = pageProject
		w.form = projectForm(&w.Answers)
	default:
		if w.Answers.Dir == "" {
			w.Answers.Dir = "./" + w.Answers.Project
		}
		w.pending = true
	}
	return w
}
```

Inference lives in `internal/cli`, which must not import `internal/wizard` cyclically; `internal/wizard` importing `internal/cli` is fine (cli does not import wizard). Import it as `cliInfer` via a small alias: `func cliInfer(m *types.Manifest, a *render.Answers) error { return cli.Infer(m, a) }`. The `pending` flag defers `enterVariables` to `Init` (a model cannot emit page-transition cmds from `New`): in `Init`, when `pending`, run `enterVariables` and return its cmd together with any form init.

`Run` replaces the old function:

```go
func Run(templates string, m *types.Manifest, shapes []string, a render.Answers, yes bool) (*render.Plan, error) {
	w := New(templates, m, shapes, a, yes)
	out, err := tea.NewProgram(w).Run()
	if err != nil {
		return nil, err
	}
	final := out.(*Model)
	if final.err != nil {
		return nil, final.err
	}
	if final.aborted || final.page != pageDone {
		return nil, ErrAborted
	}
	return final.plan, nil
}
```

No `tea.WithAltScreen`: inline is the point, `tea.Printf` lines must persist.

Replace `runTUI` in `cmd/odyssey-cli/main.go`:

```go
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
```

- [ ] **Step 4: Run all tests, vet, and a real TTY smoke**

Run: `go vet ./... && go test ./...`
Expected: PASS.

Manual (needs a real terminal, ask the human partner if executing as a subagent): `go run ./cmd/odyssey-cli new` and walk all five pages against a temp `--dir`; confirm the status panel fills, esc walks back, the apply lines persist after exit, and the checklist prints.

- [ ] **Step 5: Commit**

```bash
git add internal/wizard cmd/odyssey-cli/main.go
git commit -m "feat(wizard): plan and apply pages, flag pre-seeding, interactive wiring"
```

---

### Task 11: Roadmap amendments and final verification

**Files:**
- Modify: `ROADMAP.md`

**Interfaces:** none; documentation and verification only.

- [ ] **Step 1: Amend ROADMAP.md**

In the Session 5 table: replace the `wizard` row's text with the settled design: five pages (architecture, project, variables, plan, apply), one parent Bubble Tea model embedding a fresh huh form per page, huh un-rejected because the parent owns layout and options resolve synchronously from the in-memory manifest; select order provider, architecture, stack, environments (environments filters nothing). Add rows for headless (TTY detection, inference, teaching output, exit 2) and `find`. Update the sessions table state column for session 5. Keep the existing terse table voice.

- [ ] **Step 2: Full verification**

Run: `go vet ./... && go test ./... && go run ./cmd/odyssey-cli validate`
Expected: all PASS, `manifest and fragments are valid`.

Run: `go run ./cmd/odyssey-cli find nextjs && go run ./cmd/odyssey-cli new --stack nextjs --project t < /dev/null; echo "exit=$?"`
Expected: card, then teaching report with `exit=2`.

- [ ] **Step 3: Commit**

```bash
git add ROADMAP.md
git commit -m "docs: record session 5 wizard, headless and find decisions"
```
