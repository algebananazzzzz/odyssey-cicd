package wizard

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/algebananazzzzz/odyssey/internal/types"
)

func testModel() model {
	m := model{
		manifest: &types.Manifest{
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
		},
		shapes: []string{"dual", "single"},
	}
	m.load()
	return m
}

func drive(t *testing.T, m model, keys ...tea.KeyMsg) model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(model)
	}
	return m
}

var (
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	down  = tea.KeyMsg{Type: tea.KeyDown}
)

func TestWaterfallFilters(t *testing.T) {
	m := drive(t, testModel(), enter, down, enter)
	if want := []string{"cloudflare-pages", "cloudflare-worker"}; !slices.Equal(m.options, want) {
		t.Fatalf("architecture options for cloudflare = %v, want %v", m.options, want)
	}
	m = drive(t, m, down, enter)
	if want := []string{"nextjs"}; !slices.Equal(m.options, want) {
		t.Fatalf("stack options for cloudflare-worker = %v, want %v", m.options, want)
	}
	m = drive(t, m, enter)
	if m.step != stepDone {
		t.Fatalf("step = %v, want stepDone", m.step)
	}
	want := Selection{Environments: "dual", Provider: "cloudflare", Architecture: "cloudflare-worker", Stack: "nextjs"}
	if m.sel != want {
		t.Fatalf("selection = %+v, want %+v", m.sel, want)
	}
}

func TestFirstOptionDefaults(t *testing.T) {
	m := drive(t, testModel(), enter, enter, enter, enter)
	want := Selection{Environments: "dual", Provider: "aws", Architecture: "aws-ecs", Stack: "go-service"}
	if m.sel != want {
		t.Fatalf("selection = %+v, want %+v", m.sel, want)
	}
}

func TestSidebarTracksAnswers(t *testing.T) {
	m := drive(t, testModel(), enter, down, enter)
	m.width, m.height = 80, 24
	side := m.sidebar()
	for _, want := range []string{"Environments", "dual", "cloudflare", "Architecture", "Stack"} {
		if !strings.Contains(side, want) {
			t.Fatalf("sidebar missing %q:\n%s", want, side)
		}
	}
	view := m.View()
	if !strings.Contains(view, "│") {
		t.Fatalf("view has no sidebar border:\n%s", view)
	}
	if !strings.Contains(view, "cloudflare-pages") {
		t.Fatalf("view missing current options:\n%s", view)
	}
}

func TestAbort(t *testing.T) {
	m := drive(t, testModel(), enter, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !m.aborted {
		t.Fatal("q did not abort")
	}
}
