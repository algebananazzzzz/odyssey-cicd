package wizard

import (
	"errors"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/types"
	"github.com/algebananazzzzz/odyssey/internal/utils"
)

var ErrAborted = errors.New("aborted")

type Selection struct {
	Environments string
	Provider     types.Provider
	Architecture types.Architecture
	Stack        types.Stack
}

func Shapes(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var shapes []string
	for _, e := range entries {
		if e.IsDir() {
			shapes = append(shapes, e.Name())
		}
	}
	return shapes, nil
}

type step int

const (
	stepEnvironments step = iota
	stepProvider
	stepArchitecture
	stepStack
	stepDone
)

var titles = map[step]string{
	stepEnvironments: "Environments",
	stepProvider:     "Provider",
	stepArchitecture: "Architecture",
	stepStack:        "Stack",
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	helpStyle    = lipgloss.NewStyle().Faint(true)
	appStyle     = lipgloss.NewStyle().Padding(1, 2)
	sidebarStyle = lipgloss.NewStyle().Width(20).PaddingRight(2).MarginRight(3).
			Border(lipgloss.NormalBorder(), false, true, false, false)
)

type model struct {
	manifest *types.Manifest
	shapes   []string
	step     step
	options  []string
	cursor   int
	sel      Selection
	aborted  bool
	width    int
	height   int
}

func (m *model) load() {
	m.cursor = 0
	m.options = nil
	switch m.step {
	case stepEnvironments:
		m.options = m.shapes
	case stepProvider:
		for _, p := range utils.Sorted(m.manifest.Providers) {
			m.options = append(m.options, string(p))
		}
	case stepArchitecture:
		for _, a := range utils.Sorted(m.manifest.Architectures) {
			if m.manifest.Architectures[a].Provider == m.sel.Provider {
				m.options = append(m.options, string(a))
			}
		}
	case stepStack:
		for _, s := range utils.Sorted(m.manifest.Stacks) {
			if slices.Contains(m.manifest.Stacks[s].Architectures, m.sel.Architecture) {
				m.options = append(m.options, string(s))
			}
		}
	}
}

func (m model) answer(s step) string {
	switch s {
	case stepEnvironments:
		return m.sel.Environments
	case stepProvider:
		return string(m.sel.Provider)
	case stepArchitecture:
		return string(m.sel.Architecture)
	case stepStack:
		return string(m.sel.Stack)
	}
	return ""
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.options) == 0 {
				return m, nil
			}
			choice := m.options[m.cursor]
			switch m.step {
			case stepEnvironments:
				m.sel.Environments = choice
			case stepProvider:
				m.sel.Provider = types.Provider(choice)
			case stepArchitecture:
				m.sel.Architecture = types.Architecture(choice)
			case stepStack:
				m.sel.Stack = types.Stack(choice)
			}
			m.step++
			if m.step == stepDone {
				return m, tea.Quit
			}
			(&m).load()
		}
	}
	return m, nil
}

func (m model) sidebar() string {
	var b strings.Builder
	for s := stepEnvironments; s < stepDone; s++ {
		switch {
		case s < m.step:
			b.WriteString(helpStyle.Render(titles[s]) + "\n")
			b.WriteString(cursorStyle.Render("  "+m.answer(s)) + "\n\n")
		case s == m.step:
			b.WriteString(titleStyle.Render(titles[s]) + "\n")
			b.WriteString(cursorStyle.Render("  ❯") + "\n\n")
		default:
			b.WriteString(helpStyle.Render(titles[s]) + "\n\n\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) question() string {
	v := titleStyle.Render(titles[m.step]) + "\n\n"
	for i, opt := range m.options {
		if i == m.cursor {
			v += cursorStyle.Render("❯ "+opt) + "\n"
		} else {
			v += "  " + opt + "\n"
		}
	}
	return v + "\n" + helpStyle.Render("↑/↓ move · enter select · q quit")
}

func (m model) View() string {
	if m.step == stepDone || m.aborted {
		return ""
	}
	sb := sidebarStyle
	if m.height > 0 {
		sb = sb.Height(m.height - 2)
	}
	return appStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, sb.Render(m.sidebar()), m.question()),
	)
}

func Run(manifest *types.Manifest, shapes []string) (Selection, error) {
	m := model{manifest: manifest, shapes: shapes}
	m.load()
	out, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Selection{}, err
	}
	final := out.(model)
	if final.aborted {
		return Selection{}, ErrAborted
	}
	if final.step != stepDone {
		return Selection{}, errors.New("input closed before the selection finished")
	}
	return final.sel, nil
}
