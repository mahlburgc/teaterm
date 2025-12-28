package footer

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/internal/styles"
)

const (
	ConStatusDisconnected = iota
	ConStatusConnecting
	ConStatusConnected
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

func (m Model) View(portName string, conStatus int, spinner spinner.Model) string {
	helpText := portName + " | ?: help · ↑/↓: cmds · PgUp/PgDn: scroll · ctrl+e: editor"

	var connectionSymbol string

	switch conStatus {
	case ConStatusConnected:
		connectionSymbol = fmt.Sprintf(" %s ", styles.ConnectSymbolStyle.Render("●"))

	case ConStatusDisconnected:
		connectionSymbol = fmt.Sprintf(" %s ", styles.DisconnectedSymbolStyle.Render("●"))

	case ConStatusConnecting:
		connectionSymbol = fmt.Sprintf(" %s", spinner.View())
	}

	connectionSymbol = zone.Mark("consymbol", connectionSymbol)

	return lipgloss.NewStyle().MaxWidth(m.width).Render(connectionSymbol + styles.FooterStyle.Render(helpText))
}
