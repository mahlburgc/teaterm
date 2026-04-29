package footer

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	width   int
	help    help.Model
	version string
}

func New(version string) (m Model) {
	m.help = help.New()
	m.help.ShowAll = false
	m.help.Styles.ShortKey = styles.HelpKey
	m.help.Styles.ShortDesc = styles.HelpDesc
	m.help.Styles.ShortSeparator = styles.HelpSep
	m.version = version
	return m
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m Model) GetHeight() int {
	return lipgloss.Height(m.View(""))
}

func (m Model) View(connection string) string {
	versionText := styles.VersionStyle.Render(" " + m.version)
	helpPrefix := " | "

	middleWidth := m.width - lipgloss.Width(connection) - lipgloss.Width(versionText)
	if middleWidth < 0 {
		middleWidth = 0
	}

	// help.Width <= 0 disables truncation in the bubbles help library, which
	// would let the help text overflow and push the version off-screen.
	helpBudget := middleWidth - lipgloss.Width(helpPrefix)
	if helpBudget < 1 {
		helpBudget = 1
	}
	m.help.Width = helpBudget

	helpRendered := styles.FooterStyle.Render(helpPrefix + m.help.View(keymap.Default))
	helpRendered = lipgloss.NewStyle().MaxWidth(middleWidth).Render(helpRendered)

	padWidth := middleWidth - lipgloss.Width(helpRendered)
	if padWidth < 0 {
		padWidth = 0
	}

	return connection + helpRendered + strings.Repeat(" ", padWidth) + versionText
}
