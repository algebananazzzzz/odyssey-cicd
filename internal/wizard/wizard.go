package wizard

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/cli"
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
	pending bool

	asks      []render.Ask
	envs      []string
	varScreen int
	varVals   map[string]map[string]*string

	flagPages map[page]bool

	plan     *render.Plan
	confirm  bool
	spin     spinner.Model
	applying bool
	applied  []render.Step
	events   chan render.Step
	applyErr chan error
}

func cliInfer(m *types.Manifest, a *render.Answers) error { return cli.Infer(m, a) }

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

func (w *Model) Page() page    { return w.page }
func (w *Model) Aborted() bool { return w.aborted }

func (w *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if w.form != nil {
		cmds = append(cmds, w.form.Init())
	}
	if w.pending {
		w.pending = false
		_, cmd := w.enterVariables()
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (w *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width, w.height = msg.Width, msg.Height
		if w.form != nil {
			w.form = w.form.WithWidth(w.width)
		}
		return w, nil
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
	case pagePlan:
		if !w.confirm {
			w.aborted = true
			return w, tea.Quit
		}
		return w.enterApply()
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
		return w.backTo(pageArchitecture)
	case pageVariables:
		if w.varScreen > 0 {
			w.varScreen--
			w.form = variablesForm(w.asks, w.envs, w.varScreen, w.varVals)
			return w, w.form.Init()
		}
		return w.backTo(pageProject)
	case pagePlan:
		return w.enterVariables()
	}
	return w, nil
}

func (w *Model) backTo(target page) (tea.Model, tea.Cmd) {
	for target >= pageArchitecture && w.flagPages[target] {
		target--
	}
	switch target {
	case pageArchitecture:
		w.page = pageArchitecture
		w.form = architectureForm(w.manifest, w.shapes, &w.Answers)
		return w, w.form.Init()
	case pageProject:
		w.page = pageProject
		w.form = projectForm(&w.Answers)
		return w, w.form.Init()
	}
	w.aborted = true
	return w, tea.Quit
}

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
