package input

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	Ta textarea.Model
}

// New creates a new model with default settings.
// Input text area contains text field to send commands to the serial port.
func New() (m Model) {
	m.Ta = textarea.New()
	m.Ta.SetWidth(30)
	m.Ta.SetHeight(1)
	m.Ta.Placeholder = "Send a message..."
	m.Ta.Focus()
	m.Ta.Prompt = "> "
	m.Ta.CharLimit = 256
	m.Ta.ShowLineNumbers = false
	m.Ta.KeyMap.InsertNewline.SetEnabled(false)
	m.Ta.Cursor.Style = styles.CursorStyle
	m.Ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	m.Ta.FocusedStyle.Placeholder = styles.FocusedPlaceholderStyle
	m.Ta.FocusedStyle.Prompt = styles.FocusedPromtStyle
	m.Ta.BlurredStyle.Prompt = styles.BlurredPromtStyle
	m.Ta.FocusedStyle.Base = styles.FocusedBorderStyle
	m.Ta.BlurredStyle.Base = styles.BlurredBorderStyle

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	// ignore specific shortcuts
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "alt+j", "alt+k", "alt+h", "alt+l":
			return m, nil
		}
	}

	m.Ta, cmd = m.Ta.Update(msg)
	if cmd != nil {
		return m, cmd
	}

	switch msg := msg.(type) {

	case events.ConnectionStatusMsg:
		switch msg.Status {
		case events.Disconnected:
			m.SetDisconnectet()

		case events.Connected:
			cmd = m.SetConnected()

		case events.Connecting:
			m.SetConnecting()
		}
		return m, cmd

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.SendKey):
			if m.Ta.Value() == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return events.SendMsg{Data: m.Ta.Value(), FromCmdHist: false}
			}

		case key.Matches(msg, keymap.Default.ResetKey):
			return m, m.Reset()
		}

	case events.SendMsg:
		return m, m.Reset()

	case events.HistCmdSelected:
		if string(msg) == "" {
			return m, m.Reset()
		} else {
			m.SetValue(string(msg))
			return m, nil
		}

	}

	return m, nil
}

func (m Model) View() string {
	return m.Ta.View()
}

func (m *Model) SetWidth(width int) {
	m.Ta.SetWidth(width)
}

func (m Model) GetHeight() int {
	return lipgloss.Height(m.View())
}

func (m *Model) SetValue(value string) {
	m.Ta.SetValue(value)
}

func (m *Model) SetDisconnectet() {
	m.Ta.Reset()
	m.Ta.Placeholder = "Disconnected"
	m.Ta.Blur()
}

func (m *Model) SetConnected() tea.Cmd {
	m.Ta.Reset()
	m.Ta.Placeholder = "Send a message..."
	return m.Ta.Focus()
}

func (m *Model) SetConnecting() {
	m.Ta.Reset()
	m.Ta.Placeholder = "Connecting..."
	m.Ta.Blur()
}

func (m *Model) Reset() tea.Cmd {
	return m.SetConnected()
}
