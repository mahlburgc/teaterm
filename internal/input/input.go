// Package input provides the text input view for transmitting data to the serial port.
// A input suggestion will be shown as shadow text if the current input matches
// the beginning of an already sent command from the command history.
package input

import (
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
	ta              textarea.Model
	inputSuggestion string
	width           int
}

func New() (m Model) {
	m.ta = textarea.New()
	m.ta.SetWidth(30)
	m.ta.SetHeight(1)
	m.ta.Placeholder = "Send a message..."
	m.ta.Focus()
	m.ta.Prompt = "> "
	m.ta.CharLimit = 256
	m.ta.ShowLineNumbers = false
	m.ta.KeyMap.InsertNewline.SetEnabled(false)
	m.ta.Cursor.Style = styles.CursorStyle
	m.ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	m.ta.FocusedStyle.Placeholder = styles.FocusedPlaceholderStyle
	m.ta.FocusedStyle.Prompt = styles.FocusedPromtStyle
	m.ta.BlurredStyle.Prompt = styles.BlurredPromtStyle
	m.ta.FocusedStyle.Base = lipgloss.NewStyle() // No border
	m.ta.BlurredStyle.Base = lipgloss.NewStyle() // No border

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	// Ignore specific shortcuts to avoid adding the to the textarea while
	// using them for navigation.
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "alt+j", "alt+k", "alt+h", "alt+l":
			return m, nil
		}
	}

	// Capture old value to check for changes
	oldValue := m.ta.Value()

	m.ta, cmd = m.ta.Update(msg)

	// If the value changed, verify if the suggestion is still valid
	newValue := m.ta.Value()
	if oldValue != newValue {
		if !strings.HasPrefix(m.inputSuggestion, newValue) {
			m.inputSuggestion = ""
		}
	}

	// Broadcast the current ta input. The input will be parsed by the command history
	// to check for an input suggestion.
	if cmd != nil {
		switch msg.(type) {
		case cursor.BlinkMsg:
			// Do not broadcast the current ta input on blink messages to avoid spamming messages.
			return m, cmd

		default:
			// Ta input may changed, broadcast current ta input.
			var partialTxMsgCmd tea.Cmd
			inputVal := m.ta.Value()
			partialTxMsgCmd = func() tea.Msg {
				return events.PartialTxMsg(inputVal)
			}
			if m.ta.Length() == 0 {
				m.inputSuggestion = "" // Clear suggestion if input is empty.
			}
			return m, tea.Batch(cmd, partialTxMsgCmd)
		}
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
			if m.ta.Value() == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return events.SendMsg{Data: m.ta.Value(), FromCmdHist: false}
			}

		case key.Matches(msg, keymap.Default.ResetKey):
			return m, m.Reset()

		case key.Matches(msg, keymap.Default.AutoCompleteKey):
			if m.inputSuggestion != "" {
				m.ta.SetValue(m.inputSuggestion)
				return m, m.ta.Focus() // Force cursor to be immediately visible
			}
			return m, nil
		}

	case events.SendMsg:
		m.inputSuggestion = ""
		return m, m.Reset()

	case events.HistCmdSelected:
		if string(msg) != "" {
			m.SetValue(string(msg))
			return m, nil
		} else {
			return m, m.Reset()
		}

	// new input suggestion received from cmd history
	case events.InputSuggestion:
		m.inputSuggestion = string(msg)
		return m, nil
	}

	return m, nil
}

// Shows the input view.
// The input view shows the current textarea input and, if available the input suggestion.
// Unfortunately we cannot just add the input suggestion to the ta because we want to show the
// suggestion with different color than the actual ta input.
// This makes things complicated here because the ta input can not be colorized partially.
// So we will build the input view from the ta.View() and the input suggestion and show it in a \
// viewport.
//
// TODO: Currently the input suggestion is only shown as long as the ta.Value() fits into the view.
// On line wrap we do not show any suggestion anymore because the view seems to get broken.
// Could be fixed / improved
func (m Model) View() string {
	var vp viewport.Model
	var content string
	vp.Height = m.ta.Height()
	vp.Width = m.width

	val := m.ta.Value()
	inputSpace := m.width - lipgloss.Width(m.ta.Prompt)

	// Check if we should show a suggestion
	if m.inputSuggestion != "" &&
		strings.HasPrefix(m.inputSuggestion, val) &&
		len(val) < inputSpace && // only show suggestion if no line wrap
		len(val) < len(m.inputSuggestion) {

		taWidth := lipgloss.Width(m.ta.Prompt+m.ta.Value()) + 1
		m.ta.SetWidth(taWidth)

		var prefix string
		var suffix string

		// Build the displayed text with input + suggestion + cursor.
		// Important: the strings include escape sequences for colors.
		// We need to use len() here several times to determine e.g. the exact cursor characters
		// to remove the old cursor and replace it with a modified one.
		// Keep in mind that these are two different things:
		//    len(m.ta.Cursor.View()) --> counts all bytes including escape sequences
		//    lipgloss.Width(m.ta.Cursor.View()) --> counts the actually displayed char
		if m.ta.LineInfo().ColumnOffset == len(val) { // cursor is at end
			insertChar := string(m.inputSuggestion[len(val)])
			if m.ta.Cursor.Blink { // cursor is not visible
				prefix = m.ta.View()
				prefix = prefix[:len(prefix)-1] + styles.InfoMsgStyle.Render(insertChar)
			} else { // cursor is visible
				prefix = m.ta.View()
				prefix = prefix[:len(prefix)-len(m.ta.Cursor.View())]
				m.ta.Cursor.SetChar(insertChar)
				prefix += m.ta.Cursor.View()
			}
			suffix = styles.InfoMsgStyle.Render(m.inputSuggestion[len(val)+1:])
		} else { // cursor is not at end
			prefix = m.ta.View()
			prefix = prefix[:len(prefix)-1]
			suffix = styles.InfoMsgStyle.Render(m.inputSuggestion[len(val):])
		}

		content = prefix + suffix
	} else {
		m.ta.SetWidth(m.width)
		content = m.ta.View()
	}

	vp.SetContent(content)
	// Uncomment lines below for better debugging
	// log.Printf("width: %v, ta.width: %v\n", m.width, m.ta.Width())
	// log.Printf("m.ta.Value: %v\n", m.ta.Value())
	// log.Printf("conntent: %s\n", content)
	return styles.AddBorder(vp, "", "", false)
}

func (m *Model) SetWidth(width int) {
	m.width = width - 2 // -2 because border will be added later
	m.ta.SetWidth(m.width)
}

func (m Model) GetHeight() int {
	return m.ta.Height() + 2 // +2 because border will be added later
}

func (m *Model) SetValue(value string) {
	m.ta.SetValue(value)
}

func (m *Model) SetDisconnectet() {
	m.ta.Reset()
	m.ta.Placeholder = "Disconnected"
	m.ta.Blur()
	m.inputSuggestion = ""
}

func (m *Model) SetConnected() tea.Cmd {
	m.ta.Reset()
	m.ta.Placeholder = "Send a message..."
	m.inputSuggestion = ""
	return m.ta.Focus()
}

func (m *Model) SetConnecting() {
	m.ta.Reset()
	m.ta.Placeholder = "Connecting..."
	m.ta.Blur()
	m.inputSuggestion = ""
}

func (m *Model) Reset() tea.Cmd {
	return m.SetConnected()
}
