package footer

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	width int
}

func New() Model {
	return Model{}
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m Model) GetHeight() int {
	return lipgloss.Height(m.View(""))
}

func (m Model) View(connection string) string {
	helpText := " | ctrl+q: quit · ↑/↓: cmds · PgUp/PgDn: scroll · ctrl+e: editor"

	return lipgloss.NewStyle().MaxWidth(m.width).Render(connection +
		styles.FooterStyle.Render(helpText))
}
