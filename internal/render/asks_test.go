package render

import (
	"strings"
	"testing"

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
