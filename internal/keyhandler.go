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

	case tea.KeyEnter:
		return handleEnterKey(m)

	}
	return nil
}

// Handle enter key. The enter key sends the message in the input
// text area to the serial port. If sent was successfull,
// further handling like store the command in the command history and
// print it to the message window will be done in the event loop.
func handleEnterKey(m *model) tea.Cmd {
	userInput := m.input.Ta.Value()
	if userInput == "" {
		return nil
	}

	// Add command to history.
	m.cmdhist.AddCmd(userInput)

	return SendToPort(*m.port, userInput)
}
