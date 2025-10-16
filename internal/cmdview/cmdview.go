package cmdview

import (
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Model struct {
	vp           viewport.Model
	SelectStyle  lipgloss.Style
	cmdHist      []string
	cmdHistIndex int
}

// New creates a new model with default settings.
func New() (m Model) {
	m.vp = viewport.New(30, 5)
	m.SelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.cmdHistIndex = len(m.cmdHist)
	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {

		case tea.KeyCtrlD:
			m.deleteCmd()
			return m, nil

		case tea.KeyUp:
			m.scrollUp()
			return m, nil

		case tea.KeyDown:
			m.scrollDown()
			return m, nil

		default:
			return m, nil
		}

	default:
		return m, nil
	}
}

// View renders the model's view.
func (m Model) View() string {
	return m.vp.View()
}

// Scroll up cmd view.
func (m Model) scrollUp() {
	if m.cmdHistIndex > 0 {
		m.cmdHistIndex--
	}
	if m.cmdHistIndex < m.vp.YOffset {
		m.vp.ScrollUp(1)
	}
	m.updateCmdHistView()
}

// Scroll down cmd view.
func (m Model) scrollDown() {
	if m.cmdHistIndex < len(m.cmdHist) {
		m.cmdHistIndex++
		if m.cmdHistIndex < len(m.cmdHist) {
			// The bottom-most visible line is at YOffset + Height - 1.
			bottomEdge := m.vp.YOffset + m.vp.Height - 1
			// If the selection is now below the visible area of the viewport,
			// scroll the viewport down to keep it in view.
			if m.cmdHistIndex > bottomEdge {
				m.vp.ScrollDown(1)
			}
		}
		m.updateCmdHistView()
	}
}

func (m Model) updateCmdHistView() {
	cmdHistLines := make([]string, len(m.cmdHist))
	for i, cmd := range m.cmdHist {
		if i == m.cmdHistIndex {
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), m.SelectStyle.Render("> "+cmd))
		} else {
			cmdHistLines[i] = zone.Mark(strconv.Itoa(i), cmd)
		}
	}
	m.vp.SetContent(lipgloss.NewStyle().Render(strings.Join(cmdHistLines, "\n")))
	// m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
	// m.inputTa.SetCursor(len(m.inputTa.Value()))
}

// Delete cmd from command history and reset cmd hist index.
func (m Model) deleteCmd() {
	if m.cmdHistIndex != len(m.cmdHist) {
		m.cmdHist = append(m.cmdHist[:m.cmdHistIndex], m.cmdHist[m.cmdHistIndex+1:]...)
		m.resetVp()
	}
}

func (m Model) resetVp() {
	log.Printf("reset vp: vp height, msg len: %v, %v\n", m.vp.Height, len(m.cmdHist))

	if m.vp.Height > 0 && len(m.cmdHist) > 0 {
		m.cmdHistIndex = len(m.cmdHist)
		m.updateCmdHistView()
		m.vp.GotoBottom()
	}
}
