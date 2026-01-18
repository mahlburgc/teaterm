package cmdhist

import (
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
	"github.com/sahilm/fuzzy"
)

const scrollPadding = 3

type Model struct {
	Vp              viewport.Model
	SelectStyle     lipgloss.Style
	cmdHist         []string
	cmdHistIndex    int
	active          bool
	cmdHistFiltered []string
}

// New creates a new model with default settings.
// Command history can be passed to start with existing commands.
func New(cmdHist []string) (m Model) {
	m.Vp = viewport.New(30, 5)
	m.SelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.cmdHistIndex = len(m.cmdHist)

	// Disable the viewport's default up/down key handling.
	m.Vp.KeyMap.Up.SetEnabled(false)
	m.Vp.KeyMap.Down.SetEnabled(false)
	m.Vp.KeyMap.PageUp.SetEnabled(false)
	m.Vp.KeyMap.PageDown.SetEnabled(false)

	if cmdHist == nil {
		return m
	}

	for _, cmd := range cmdHist {
		if cmd != "" {
			m.cmdHist = append(m.cmdHist, cmd)
		}
	}
	m.cmdHistFiltered = m.cmdHist
	m.active = true

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case events.ConnectionStatusMsg:
		switch msg.Status {
		case events.Disconnected:
			m.ResetVp(true)
			m.active = false

		case events.Connected:
			m.active = true

		case events.Connecting:
			m.ResetVp(true)
			m.active = false
		}
		return m, cmd
	}

	// do not handle any other events during inactive state
	if m.active == false {
		return m, nil
	}

	// m.Vp, cmd = m.Vp.Update(msg) is currently not called because it breaks the manual vp handling
	switch msg := msg.(type) {

	case events.SendMsg:
		if !msg.FromCmdHist {
			m.AddCmd(msg.Data)
		}

	case events.PartialTxMsg:
		m.runFuzzySearch(string(msg))
		return m, m.findCmd(string(msg))

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.DeleteCmdKey):
			return m, m.deleteCmd() // TODO

		case key.Matches(msg, keymap.Default.HistUpKey):
			return m, m.scrollUp()

		case key.Matches(msg, keymap.Default.HistDownKey):
			return m, m.scrollDown()

		case key.Matches(msg, keymap.Default.ToggleHistKey, keymap.Default.ResetKey):
			return m, m.ResetVp(true)

		default:
			return m, nil
		}

	case tea.MouseMsg:
		// Every cmd in cmd history has number as zone name that can
		// directly used as index for the cmd history
		// Check on which cmd the mouse action is performed
		var cmdSelected bool
		for i := range m.cmdHistFiltered {
			if zone.Get(strconv.Itoa(i)).InBounds(msg) {
				m.cmdHistIndex = i
				cmdSelected = true
				break
			}
		}

		// mouse action not performed over cmd
		if !cmdSelected {
			return m, nil
		}

		if msg.Button == tea.MouseButtonRight ||
			msg.Action == tea.MouseActionPress ||
			msg.Action == tea.MouseActionMotion ||
			m.cmdHistIndex == len(m.cmdHistFiltered) {
			return m, m.updateCmdHistView()
		}

		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			cmd = SendCmdExecutedMsg(m.cmdHistFiltered[m.cmdHistIndex])
			m.cmdHistIndex = len(m.cmdHistFiltered)
			m.updateCmdHistView()
			return m, cmd
		}

		// x, y := zone.Get("confirm").Pos() can be used to get the relative
		// coordinates within the zone. Useful if you need to move a cursor in a
		// input box as an example.

	default:
		return m, nil
	}

	return m, nil
}

func (m *Model) runFuzzySearch(msg string) {
	if msg == "" {
		m.cmdHistFiltered = m.cmdHist
	} else {
		matches := fuzzy.Find(msg, m.cmdHist)
		m.cmdHistFiltered = make([]string, len(matches))
		for i, match := range matches {
			m.cmdHistFiltered[i] = match.Str
		}
	}
	// log.Printf("Filetered command list: %v", m.cmdHistFiltered)

	m.ResetVp(false)
}

func (m *Model) findCmd(msg string) tea.Cmd {
	// If input is empty, don't search
	if msg == "" {
		return nil
	}

	// Iterate backwards to find the most recent match
	for i := len(m.cmdHist) - 1; i >= 0; i-- {
		cmd := m.cmdHist[i]
		if strings.HasPrefix(cmd, msg) {
			// Found a match
			return func() tea.Msg {
				return events.InputSuggestion(cmd)
			}
		}
	}
	return nil
}

// Returns a Tea command to send a message with the mouse selected cmd to the event loop.
func SendCmdSelectedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return events.HistCmdSelected(cmd)
	}
}

// Returns a Tea command to send a message with the arrow selected cmd to the event loop.
func SendCmdExecutedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return events.SendMsg{Data: cmd, FromCmdHist: true}
	}
}

// View renders the model's view.
func (m Model) View() string {
	return styles.AddBorder(m.Vp, "Commands", "")
}

func (m *Model) SetSize(width, height int) {
	borderWidth, borderHeight := styles.FocusedBorderStyle.GetFrameSize()

	m.Vp.Width = width - borderWidth
	m.Vp.Height = height - borderHeight

	m.ResetVp(false)
}

// TODO (cma): if hight is too low, selected command is not shown
// scrollUp moves selection up and handles scroll padding at the top.
func (m *Model) scrollUp() tea.Cmd {
	if m.cmdHistIndex > 0 {
		m.cmdHistIndex--

		// If index enters the top padding zone, scroll the viewport up.
		if m.cmdHistIndex < m.Vp.YOffset+scrollPadding && m.Vp.YOffset > 0 {
			m.Vp.ScrollUp(1)
		} else if m.cmdHistIndex < m.Vp.YOffset {
			// Safety fallback: if we are strictly above the view, snap to it.
			m.Vp.SetYOffset(m.cmdHistIndex)
		}
	}
	return m.updateCmdHistView()
}

// scrollDown moves selection down and handles scroll padding at the bottom.
func (m *Model) scrollDown() (c tea.Cmd) {
	if m.cmdHistIndex < len(m.cmdHistFiltered) {
		m.cmdHistIndex++

		if m.cmdHistIndex < len(m.cmdHistFiltered) {
			bottomEdge := m.Vp.YOffset + m.Vp.Height - 1

			// If index enters the bottom padding zone, scroll viewport down.
			if m.cmdHistIndex > bottomEdge-scrollPadding && m.Vp.YOffset+m.Vp.Height < len(m.cmdHistFiltered) {
				m.Vp.ScrollDown(1)
			} else if m.cmdHistIndex > bottomEdge {
				// Safety fallback: if we are strictly below the view, snap to it.
				m.Vp.SetYOffset(m.cmdHistIndex - m.Vp.Height + 1)
			}
		} else {
			// Selection reached the empty end prompt
			m.Vp.GotoBottom()
		}
		c = m.updateCmdHistView()
	}
	return m.updateCmdHistView()
}

func (m *Model) updateCmdHistView() (c tea.Cmd) {
	cmdHistLines := make([]string, len(m.cmdHistFiltered))
	for i, cmd := range m.cmdHistFiltered {
		if i == m.cmdHistIndex {
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), m.SelectStyle.Render("> "+cmd))
			c = SendCmdSelectedMsg(m.cmdHistFiltered[m.cmdHistIndex])
		} else {
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), cmd)
		}
	}
	if c == nil {
		c = SendCmdSelectedMsg("")
	}
	m.Vp.SetContent(lipgloss.NewStyle().Render(strings.Join(cmdHistLines, "\n")))

	return c
}

// Delete cmd from command history and reset cmd hist index.
func (m *Model) deleteCmd() (c tea.Cmd) {
	if m.cmdHistIndex != len(m.cmdHist) {
		m.cmdHist = append(m.cmdHist[:m.cmdHistIndex], m.cmdHist[m.cmdHistIndex+1:]...)
		c = m.ResetVp(true)
		log.Printf("command will be deleted! New Command list: %v", m.cmdHist)
	}
	return c
}

func (m *Model) ResetVp(resetFilter bool) (c tea.Cmd) {
	log.Printf("reset cmd vp: vp height, msg len: %v, %v\n", m.Vp.Height, len(m.cmdHist))

	if m.Vp.Height > 0 {
		if resetFilter {
			m.cmdHistFiltered = m.cmdHist
		}
		m.cmdHistIndex = len(m.cmdHistFiltered)
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

	return m.ResetVp(true)
}

func (m Model) GetCmdHist() []string {
	return m.cmdHist
}
