package help

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	width    int
	height   int
	help     help.Model
	viewport viewport.Model
}

func New() (m Model) {
	m.help = help.New()
	m.help.ShowAll = true

	m.help.Styles.FullKey = styles.HelpKey
	m.help.Styles.FullDesc = styles.HelpDesc
	m.help.Styles.FullSeparator = styles.HelpSep

	m.viewport = viewport.New(0, 0)
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, cmd
}

func (m Model) View() string {
	m.updateViewportContent()
	return styles.HelpOverlayBorderStyle.Render(m.viewport.View())
}

func (m *Model) updateViewportContent() {
	hFrame, vFrame := styles.HelpOverlayBorderStyle.GetFrameSize()

	title := "Teaterm Keybindings\n"
	content := lipgloss.JoinVertical(lipgloss.Left, title, m.help.View(keymap.Default))
	m.viewport.SetContent(content)

	if lipgloss.Width(content) <= m.width-hFrame-2 {
		m.viewport.Width = lipgloss.Width(content)
	} else {
		m.viewport.Width = m.width - hFrame - 2
	}

	if lipgloss.Height(content) <= m.height-vFrame-2 {
		m.viewport.Height = lipgloss.Height(content)
	} else {
		m.viewport.Height = m.height - vFrame - 2
	}
}
