package msglog

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/internal/styles"
)

const (
	msgLogZone     = "msglog"
	noticeDuration = 2 * time.Second
)

// selPos is a position inside the message log: line is an index into
// logFiltered, col is the display cell inside the full (unscrolled) line.
type selPos struct {
	line int
	col  int
}

// selection is a mouse text selection in the message log. Positions refer to
// log lines, not screen rows, so the selection sticks to its text while the
// log scrolls.
type selection struct {
	active      bool // mouse button pressed inside the message log
	dragged     bool // mouse moved while pressed
	anchor      selPos
	cursor      selPos
	wasAtBottom bool // log was at bottom when the selection started
}

// clearNoticeMsg removes the "copied" notice, if it is still the one with id.
type clearNoticeMsg struct {
	id int
}

// ordered returns the selection start and end (inclusive) in log order.
func (s selection) ordered() (start, end selPos) {
	a, c := s.anchor, s.cursor
	if c.line < a.line || (c.line == a.line && c.col < a.col) {
		return c, a
	}
	return a, c
}

// handleMouse maps a mouse event to a message log position and drives the
// selection: press starts it, motion extends it, release copies it.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	z := zone.Get(msgLogZone)
	if z == nil || z.IsZero() {
		return nil
	}

	frameWidth, _ := styles.FrameSize()
	row := msg.Y - z.StartY - 1 // one border/separator line on top
	col := msg.X - z.StartX - frameWidth/2

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return nil
		}
		if row < 0 || row >= m.Vp.Height || col < 0 || col >= m.Vp.Width {
			return nil
		}
		m.selectStart(row, col)

	case tea.MouseActionMotion:
		if !m.sel.active {
			return nil
		}
		// Dragging beyond the top/bottom edge scrolls the log.
		if row < 0 {
			m.scrollUp(1)
			row = 0
		} else if row >= m.Vp.Height {
			m.scrollDown(1)
			row = m.Vp.Height - 1
		}
		m.selectMove(row, col)

	case tea.MouseActionRelease:
		if !m.sel.active {
			return nil
		}
		return m.selectEnd()
	}

	return nil
}

// posAt converts a viewport row/column to a log position.
func (m *Model) posAt(row, col int) selPos {
	line := m.getFirstViewableElementIndex() + row
	line = max(0, min(line, len(m.logFiltered)-1))
	return selPos{line: line, col: max(0, col+m.xOffset)}
}

func (m *Model) selectStart(row, col int) {
	pos := m.posAt(row, col)
	m.sel = selection{
		active:      true,
		anchor:      pos,
		cursor:      pos,
		wasAtBottom: m.atBottom(),
	}
	m.needsUpdate = true
}

func (m *Model) selectMove(row, col int) {
	pos := m.posAt(row, col)
	if pos != m.sel.cursor {
		m.sel.cursor = pos
		m.sel.dragged = true
		m.needsUpdate = true
	}
}

// selectEnd finishes the selection, copies the selected text and clears the
// highlight. A click without dragging copies nothing.
func (m *Model) selectEnd() tea.Cmd {
	var cmd tea.Cmd
	if m.sel.dragged {
		if text := m.selectedText(); text != "" {
			cmd = tea.Batch(copyToClipboard(text), m.showNotice(
				fmt.Sprintf("copied %d chars", utf8.RuneCountInString(text))))
		}
	}

	wasAtBottom := m.sel.wasAtBottom
	m.clearSelection()
	if wasAtBottom {
		m.scrollToBottom()
	}
	return cmd
}

func (m *Model) clearSelection() {
	if m.sel.active {
		m.needsUpdate = true
	}
	m.sel = selection{}
}

// shiftSelection moves the selection after the oldest log line was dropped.
func (m *Model) shiftSelection(n int) {
	if !m.sel.active {
		return
	}
	m.sel.anchor.line = max(0, m.sel.anchor.line-n)
	m.sel.cursor.line = max(0, m.sel.cursor.line-n)
}

// selectedText returns the selected text without colors, lines joined by \n.
func (m *Model) selectedText() string {
	if !m.sel.active || len(m.logFiltered) == 0 {
		return ""
	}
	start, end := m.sel.ordered()

	var lines []string
	for i := start.line; i <= end.line && i < len(m.logFiltered); i++ {
		line := ansi.Strip(m.logFiltered[i])
		from, to := 0, ansi.StringWidth(line)
		if i == start.line {
			from = start.col
		}
		if i == end.line {
			to = min(to, end.col+1)
		}
		if from < to {
			lines = append(lines, ansi.Cut(line, from, to))
		} else {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

// highlightSelection adds the selection background to the visible lines.
// first is the logFiltered index of lines[0].
func (m *Model) highlightSelection(lines []string, first int) {
	if !m.sel.active || !m.sel.dragged {
		return
	}
	start, end := m.sel.ordered()

	for i := range lines {
		idx := first + i
		if idx < start.line || idx > end.line {
			continue
		}
		width := ansi.StringWidth(lines[i])
		from, to := 0, width
		if idx == start.line {
			from = start.col
		}
		if idx == end.line {
			to = min(to, end.col+1)
		}
		if from >= to {
			continue
		}
		l := lines[i]
		lines[i] = ansi.Cut(l, 0, from) +
			styles.SelectionStyle.Render(ansi.Strip(ansi.Cut(l, from, to))) +
			ansi.Cut(l, to, width)
	}
}

// showNotice shows a short notice in the message log footer for a while.
func (m *Model) showNotice(notice string) tea.Cmd {
	m.noticeID++
	m.notice = notice
	id := m.noticeID
	return tea.Tick(noticeDuration, func(time.Time) tea.Msg {
		return clearNoticeMsg{id: id}
	})
}

// scrollHorizontal scrolls the log n cells to the right (negative: left).
// The offset is clamped in UpdateVp.
func (m *Model) scrollHorizontal(n int) {
	m.xOffset = max(0, m.xOffset+n)
	m.needsUpdate = true
}
