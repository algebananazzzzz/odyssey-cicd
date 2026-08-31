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
