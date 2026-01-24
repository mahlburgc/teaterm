package msglog

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/icza/gox/stringsx"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	Vp            viewport.Model
	sendStyle     lipgloss.Style
	errStyle      lipgloss.Style
	infoStyle     lipgloss.Style
	log           []string
	showTimestamp bool
	serialLog     *log.Logger
	txPrefix      string
	rxPrefix      string
	errPrefix     string
	infoPrefix    string
	showEscapes   bool
	logLimit      int
}

// This message is sent when the editor is closed.
type EditorFinishedMsg struct {
	err error
}

const (
	rxMsg = iota
	txMsg
	errMsg
	infoMsg
)

// New creates a new model with default settings.
func New(showTimestamp bool, showEscapes bool, sendStyle lipgloss.Style,
	errStyle lipgloss.Style, infoStyle lipgloss.Style, serialLog *log.Logger, logLimit int,
) (m Model) {
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
	m.errPrefix = "ERROR: "
	m.infoPrefix = "INFO: "
	m.serialLog = serialLog
	m.showEscapes = showEscapes
	m.logLimit = logLimit

	m.sendStyle = sendStyle
	m.errStyle = errStyle
	m.infoStyle = infoStyle
	m.showTimestamp = showTimestamp

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.Vp, cmd = m.Vp.Update(msg)
	if cmd != nil {
		return m, cmd
	}

	switch msg := msg.(type) {

	case events.SendMsg:
		m.addMsg(msg.Data, txMsg)

	case events.SerialRxMsgReceived:
		m.addMsg(string(msg), rxMsg)

	case events.ErrMsg:
		if msg != nil {
			m.addMsg(msg.Error(), errMsg)
		}
		return m, nil

	case events.InfoMsg:
		m.addMsg(string(msg), infoMsg)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.LogLeftKey):
			m.Vp.ScrollLeft(3)

		case key.Matches(msg, keymap.Default.LogRightKey):
			m.Vp.ScrollRight(3)

		case key.Matches(msg, keymap.Default.LogUpKey):
			m.Vp.ScrollUp(1)

		case key.Matches(msg, keymap.Default.LogDownKey):
			m.Vp.ScrollDown(1)

		case key.Matches(msg, keymap.Default.LogUpFastKey):
			m.Vp.ScrollUp(10)

		case key.Matches(msg, keymap.Default.LogDownFastKey):
			m.Vp.ScrollDown(10)

		case key.Matches(msg, keymap.Default.LogTopKey):
			m.Vp.GotoTop()

		case key.Matches(msg, keymap.Default.LogBottomKey):
			m.Vp.GotoBottom()

		case key.Matches(msg, keymap.Default.OpenEditorKey):
			return m, openEditorCmd(m.log)

		case key.Matches(msg, keymap.Default.ClearLogKey):
			if m.Vp.Height > 0 {
				m.log = nil /* reset serial message log */
				m.Vp.SetContent("")
				m.Vp.GotoBottom()
			}
		}

	default:
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	// mark scroll percentage if we are not at bottom
	// also, if we are not at bottom but at 100 percent scroll
	// map 100 percent to 99 percent for better user experience
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	var percentRenderStyle lipgloss.Style
	scrollPercentage := m.GetScrollPercent()
	if m.Vp.AtBottom() {
		percentRenderStyle = borderStyle
	} else {
		percentRenderStyle = styles.CursorStyle
	}

	log.Printf("msglog: scrollperc: %v\n", m.Vp.ScrollPercent()*100)
	log.Printf("msglog: scrollperctransformed: %v\n", scrollPercentage)
	log.Printf("msglog: scrollperctransformedint: %v\n", int(scrollPercentage))

	scrollPercentageString := percentRenderStyle.Render(fmt.Sprintf("%3d%%", int(scrollPercentage)))

	footer := borderStyle.Render(fmt.Sprintf("%v/%v, ", m.GetLen(), m.logLimit)) + scrollPercentageString
	return styles.AddBorder(m.Vp, "Messages", footer, true)
}

func (m *Model) SetSize(width, height int) {
	borderWidth, borderHeight := styles.FocusedBorderStyle.GetFrameSize()

	m.Vp.Width = width - borderWidth
	m.Vp.Height = height - borderHeight

	m.UpdateVp()
}

// Log a message to the viewport
func (m *Model) addMsg(msg string, msgType int) {
	var line strings.Builder
	if m.showTimestamp {
		t := time.Now().Format("15:04:05.000")
		line.WriteString(fmt.Sprintf("[%s] ", t))
	}

	switch msgType {
	case txMsg:
		line.WriteString(m.txPrefix)
	case errMsg:
		line.WriteString(m.errPrefix)
	case infoMsg:
		line.WriteString(m.infoPrefix)
	default:
		line.WriteString(m.rxPrefix)
	}

	if m.showEscapes {
		line.WriteString(stringsx.Clean(msg))
	} else {
		line.WriteString(msg) // fmt.Printf("%q", msg")
	}

	if m.serialLog != nil {
		m.serialLog.Println(line.String())
	}

	switch msgType {
	case txMsg:
		m.log = append(m.log, m.sendStyle.Render(line.String()))
	case errMsg:
		m.log = append(m.log, m.errStyle.Render(line.String()))
	case infoMsg:
		m.log = append(m.log, m.infoStyle.Render(line.String()))
	default:
		m.log = append(m.log, line.String())
	}

	// message histrory limit, remove oldest if exceed
	if len(m.log) > m.logLimit {
		m.log = m.log[len(m.log)-m.logLimit:]
	}

	m.UpdateVp()
}

func (m *Model) UpdateVp() {
	if m.Vp.Height > 0 && len(m.log) > 0 {
		// reset viewport only if we did not scrolled up in msg history
		atBottom := m.Vp.AtBottom()                // do not use scroll percentage as this does not work reliable
		m.Vp.SetContent(strings.Join(m.log, "\n")) // TODO performance improvements possible
		if atBottom {
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
			return EditorFinishedMsg{err: err}
		}
	}

	// Write the viewport content to the temp file.
	for _, line := range content {
		if _, err = tmpFile.WriteString(stripansi.Strip(line) + "\n"); err != nil {
			return func() tea.Msg {
				return EditorFinishedMsg{err: err}
			}
		}
	}

	// Close the file so the editor can access it.
	if err := tmpFile.Close(); err != nil {
		return func() tea.Msg {
			return EditorFinishedMsg{err: err}
		}
	}

	// This is the command that will be executed.
	c := exec.Command(editor, tmpFile.Name())

	// The magic is here: tea.ExecProcess handles suspending the Bubble Tea
	// app, running the command, and then sending a message back.
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return EditorFinishedMsg{err: err}
		}

		// Clean up the temporary file.
		err = os.Remove(tmpFile.Name())

		return EditorFinishedMsg{err: err}
	})
}
