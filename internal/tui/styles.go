package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/render"
)

const (
	maxWidth    = 120
	formWidth   = 45
	statusWidth = 26
)

var (
	red    = lipgloss.AdaptiveColor{Light: "#FE5F86", Dark: "#FE5F86"}
	indigo = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	green  = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
)

type Styles struct {
	Base,
	HeaderText,
	Status,
	StatusHeader,
	Highlight,
	ErrorHeaderText,
	Help lipgloss.Style
}

func NewStyles(lg *lipgloss.Renderer) *Styles {
	s := Styles{}
	s.Base = lg.NewStyle().
		Padding(1, 4, 0, 1)
	s.HeaderText = lg.NewStyle().
		Foreground(indigo).
		Bold(true).
		Padding(0, 1, 0, 2)
	s.Status = lg.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(indigo).
		PaddingLeft(1).
		MarginTop(1)
	s.StatusHeader = lg.NewStyle().
		Foreground(green).
		Bold(true)
	s.Highlight = lg.NewStyle().
		Foreground(lipgloss.Color("212"))
	s.ErrorHeaderText = s.HeaderText.
		Foreground(red)
	s.Help = lg.NewStyle().
		Foreground(lipgloss.Color("240"))
	return &s
}

func statusContent(s *Styles, a render.Answers) string {
	var b strings.Builder
	b.WriteString(s.StatusHeader.Render("odyssey") + "\n")
	row := func(label, value string) {
		if value == "" {
			value = s.Help.Render("—")
		}
		b.WriteString(s.Help.Render(label) + strings.Repeat(" ", 6-len(label)) + value + "\n")
	}
	row("cloud", a.Provider)
	row("arch", a.Architecture)
	row("stack", a.Stack)
	row("envs", a.Environments)
	row("proj", a.Project)
	dir := a.Dir
	if dir == "" && a.Project != "" {
		dir = "./" + a.Project
	}
	row("dir", dir)
	return strings.TrimRight(b.String(), "\n")
}
