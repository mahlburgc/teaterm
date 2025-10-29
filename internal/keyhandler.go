package internal

import (
	"os"
	"os/exec"
	"strings"

	"github.com/acarl005/stripansi"
	tea "github.com/charmbracelet/bubbletea"
)

// This message is sent when the editor is closed.
type editorFinishedMsg struct {
	err error
}

// Handle all key events in the bubbletea update loop.
func HandleKeys(m *model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+e":
		return openEditorCmd(m.serMsg)

	case "ctrl+x":
		return func() tea.Msg {
			return PortManualConnectMsg(true)
		}

	case "ctrl+left":
		m.serialVp.ScrollLeft(3)
		return nil

	case "ctrl+right":
		m.serialVp.ScrollRight(3)
		return nil

	case "ctrl+up":
		m.serialVp.ScrollUp(3)
		return nil

	case "ctrl+down":
		m.serialVp.ScrollDown(3)
		return nil

	case "home":
		m.serialVp.GotoTop()
		return nil

	case "end":
		m.serialVp.GotoBottom()
		return nil
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		StoreConfig(m.cmdhist.GetCmdHist())
		return tea.Quit

	case tea.KeyPgUp:
		m.serialVp.ScrollUp(10)
		return nil

	case tea.KeyPgDown:
		m.serialVp.ScrollDown(10)
		return nil

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
	userInput := m.inputTa.Value()
	if userInput == "" {
		return nil
	}

	return SendToPort(*m.port, userInput)
}

// openEditorCmd creates a tea.Cmd that runs the editor.
func openEditorCmd(content []string) tea.Cmd {
	// Get the editor from the environment variable. Default to vim.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Create a temporary file to store the content.
	tmpFile, err := os.CreateTemp("", "bubbletea-edit-*.txt")
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	// Write the viewport content to the temp file.
	if _, err := tmpFile.WriteString(stripansi.Strip(strings.Join(content, "\n") + "\n")); err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	// Close the file so the editor can access it.
	if err := tmpFile.Close(); err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	// This is the command that will be executed.
	c := exec.Command(editor, tmpFile.Name())

	// The magic is here: tea.ExecProcess handles suspending the Bubble Tea
	// app, running the command, and then sending a message back.
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return editorFinishedMsg{err: err}
		}

		// Clean up the temporary file.
		err = os.Remove(tmpFile.Name())

		return editorFinishedMsg{err: err}
	})
}
