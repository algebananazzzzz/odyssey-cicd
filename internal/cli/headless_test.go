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

func TestMissingFiltersStacksByProvider(t *testing.T) {
	missing := Missing(manifest(t), []string{"dual", "single"}, render.Answers{Provider: "aws"})
	for _, c := range missing {
		if c.Flag != "--stack" {
			continue
		}
		want := []string{"go-service", "node-puppeteer"}
		if len(c.Options) != len(want) || c.Options[0] != want[0] || c.Options[1] != want[1] {
			t.Fatalf("--stack options = %v, want %v", c.Options, want)
		}
		return
	}
	t.Fatal("no --stack choice in missing")
}
