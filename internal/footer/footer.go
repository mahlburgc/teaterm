package footer

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	width int
	help  help.Model
}

func New() (m Model) {
	m.help = help.New()
	m.help.ShowAll = false
	m.help.Styles.ShortKey = styles.HelpKey
	m.help.Styles.ShortDesc = styles.HelpDesc
	m.help.Styles.ShortSeparator = styles.HelpSep
	return m
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m Model) GetHeight() int {
	return lipgloss.Height(m.View(""))
}

func (m Model) View(connection string) string {
	helpText := " | "
	m.help.Width = m.width - lipgloss.Width(connection) - lipgloss.Width(helpText)

	helpText += m.help.View(keymap.Default)

	return lipgloss.NewStyle().MaxWidth(m.width).Render(connection +
		styles.FooterStyle.Render(helpText))
}
