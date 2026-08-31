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
			"preprod": {"CUSTOM_DOMAIN": "pre.example.com"},
			"prd":     {"CUSTOM_DOMAIN": "example.com"},
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
