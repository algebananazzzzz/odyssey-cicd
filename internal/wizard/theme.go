package wizard

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/algebananazzzzz/odyssey/internal/render"
)

const minPanelWidth = 60

var (
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			Padding(0, 1).MarginLeft(3).Width(24)
	panelTitle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	appStyle   = lipgloss.NewStyle().Padding(1, 2)
)

func theme() *huh.Theme {
	return huh.ThemeCharm()
}

func statusPanel(a render.Answers) string {
	var b strings.Builder
	b.WriteString(panelTitle.Render("odyssey") + "\n\n")
	row := func(label, value string) {
		if value == "" {
			value = dimStyle.Render("—")
		}
		b.WriteString(lipgloss.NewStyle().Width(6).Faint(true).Render(label) + " " + value + "\n")
	}
	row("cloud", a.Provider)
	row("arch", a.Architecture)
	row("stack", a.Stack)
	row("envs", a.Environments)
	row("proj", a.Project)
	return panelStyle.Render(strings.TrimRight(b.String(), "\n"))
}
