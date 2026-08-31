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
