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
	iStack := strings.Index(mk.Body, "npm run build")
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
