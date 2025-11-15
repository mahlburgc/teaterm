package msglog

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/icza/gox/stringsx"
)

type Model struct {
	Vp            viewport.Model
	sendStyle     lipgloss.Style
	log           []string
	showTimestamp bool
	serialLog     *log.Logger
	txPrefix      string
	rxPrefix      string
	showEscapes   bool
}

// New creates a new model with default settings.
func New(showTimestamp bool, showEscapes bool, sendStyle lipgloss.Style, serialLog *log.Logger) (m Model) {
	// Serial viewport contains all sent and received messages.
	// We will create a viewport without border and later manually
	// add the border to inject a title into the border.
	m.Vp = viewport.New(30, 5)
	m.Vp.SetContent(`Welcome to teaterm!`)
	m.Vp.Style = lipgloss.NewStyle()
	// Disable the viewport's default up/down key handling so it doesn't scroll
	// when we are navigating through the command history.
	m.Vp.KeyMap.Up.SetEnabled(false)
	m.Vp.KeyMap.Down.SetEnabled(false)
	m.Vp.KeyMap.PageUp.SetEnabled(false)
	m.Vp.KeyMap.PageDown.SetEnabled(false)

	m.log = []string{}

	m.txPrefix = ""
	m.rxPrefix = ""
	m.serialLog = serialLog

	m.sendStyle = sendStyle
	m.showTimestamp = showTimestamp

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:

		switch msg.String() {
		case "ctrl+left":
			m.Vp.ScrollLeft(3)
			return m, nil

		case "ctrl+right":
			m.Vp.ScrollRight(3)
			return m, nil

		case "ctrl+up":
			m.Vp.ScrollUp(3)
			return m, nil

		case "ctrl+down":
			m.Vp.ScrollDown(3)
			return m, nil

		case "home":
			m.Vp.GotoTop()
			return m, nil

		case "end":
			m.Vp.GotoBottom()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyPgUp:
			m.Vp.ScrollUp(10)
			return m, nil

		case tea.KeyPgDown:
			m.Vp.ScrollDown(10)
			return m, nil

		case tea.KeyCtrlL:
			if m.Vp.Height > 0 {
				m.log = nil /* reset serial message log */
				m.Vp.SetContent("")
				m.Vp.GotoBottom()
			}
			return m, nil
		}

	case tea.MouseMsg:
		return m, nil

	default:
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	return m.Vp.View()
}

// Log a message to the viewport
func (m *Model) AddMsg(msg string, isTxMsg bool) {
	var line strings.Builder
	if m.showTimestamp {
		t := time.Now().Format("15:04:05.000")
		line.WriteString(fmt.Sprintf("[%s] ", t))
	}

	if isTxMsg {
		line.WriteString(m.txPrefix)
	} else {
		line.WriteString(m.rxPrefix)
	}

	if m.showEscapes {
		line.WriteString(fmt.Sprintf("%q", msg))
	} else {
		line.WriteString(stringsx.Clean(msg))
	}

	if m.serialLog != nil {
		m.serialLog.Println(line.String())
	}

	// TODO set serial message histrory limit, remove oldest if exceed
	if isTxMsg {
		m.log = append(m.log, m.sendStyle.Render(line.String()))
	} else {
		m.log = append(m.log, lipgloss.NewStyle().Render(line.String()))
	}

	m.UpdateVp()
}

func (m *Model) UpdateVp() {
	if m.Vp.Height > 0 && len(m.log) > 0 {
		// reset viewport only if we did not scrolled up in msg history
		goToBottom := m.Vp.ScrollPercent() == 1
		m.Vp.SetContent(lipgloss.NewStyle().Render(strings.Join(m.log, "\n")))
		if goToBottom {
			m.Vp.GotoBottom()
		}

	}
}

func (m Model) GetLen() int {
	return len(m.log)
}

func (m Model) GetScrollPercent() float64 {
	return m.Vp.ScrollPercent() * 100
}
