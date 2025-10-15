package internal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Handle all key events in the bubbletea update loop.
func HandleKeys(m *model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "alt+m":
		// nothing to do for now
		return nil
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		StoreConfig(m.cmdHist)
		return tea.Quit

	case tea.KeyPgUp:
		m.serialVp.ScrollUp(3)
		return nil

	case tea.KeyPgDown:
		m.serialVp.ScrollDown(3)
		return nil

	case tea.KeyCtrlD:
		deleteCmdFromCmdHist(m)
		return nil

	case tea.KeyUp:
		scrollCmdHistUp(m)
		return nil

	case tea.KeyDown:
		scrollCmdHistDown(m)
		return nil

	case tea.KeyEnter:
		return handleEnterKey(m)
	}

	return nil
}

// Delete cmd from command history, reset cmd hist index and reset input
// text area.
func deleteCmdFromCmdHist(m *model) {
	if m.cmdHistIndex != len(m.cmdHist) {
		m.cmdHist = append(m.cmdHist[:m.cmdHistIndex], m.cmdHist[m.cmdHistIndex+1:]...)
		m.cmdHistIndex = len(m.cmdHist)
		resetVp(&m.cmdVp, &m.cmdHist, false)
		m.inputTa.Reset()
	}
}

// Scroll cmd history up.
func scrollCmdHistUp(m *model) {
	if m.cmdHistIndex > 0 {
		m.cmdHistIndex--
	}
	if m.cmdHistIndex < m.cmdVp.YOffset {
		m.cmdVp.ScrollUp(1)
	}
	updateCmdHistView(m)
}

// Scroll cmd history down.
func scrollCmdHistDown(m *model) {
	if m.cmdHistIndex < len(m.cmdHist) {
		m.cmdHistIndex++
		if m.cmdHistIndex < len(m.cmdHist) {
			// The bottom-most visible line is at YOffset + Height - 1.
			bottomEdge := m.cmdVp.YOffset + m.cmdVp.Height - 1
			// If the selection is now below the visible area of the viewport,
			// scroll the viewport down to keep it in view.
			if m.cmdHistIndex > bottomEdge {
				m.cmdVp.ScrollDown(1)
			}
			updateCmdHistView(m)
		} else {
			// reached end of cmd history
			m.inputTa.Reset()
			m.cmdVp.SetContent(lipgloss.NewStyle().Render(strings.Join(m.cmdHist, "\n")))
		}
	}
}

func updateCmdHistView(m *model) {
	m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
	m.inputTa.SetCursor(len(m.inputTa.Value()))

	// apply style for selected command in command history view
	// Create the slice with a known size to prevent reallocations in the loop
	cmdHistLines := make([]string, len(m.cmdHist))
	for i, cmd := range m.cmdHist {
		if i == m.cmdHistIndex {
			cmdHistLines[i] = SelectedCmdStyle.Render("> " + cmd)
		} else {
			cmdHistLines[i] = cmd
		}
	}
	// // Use a strings.Builder for the most efficient way to build the view
	// var b strings.Builder
	// for i, cmd := range m.cmdHist {
	// 	if i > 0 {
	// 		b.WriteString("\n")
	// 	}
	// 	if i == m.cmdHistIndex {
	// 		b.WriteString(SelectedCmdStyle.Render("> " + cmd))
	// 	} else {
	// 		b.WriteString(cmd)
	// 	}
	// }

	m.cmdVp.SetContent(lipgloss.NewStyle().Render(strings.Join(cmdHistLines, "\n")))
}

// Handle enter key. The enter key sends the message in the input
// text area to the serial port. If sent was successfull,
// further handling like store the command in the command history and
// print it to the message window will be done in the event loop.
func handleEnterKey(m *model) tea.Cmd {
	userInput := m.inputTa.Value()
	if userInput == "" {
		return nil
	}

	return SendToPort(m.port, userInput)
}
