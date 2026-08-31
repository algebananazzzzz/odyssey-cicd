package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/cli"
	"github.com/algebananazzzzz/odyssey/internal/render"
	"github.com/algebananazzzzz/odyssey/internal/types"
)

func testManifest() *types.Manifest {
	return &types.Manifest{
		Providers: map[types.Provider]types.Spec{"aws": {}, "cloudflare": {}},
		Architectures: map[types.Architecture]types.Spec{
			"aws-ecs": {Provider: "aws"},
			"cloudflare-pages": {Provider: "cloudflare", Inputs: map[string]types.Input{
				"CUSTOM_DOMAIN": {Optional: true},
			}},
			"cloudflare-worker": {Provider: "cloudflare", Inputs: map[string]types.Input{
				"CUSTOM_DOMAIN": {Optional: true},
			}},
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
	left  = tea.KeyMsg{Type: tea.KeyLeft}
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
	if _, ok := msg.(cursor.BlinkMsg); ok {
		return nil
	}
	return []tea.Msg{msg}
}

func start(t *testing.T) tea.Model {
	t.Helper()
	m := New(os.DirFS("../.."), testManifest(), []string{"dual", "single"}, render.Answers{}, false)
	var model tea.Model = m
	queue := collect(m.Init())
	model = drive(t, model, queue...)
	return drive(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})
}

func TestViewMatchesExampleLayout(t *testing.T) {
	m := start(t)
	view := m.View()
	if !strings.Contains(view, "Odyssey Project Wizard") {
		t.Fatalf("header title missing:\n%s", view)
	}
	if !strings.Contains(view, "////") {
		t.Fatalf("header boundary fill missing:\n%s", view)
	}
	if !strings.Contains(view, "enter") {
		t.Fatalf("footer help missing:\n%s", view)
	}
	if got := lipgloss.Width(view); got != 100 {
		t.Fatalf("view width = %d, want the full 100-column window:\n%s", got, view)
	}
	if got := lipgloss.Height(view); got != 30 {
		t.Fatalf("view height = %d, want the full 30-row screen:\n%s", got, view)
	}
	for _, want := range []string{"Provider", "Architecture", "Stack", "Environments"} {
		if !strings.Contains(view, want) {
			t.Fatalf("question %q not visible:\n%s", want, view)
		}
	}
	if got := lipgloss.Height(m.(*Model).form.View()); got < 20 {
		t.Fatalf("form height = %d, not filling the 30-row screen", got)
	}
	wide := drive(t, m, tea.WindowSizeMsg{Width: 200, Height: 50})
	if got := lipgloss.Width(wide.View()); got != maxWidth {
		t.Fatalf("view width = %d on a 200-column window, want capped at %d", got, maxWidth)
	}
	if got := lipgloss.Height(wide.View()); got != 50 {
		t.Fatalf("view height = %d after resize, want 50", got)
	}
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
	m := start(t)
	for _, msg := range []tea.Msg{down, enter, down, enter, enter, enter} {
		m = drive(t, m, msg)
	}
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

func TestStatusPanelHidesWhenNarrow(t *testing.T) {
	m := drive(t, start(t), tea.WindowSizeMsg{Width: 75, Height: 20})
	if strings.Contains(m.View(), "odyssey") {
		t.Fatalf("status panel should hide when it cannot fit:\n%s", m.View())
	}
}

func TestCtrlCAborts(t *testing.T) {
	m := drive(t, start(t), ctrlC)
	if !m.(*Model).Aborted() {
		t.Fatal("ctrl+c did not abort")
	}
}

func typeString(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return msgs
}

func selectAll(t *testing.T) tea.Model {
	m := start(t)
	for _, msg := range []tea.Msg{down, enter, down, enter, enter, enter} {
		m = drive(t, m, msg)
	}
	return m
}

func TestProjectPage(t *testing.T) {
	m := selectAll(t)
	if m.(*Model).Page() != pageProject {
		t.Fatalf("page = %v, want pageProject", m.(*Model).Page())
	}
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter)
	m = drive(t, m, enter)
	w := m.(*Model)
	if w.Answers.Project != "acme-web" {
		t.Fatalf("project = %q", w.Answers.Project)
	}
	if w.Answers.Dir != "./acme-web" {
		t.Fatalf("dir = %q", w.Answers.Dir)
	}
}

func TestProjectCodeValidation(t *testing.T) {
	if err := cli.ValidProject("Acme Web"); err == nil {
		t.Fatal("bad project code accepted")
	}
	if err := cli.ValidProject("acme-web2"); err != nil {
		t.Fatalf("good project code rejected: %v", err)
	}
}

func TestVariablesScreens(t *testing.T) {
	m := selectAll(t)
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter)
	m = drive(t, m, enter)
	w := m.(*Model)
	if w.Page() != pageVariables {
		t.Fatalf("page = %v, want pageVariables", w.Page())
	}
	view := m.View()
	for _, want := range []string{"CUSTOM_DOMAIN", "./acme-web"} {
		if !strings.Contains(view, want) {
			t.Fatalf("variables screen missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "_URL") {
		t.Fatalf("variables screen still asks for a URL:\n%s", view)
	}
	m = drive(t, m, typeString("pre.example.com")...)
	m = drive(t, m, enter)
	w = m.(*Model)
	if w.Page() != pageVariables || w.varScreen != 1 {
		t.Fatalf("expected second env screen, page=%v screen=%d", w.Page(), w.varScreen)
	}
	if got := *w.varVals["prd"]["CUSTOM_DOMAIN"]; got != "" {
		t.Fatalf("prd should start blank, not copy preprod, got %q", got)
	}
	m = drive(t, m, enter)
	w = m.(*Model)
	if w.Page() != pagePlan {
		t.Fatalf("page after variables = %v, want pagePlan", w.Page())
	}
	if w.Answers.Vars["preprod"]["CUSTOM_DOMAIN"] != "pre.example.com" {
		t.Fatalf("vars not harvested: %v", w.Answers.Vars)
	}
	if w.Answers.Vars["prd"]["CUSTOM_DOMAIN"] != "" {
		t.Fatalf("prd should stay empty, got %q", w.Answers.Vars["prd"]["CUSTOM_DOMAIN"])
	}
}

func TestPlanPageAndConfirm(t *testing.T) {
	m := selectAll(t)
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter)
	m = drive(t, m, enter)
	m = drive(t, m, typeString("pre.example.com")...)
	m = drive(t, m, enter)
	m = drive(t, m, enter)
	w := m.(*Model)
	if w.Page() != pagePlan {
		t.Fatalf("page = %v", w.Page())
	}
	view := m.View()
	if !strings.Contains(view, "files") || !strings.Contains(view, "Write") {
		t.Fatalf("plan view:\n%s", view)
	}
	m = drive(t, m, left)
	m = drive(t, m, enter)
	if !m.(*Model).Aborted() {
		t.Fatal("answering No did not abort")
	}
}

func TestEscFromPlanReturnsToVariables(t *testing.T) {
	m := selectAll(t)
	m = drive(t, m, typeString("acme-web")...)
	m = drive(t, m, enter)
	m = drive(t, m, enter)
	m = drive(t, m, typeString("pre.example.com")...)
	m = drive(t, m, enter)
	m = drive(t, m, enter)
	w := m.(*Model)
	if w.Page() != pagePlan {
		t.Fatalf("page = %v, want pagePlan", w.Page())
	}
	m = drive(t, m, esc)
	w = m.(*Model)
	if w.Page() != pageVariables {
		t.Fatalf("page after esc = %v, want pageVariables", w.Page())
	}
	if w.varScreen != 0 {
		t.Fatalf("varScreen after esc = %d, want 0", w.varScreen)
	}
	if got := *w.varVals["preprod"]["CUSTOM_DOMAIN"]; got != "pre.example.com" {
		t.Fatalf("preprod CUSTOM_DOMAIN not carried over, got %q", got)
	}
}

func TestFlagSeededSkipsPages(t *testing.T) {
	a := render.Answers{
		Provider: "cloudflare", Architecture: "cloudflare-worker",
		Stack: "nextjs", Environments: "dual", Project: "acme-web", Dir: "./x-does-not-exist",
	}
	w := New(os.DirFS("../.."), testManifest(), []string{"dual", "single"}, a, false)
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
