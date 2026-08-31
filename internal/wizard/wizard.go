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
	width   int
	height  int
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
		w.page = pageDone
		return w, tea.Quit
	}
	return w, nil
}

func (w *Model) back() (tea.Model, tea.Cmd) {
	if w.page == pageArchitecture {
		w.aborted = true
		return w, tea.Quit
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
