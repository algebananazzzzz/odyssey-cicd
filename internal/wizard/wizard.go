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
	lg      *lipgloss.Renderer
	styles  *Styles
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
	w.lg = lipgloss.DefaultRenderer()
	w.styles = NewStyles(w.lg)
	w.width = maxWidth
	if err := cliInfer(m, &w.Answers); err == nil &&
		a.Provider != "" && a.Architecture != "" && a.Stack != "" && a.Environments != "" {
		w.flagPages[pageArchitecture] = true
	}
	if a.Project != "" && cli.ValidProject(a.Project) == nil {
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
		w.width = min(msg.Width, maxWidth) - w.styles.Base.GetHorizontalFrameSize()
		w.height = msg.Height
		if w.form != nil {
			w.form = w.sized(w.form)
		}
	case stepMsg:
		if msg.ok {
			w.applied = append(w.applied, msg.step)
			return w, w.nextStep()
		}
		err := <-w.applyErr
		w.page = pageDone
		if err != nil {
			w.fail(fmt.Errorf("%w; the directory %s is incomplete, delete it before retrying", err, w.Answers.Dir))
		}
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

func (w *Model) sized(f *huh.Form) *huh.Form {
	if w.height == 0 || w.page == pagePlan {
		return f
	}
	return f.WithHeight(max(w.height-6, 5))
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
		w.form = w.sized(projectForm(&w.Answers))
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
			w.form = w.sized(variablesForm(w.asks, w.envs, w.varScreen, w.varVals))
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
	w.buildVarVals()
	w.page = pageVariables
	w.varScreen = 0
	w.form = w.sized(variablesForm(w.asks, w.envs, 0, w.varVals))
	return w, w.form.Init()
}

func (w *Model) buildVarVals() {
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
}

func (w *Model) hasPerEnv() bool {
	for _, ask := range w.asks {
		if ask.PerEnv {
			return true
		}
	}
	return false
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
			w.form = w.sized(variablesForm(w.asks, w.envs, w.varScreen, w.varVals))
			return w, w.form.Init()
		}
		return w.backTo(pageProject)
	case pagePlan:
		if len(w.asks) > 0 {
			return w.backToVariables()
		}
		return w.backTo(pageProject)
	}
	return w, nil
}

func (w *Model) backToVariables() (tea.Model, tea.Cmd) {
	w.varScreen = 0
	w.buildVarVals()
	w.page = pageVariables
	w.form = w.sized(variablesForm(w.asks, w.envs, 0, w.varVals))
	return w, w.form.Init()
}

func (w *Model) backTo(target page) (tea.Model, tea.Cmd) {
	for target >= pageArchitecture && w.flagPages[target] {
		target--
	}
	switch target {
	case pageArchitecture:
		w.page = pageArchitecture
		w.form = w.sized(architectureForm(w.manifest, w.shapes, &w.Answers))
		return w, w.form.Init()
	case pageProject:
		w.page = pageProject
		w.form = w.sized(projectForm(&w.Answers))
		return w, w.form.Init()
	}
	w.aborted = true
	return w, tea.Quit
}

func (w *Model) View() string {
	if w.page == pageDone || w.aborted {
		return ""
	}
	s := w.styles

	if w.page == pageApply {
		var b strings.Builder
		for _, st := range w.applied {
			fmt.Fprintf(&b, "✓ %s (%d files)\n", st.Unit, st.Files)
		}
		b.WriteString(w.spin.View() + " applying " + w.Answers.Dir)
		body := w.lg.NewStyle().Margin(1, 0).Render(b.String())
		return w.frame(w.appBoundaryView("Odyssey Project Wizard"), body, w.appBoundaryView(""))
	}

	v := strings.TrimSuffix(w.form.View(), "\n\n")
	if w.page == pagePlan {
		planBox := s.Status.MarginTop(0).Width(formWidth - s.Status.GetHorizontalBorderSize()).Render(
			s.StatusHeader.Render("Plan: "+w.Answers.Dir) + "\n" + strings.TrimRight(w.plan.Tree(), "\n"))
		v = lipgloss.JoinVertical(lipgloss.Left, planBox, "", v)
	}
	form := w.lg.NewStyle().Margin(1, 0).Render(v)

	body := form
	statusMarginLeft := w.width - statusWidth - s.Status.GetHorizontalBorderSize() - lipgloss.Width(form)
	if statusMarginLeft >= 1 {
		status := s.Status.
			Width(statusWidth).
			MarginLeft(statusMarginLeft).
			Render(statusContent(s, w.Answers))
		body = lipgloss.JoinHorizontal(lipgloss.Left, form, status)
	}

	errors := w.form.Errors()
	header := w.appBoundaryView("Odyssey Project Wizard")
	if len(errors) > 0 {
		header = w.appErrorBoundaryView(w.errorView())
	}

	footer := w.appBoundaryView(w.form.Help().ShortHelpView(w.form.KeyBinds()))
	if len(errors) > 0 {
		footer = w.appErrorBoundaryView("")
	}

	return w.frame(header, body, footer)
}

func (w *Model) frame(header, body, footer string) string {
	content := header + "\n" + body
	gap := 2
	if w.height > 0 {
		if g := w.height - w.styles.Base.GetVerticalFrameSize() - lipgloss.Height(content) - lipgloss.Height(footer) + 1; g > gap {
			gap = g
		}
	}
	return w.styles.Base.Render(content + strings.Repeat("\n", gap) + footer)
}

func (w *Model) errorView() string {
	var s string
	for _, err := range w.form.Errors() {
		s += err.Error()
	}
	return s
}

func (w *Model) appBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		w.width,
		lipgloss.Left,
		w.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(indigo),
	)
}

func (w *Model) appErrorBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		w.width,
		lipgloss.Left,
		w.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(red),
	)
}

func Run(templates string, m *types.Manifest, shapes []string, a render.Answers, yes bool) (*render.Plan, error) {
	w := New(templates, m, shapes, a, yes)
	out, err := tea.NewProgram(w, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	final := out.(*Model)
	for _, st := range final.applied {
		fmt.Printf("✓ %s (%d files)\n", st.Unit, st.Files)
	}
	if final.err != nil {
		return nil, final.err
	}
	if final.aborted || final.page != pageDone {
		return nil, ErrAborted
	}
	return final.plan, nil
}
