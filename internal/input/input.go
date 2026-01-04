package input

import (
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	Ta              textarea.Model
	inputSuggestion string
	width           int
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
	m.Ta.FocusedStyle.Base = lipgloss.NewStyle() // No border
	m.Ta.BlurredStyle.Base = lipgloss.NewStyle()

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

	// Capture old value to check for changes
	oldValue := m.Ta.Value()

	m.Ta, cmd = m.Ta.Update(msg)

	// If the value changed, verify if the suggestion is still valid
	newValue := m.Ta.Value()
	if oldValue != newValue {
		if !strings.HasPrefix(m.inputSuggestion, newValue) {
			m.inputSuggestion = ""
		}
	}

	if cmd != nil {
		switch msg.(type) {
		case cursor.BlinkMsg:
			// ignore blink messages
			return m, cmd

		default:
			var partialTxMsgCmd tea.Cmd
			if m.Ta.Length() > 0 {
				inputVal := m.Ta.Value()
				partialTxMsgCmd = func() tea.Msg {
					return events.PartialTxMsg(inputVal)
				}
				return m, tea.Batch(cmd, partialTxMsgCmd)
			} else {
				m.inputSuggestion = "" // Clear suggestion if input is empty
			}
		}
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

		case key.Matches(msg, keymap.Default.AutoCompleteKey):
			if m.inputSuggestion != "" {
				m.Ta.SetValue(m.inputSuggestion)
				return m, m.Ta.Focus()
			}
			return m, nil
		}

	case events.SendMsg:
		m.inputSuggestion = ""
		return m, m.Reset()

	case events.HistCmdSelected:
		if string(msg) == "" {
			return m, m.Reset()
		} else {
			m.SetValue(string(msg))
			return m, nil
		}

	case events.InputSuggestion:
		m.inputSuggestion = string(msg)
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	var vp viewport.Model
	var content string
	vp.Height = m.Ta.Height()
	vp.Width = m.width

	val := m.Ta.Value()
	m.Ta.SetWidth(m.width)

	// Check if we should show a suggestion
	if m.inputSuggestion != "" && strings.HasPrefix(m.inputSuggestion, val) && len(val) < len(m.inputSuggestion) {

		taWidth := lipgloss.Width(m.Ta.Prompt+m.Ta.Value()) + 1
		m.Ta.SetWidth(taWidth)

		var prefix string
		var suffix string

		if m.Ta.LineInfo().ColumnOffset == len(val) { // cursor is at end
			insert := string(m.inputSuggestion[len(val)])
			log.Printf("Char: %v\n", insert)
			if m.Ta.Cursor.Blink { // cursor is not visible
				prefix = m.Ta.View()
				prefix = prefix[:len(prefix)-1] + styles.InfoMsgStyle.Render(insert)
			} else { // cursor is visible
				prefix = m.Ta.View()
				prefix = prefix[:len(prefix)-len(m.Ta.Cursor.View())]
				m.Ta.Cursor.SetChar(insert)
				prefix += m.Ta.Cursor.View()
			}
			suffix = styles.InfoMsgStyle.Render(m.inputSuggestion[len(val)+1:])
		} else { // cursor is not at end
			prefix = m.Ta.View()
			prefix = prefix[:len(prefix)-1]
			suffix = styles.InfoMsgStyle.Render(m.inputSuggestion[len(val):])
		}

		content = prefix + suffix
	} else {
		content = m.Ta.View()
	}

	vp.SetContent(content)
	log.Printf("width: %v, ta.width: %v\n", m.width, m.Ta.Width())
	log.Printf("conntent: %s\n", content)
	return styles.AddBorder(vp, "", "")
}

func (m *Model) SetWidth(width int) {
	m.width = width - 2 //-2 because border will be added later
	m.Ta.SetWidth(m.width)
}

func (m Model) GetHeight() int {
	return 3 // fixed 1 line input + border
}

func (m *Model) SetValue(value string) {
	m.Ta.SetValue(value)
}

func (m *Model) SetDisconnectet() {
	m.Ta.Reset()
	m.Ta.Placeholder = "Disconnected"
	m.Ta.Blur()
	m.inputSuggestion = ""
}

func (m *Model) SetConnected() tea.Cmd {
	m.Ta.Reset()
	m.Ta.Placeholder = "Send a message..."
	m.inputSuggestion = ""
	return m.Ta.Focus()
}

func (m *Model) SetConnecting() {
	m.Ta.Reset()
	m.Ta.Placeholder = "Connecting..."
	m.Ta.Blur()
	m.inputSuggestion = ""
}

func (m *Model) Reset() tea.Cmd {
	return m.SetConnected()
}
