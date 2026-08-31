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
	err     error
	width   int
	height  int

	asks      []render.Ask
	envs      []string
	varScreen int
	varVals   map[string]map[string]*string
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

func (w *Model) enterPlan() (tea.Model, tea.Cmd) {
	w.page = pagePlan
	w.form = nil
	return w, nil
}

func (w *Model) fail(err error) {
	w.err = err
	w.aborted = true
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
