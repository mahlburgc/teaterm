package cmdhist

import (
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// This message is sent when a cmd was selected.
type CmdHistMsg struct {
	Type MsgType
	Cmd  string
}

type MsgType int

const (
	CmdSelected MsgType = 0
	CmdExecuted MsgType = 1
)

type Model struct {
	Vp           viewport.Model
	SelectStyle  lipgloss.Style
	cmdHist      []string
	cmdHistIndex int
}

// New creates a new model with default settings.
// Command history can be passed to start with existing commands.
func New(cmdHist []string) (m Model) {
	m.Vp = viewport.New(30, 5)
	m.SelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.cmdHistIndex = len(m.cmdHist)

	if cmdHist == nil {
		return m
	}

	for _, cmd := range cmdHist {
		if cmd != "" {
			m.cmdHist = append(m.cmdHist, cmd)
		}
	}
	return m
}

func (m Model) GetIndex() int {
	return m.cmdHistIndex
}

func (m Model) GetHistLen() int {
	return len(m.cmdHist)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {

		case tea.KeyCtrlD:
			return m, m.deleteCmd()

		case tea.KeyUp:
			return m, m.scrollUp()

		case tea.KeyDown:
			return m, m.scrollDown()

		default:
			return m, nil
		}

	case tea.MouseMsg:
		for i := range m.cmdHist {
			if zone.Get(strconv.Itoa(i)).InBounds(msg) {
				m.cmdHistIndex = i
			}
		}

		var c tea.Cmd
		c = m.updateCmdHistView()

		if m.cmdHistIndex == len(m.cmdHist) {
			return m, c
		}

		if msg.Button != tea.MouseButtonLeft {
			return m, c
		}

		if msg.Action == tea.MouseActionPress {
			return m, c
		}

		if msg.Action == tea.MouseActionRelease {
			return m, SendCmdExecutedMsg(m.cmdHist[m.cmdHistIndex])
		}

		// x, y := zone.Get("confirm").Pos() can be used to get the relative
		// coordinates within the zone. Useful if you need to move a cursor in a
		// input box as an example.

		return m, nil

	default:
		return m, nil
	}
}

// Returns a Tea command to send a message with the mouse selected cmd to the event loop.
func SendCmdSelectedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return CmdHistMsg{
			Type: CmdSelected,
			Cmd:  cmd,
		}
	}
}

// Returns a Tea command to send a message with the arrow selected cmd to the event loop.
func SendCmdExecutedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return CmdHistMsg{
			Type: CmdExecuted,
			Cmd:  cmd,
		}
	}
}

// View renders the model's view.
func (m Model) View() string {
	return m.Vp.View()
}

// Scroll up cmd view.
func (m *Model) scrollUp() tea.Cmd {
	if m.cmdHistIndex > 0 {
		m.cmdHistIndex--
	}
	if m.cmdHistIndex < m.Vp.YOffset {
		m.Vp.ScrollUp(1)
	}
	return m.updateCmdHistView()
}

// Scroll down cmd view.
func (m *Model) scrollDown() (c tea.Cmd) {
	if m.cmdHistIndex < len(m.cmdHist) {
		m.cmdHistIndex++
		if m.cmdHistIndex < len(m.cmdHist) {
			// The bottom-most visible line is at YOffset + Height - 1.
			bottomEdge := m.Vp.YOffset + m.Vp.Height - 1
			// If the selection is now below the visible area of the viewport,
			// scroll the viewport down to keep it in view.
			if m.cmdHistIndex > bottomEdge {
				m.Vp.ScrollDown(1)
			}
		}
		c = m.updateCmdHistView()
	}
	return c
}

func (m *Model) updateCmdHistView() (c tea.Cmd) {
	cmdHistLines := make([]string, len(m.cmdHist))
	for i, cmd := range m.cmdHist {
		if i == m.cmdHistIndex {
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), m.SelectStyle.Render("> "+cmd))
			c = SendCmdSelectedMsg(m.cmdHist[m.cmdHistIndex])
		} else {
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), cmd)
		}
	}
	log.Printf("Testtest %v", cmdHistLines)
	m.Vp.SetContent(lipgloss.NewStyle().Render(strings.Join(cmdHistLines, "\n")))

	return c
	// m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
	// m.inputTa.SetCursor(len(m.inputTa.Value()))
}

// Delete cmd from command history and reset cmd hist index.
func (m *Model) deleteCmd() (c tea.Cmd) {
	if m.cmdHistIndex != len(m.cmdHist) {
		m.cmdHist = append(m.cmdHist[:m.cmdHistIndex], m.cmdHist[m.cmdHistIndex+1:]...)
		c = m.ResetVp()
		log.Printf("command will be deleted! New Command list: %v", m.cmdHist)
	}
	return c
}

func (m *Model) ResetVp() (c tea.Cmd) {
	log.Printf("reset cmd vp: vp height, msg len: %v, %v\n", m.Vp.Height, len(m.cmdHist))

	if m.Vp.Height > 0 {
		m.cmdHistIndex = len(m.cmdHist)
		c = m.updateCmdHistView()
		m.Vp.GotoBottom()
	}
	return c
}

// Add a new command to the command history. The command will only be added, if not
// already exisiting in the hist. If cmd is found, it will be moved to the end.
func (m *Model) AddCmd(newCmd string) (c tea.Cmd) {
	log.Printf("add command: %s\n", newCmd)
	foundIndex := -1
	for i, cmd := range m.cmdHist {
		if cmd == newCmd {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		m.cmdHist = append(m.cmdHist[:foundIndex], m.cmdHist[foundIndex+1:]...)
		m.cmdHist = append(m.cmdHist, newCmd)
	} else {
		m.cmdHist = append(m.cmdHist, newCmd)
	}

	return m.ResetVp()
}

func (m Model) GetCmdHist() []string {
	return m.cmdHist
}
