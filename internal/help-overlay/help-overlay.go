package help

import (
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/internal/keymap"
)

type Model struct {
	width  int
	height int
	help   help.Model
}

func New() (m Model) {
	m.help = help.New()
	m.help.ShowAll = true
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m Model) View() string {
	foreStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1)

	boldStyle := lipgloss.NewStyle().Bold(true)
	title := boldStyle.Render("Teaterm Keybindings\n")
	m.help.Width = m.width
	layout := lipgloss.JoinVertical(lipgloss.Left, title, m.help.View(keymap.Default))

	return foreStyle.Render(layout)
}
