package internal

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Handle all key events in the bubbletea update loop.
func HandleKeys(m *model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+x":
		return func() tea.Msg {
			return PortManualConnectMsg(true)
		}
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		StoreConfig(m.cmdhist.GetCmdHist())
		return tea.Quit
	}
	return nil
}
