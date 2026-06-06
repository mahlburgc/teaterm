package cmdhist

import (
	"fmt"
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
	// cmdHistMatchIdx[i] holds byte offsets of fuzzy-matched runes inside
	// cmdHistFiltered[i]. nil/empty when no filter is active.
	cmdHistMatchIdx [][]int
}

// New creates a new model with default settings.
// Command history can be passed to start with existing commands.
func New(cmdHist []string) (m Model) {
	m.Vp = viewport.New(30, 5)
	m.SelectStyle = styles.SelectedCmdStyle
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
	m.cmdHistMatchIdx = make([][]int, len(m.cmdHist))
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
			m.deleteCmd() // TODO fix command deletion
			return m, nil

		case key.Matches(msg, keymap.Default.HistUpKey):
			m.scrollUp()
			return m, nil

		case key.Matches(msg, keymap.Default.HistDownKey):
			m.scrollDown()
			return m, nil

		case key.Matches(msg, keymap.Default.ToggleHistKey):
			// The filter itself is preserved here: the input broadcasts its
			// current value as PartialTxMsg on this key, so the popup opens
			// already filtered by whatever is in the input.
			m.ResetVp(false)
			return m, nil

		case key.Matches(msg, keymap.Default.ResetKey):
			m.ResetVp(true)
			return m, nil

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
			m.updateCmdHistView()
			return m, nil
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
		m.cmdHistMatchIdx = make([][]int, len(m.cmdHist))
	} else {
		matches := fuzzy.FindNoSort(msg, m.cmdHist)
		m.cmdHistFiltered = make([]string, len(matches))
		m.cmdHistMatchIdx = make([][]int, len(matches))
		for i, match := range matches {
			m.cmdHistFiltered[i] = match.Str
			m.cmdHistMatchIdx[i] = match.MatchedIndexes
		}
	}

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

// Returns a Tea command that mirrors the given cmd into the input
// (events.HistCmdSelected). Used when the selection is confirmed with Enter.
func SendCmdSelectedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return events.HistCmdSelected(cmd)
	}
}

// Returns a Tea command that directly transmits the given cmd
// (events.SendMsg). Used when a command is clicked with the mouse.
func SendCmdExecutedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return events.SendMsg{Data: cmd, FromCmdHist: true}
	}
}

// View renders the model's view.
func (m Model) View() string {
	nrFilteredCmds := len(m.cmdHistFiltered)
	nrCmds := len(m.cmdHist)
	footer := fmt.Sprintf("%v/%v", nrFilteredCmds, nrCmds)

	return styles.AddBorder(m.Vp, "Commands", footer, false)
}

func (m *Model) SetSize(width, height int) {
	borderWidth, borderHeight := styles.BorderStyle.GetFrameSize()

	m.Vp.Width = width - borderWidth
	m.Vp.Height = height - borderHeight

	m.ResetVp(false)
}

// TODO (cma): if hight is too low, selected command is not shown
// scrollUp moves selection up and handles scroll padding at the top.
func (m *Model) scrollUp() {
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
	m.updateCmdHistView()
}

// scrollDown moves selection down and handles scroll padding at the bottom.
func (m *Model) scrollDown() {
	if m.cmdHistIndex < len(m.cmdHistFiltered)-1 {
		m.cmdHistIndex++

		bottomEdge := m.Vp.YOffset + m.Vp.Height - 1

		// If index enters the bottom padding zone, scroll viewport down.
		if m.cmdHistIndex > bottomEdge-scrollPadding && m.Vp.YOffset+m.Vp.Height < len(m.cmdHistFiltered) {
			m.Vp.ScrollDown(1)
		} else if m.cmdHistIndex > bottomEdge {
			// Safety fallback: if we are strictly below the view, snap to it.
			m.Vp.SetYOffset(m.cmdHistIndex - m.Vp.Height + 1)
		}
	}
	m.updateCmdHistView()
}

func (m *Model) updateCmdHistView() {
	cmdHistLines := make([]string, len(m.cmdHistFiltered))
	for i, cmd := range m.cmdHistFiltered {
		var idx []int
		if i < len(m.cmdHistMatchIdx) {
			idx = m.cmdHistMatchIdx[i]
		}
		if i == m.cmdHistIndex {
			const prefix = "> "
			shifted := make([]int, len(idx))
			for k, ix := range idx {
				shifted[k] = ix + len(prefix)
			}
			// Fuzzy matches keep their highlight but get the selection
			// background so they don't punch holes into the selection bar.
			hlStyle := styles.SearchHighlightStyle.Background(styles.AdaptiveSelectedBg)
			line := highlightMatches(prefix+cmd, shifted, m.SelectStyle, hlStyle)
			// Extend the selection background over the whole line up to the
			// border. Pad before zone.Mark so the markers don't distort the
			// width calculation.
			if pad := m.Vp.Width - lipgloss.Width(line); pad > 0 {
				line += m.SelectStyle.Render(strings.Repeat(" ", pad))
			}
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), line)
		} else {
			line := highlightMatches(cmd, idx, lipgloss.NewStyle(), styles.SearchHighlightStyle)
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), line)
		}
	}
	m.Vp.SetContent(lipgloss.NewStyle().Render(strings.Join(cmdHistLines, "\n")))
}

func highlightMatches(s string, matchIdx []int, base, hl lipgloss.Style) string {
	if len(matchIdx) == 0 {
		return base.Render(s)
	}
	matchSet := make(map[int]bool, len(matchIdx))
	for _, i := range matchIdx {
		matchSet[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		if matchSet[i] {
			b.WriteString(hl.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

// Delete cmd from command history and reset cmd hist index.
func (m *Model) deleteCmd() {
	// Check if index is valid within the currently visible (filtered) list
	if m.cmdHistIndex >= 0 && m.cmdHistIndex < len(m.cmdHistFiltered) {
		// 1. Identify the command to delete
		cmdToDelete := m.cmdHistFiltered[m.cmdHistIndex]

		// 2. Remove from Filtered List
		m.cmdHistFiltered = append(m.cmdHistFiltered[:m.cmdHistIndex], m.cmdHistFiltered[m.cmdHistIndex+1:]...)
		if m.cmdHistIndex < len(m.cmdHistMatchIdx) {
			m.cmdHistMatchIdx = append(m.cmdHistMatchIdx[:m.cmdHistIndex], m.cmdHistMatchIdx[m.cmdHistIndex+1:]...)
		}

		// 3. Remove from Main History List
		// We iterate to find the matching string in the main list
		for i, cmd := range m.cmdHist {
			if cmd == cmdToDelete {
				m.cmdHist = append(m.cmdHist[:i], m.cmdHist[i+1:]...)
				break // Stop after finding the match
			}
		}

		// 4. Adjust Index
		// If we deleted the last item, move index up by one
		if m.cmdHistIndex >= len(m.cmdHistFiltered) && m.cmdHistIndex > 0 {
			m.cmdHistIndex--
		}

		log.Printf("Command deleted: %s", cmdToDelete)

		// 5. Update the view to reflect the deletion without losing the current search context
		m.updateCmdHistView()
	}
}

func (m *Model) ResetVp(resetFilter bool) {
	log.Printf("reset cmd vp: vp height, msg len: %v, %v\n", m.Vp.Height, len(m.cmdHist))

	if m.Vp.Height > 0 {
		if resetFilter {
			m.cmdHistFiltered = m.cmdHist
			m.cmdHistMatchIdx = make([][]int, len(m.cmdHist))
		}

		if len(m.cmdHistFiltered) > 0 {
			m.cmdHistIndex = len(m.cmdHistFiltered) - 1
		} else {
			m.cmdHistIndex = 0
		}
		m.updateCmdHistView()
		m.Vp.GotoBottom()
	}
}

// Add a new command to the command history. The command will only be added, if not
// already exisiting in the hist. If cmd is found, it will be moved to the end.
func (m *Model) AddCmd(newCmd string) {
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

	m.ResetVp(true)
}

func (m Model) GetCmdHist() []string {
	return m.cmdHist
}

// GetSelectedCmd returns the currently highlighted command,
// or "" if nothing is selected.
func (m Model) GetSelectedCmd() string {
	if m.cmdHistIndex >= 0 && m.cmdHistIndex < len(m.cmdHistFiltered) {
		return m.cmdHistFiltered[m.cmdHistIndex]
	}
	return ""
}
