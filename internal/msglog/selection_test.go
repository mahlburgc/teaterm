package msglog

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mahlburgc/teaterm/events"
	"github.com/muesli/termenv"
)

// newTestLog returns a message log with a 40x5 viewport and the given rx
// lines (after the start message at index 0).
func newTestLog(t *testing.T, limit int, lines ...string) Model {
	t.Helper()
	m := New(false, false, lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), nil, limit)
	m.SetSize(40+2, 5+2) // full border frame
	for _, l := range lines {
		m, _ = m.Update(events.SerialRxMsgReceived(l))
	}
	return m
}

// captureClipboard replaces copyToClipboard and returns the copied texts.
func captureClipboard(t *testing.T) *[]string {
	t.Helper()
	var copied []string
	orig := copyToClipboard
	copyToClipboard = func(text string) tea.Cmd {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { copyToClipboard = orig })
	return &copied
}

// visibleRow returns the logFiltered index shown in viewport row 0.
func firstVisible(m *Model) int {
	return m.getFirstViewableElementIndex()
}

func TestSelectedText(t *testing.T) {
	m := newTestLog(t, 100, "hello world", "\x1b[31mred\x1b[0m line", "third line")
	// rows: 0 start msg, 1 hello world, 2 red line, 3 third line

	tests := []struct {
		name             string
		fromRow, fromCol int
		toRow, toCol     int
		want             string
	}{
		{"single line", 1, 6, 1, 10, "world"},
		{"multi line, colors stripped", 1, 6, 3, 4, "world\nred line\nthird"},
		{"backwards", 3, 4, 1, 6, "world\nred line\nthird"},
		{"beyond line end", 1, 6, 1, 99, "world"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m.selectStart(tc.fromRow, tc.fromCol)
			m.selectMove(tc.toRow, tc.toCol)
			if got := m.selectedText(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			m.clearSelection()
		})
	}
}

func TestSelectionCopyAndHighlight(t *testing.T) {
	copied := captureClipboard(t)
	m := newTestLog(t, 100, "hello world")

	// Click without drag: nothing copied.
	m.selectStart(1, 2)
	m.selectEnd()
	if len(*copied) != 0 {
		t.Fatalf("click copied %q", *copied)
	}

	// Drag: highlight visible while dragging, copied on release.
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	m.selectStart(1, 0)
	m.selectMove(1, 4)
	m.UpdateVp()
	if !strings.Contains(m.Vp.View(), "\x1b[") {
		t.Error("no highlight in viewport while dragging")
	}
	if got := ansi.Strip(m.Vp.View()); !strings.Contains(got, "hello world") {
		t.Errorf("highlight changed the text: %q", got)
	}
	if cmd := m.selectEnd(); cmd == nil {
		t.Error("no command on release")
	}
	if len(*copied) != 1 || (*copied)[0] != "hello" {
		t.Errorf("copied %q, want [hello]", *copied)
	}
	if m.sel.active {
		t.Error("selection not cleared on release")
	}
	if m.notice != "copied 5 chars" {
		t.Errorf("notice = %q", m.notice)
	}
	m, _ = m.Update(clearNoticeMsg{id: m.noticeID})
	if m.notice != "" {
		t.Error("notice not cleared")
	}
}

// New messages while dragging must not move the visible text, and the
// selection must keep its text. On release the log jumps back to the bottom.
func TestSelectionFreezesView(t *testing.T) {
	copied := captureClipboard(t)
	var lines []string
	for i := range 10 {
		lines = append(lines, fmt.Sprintf("line %02d", i))
	}
	m := newTestLog(t, 100, lines...)
	if !m.atBottom() {
		t.Fatal("log should be at bottom")
	}

	first := firstVisible(&m)
	m.selectStart(0, 0)
	m.selectMove(1, 6)
	want := m.selectedText()

	for i := range 5 {
		m, _ = m.Update(events.SerialRxMsgReceived(fmt.Sprintf("new %d", i)))
	}
	m, _ = m.Update(events.InfoMsg("info during drag"))

	if got := firstVisible(&m); got != first {
		t.Errorf("view moved during drag: first visible %d, want %d", got, first)
	}
	if got := m.selectedText(); got != want {
		t.Errorf("selected text changed: %q, want %q", got, want)
	}

	m.selectEnd()
	if !m.atBottom() {
		t.Error("log not back at bottom after release")
	}
	if len(*copied) != 1 || (*copied)[0] != want {
		t.Errorf("copied %q, want [%q]", *copied, want)
	}
}

// Dropping the oldest lines at the log limit must keep the selection on
// its text.
func TestSelectionShiftsOnLogLimit(t *testing.T) {
	m := newTestLog(t, 8, "a1", "a2", "a3", "a4", "a5", "a6", "a7")
	// logFiltered: start, a1..a7 (8 lines, limit reached)
	m.scrollToTop()
	m.UpdateVp()
	m.selectStart(2, 0) // a2
	m.selectMove(3, 1)  // a3
	if got := m.selectedText(); got != "a2\na3" {
		t.Fatalf("before: %q", got)
	}

	// a1 is dropped, a2/a3 move up by one line.
	m, _ = m.Update(events.SerialRxMsgReceived("a8"))

	if got := m.selectedText(); got != "a2\na3" {
		t.Errorf("after trimming: %q, want %q", got, "a2\na3")
	}
}

func TestHorizontalOffsetSelection(t *testing.T) {
	m := newTestLog(t, 100, strings.Repeat("x", 80)+"TARGET")
	m.scrollHorizontal(30)
	m.UpdateVp()
	if m.xOffset != 30 {
		t.Fatalf("xOffset = %d, want 30", m.xOffset)
	}
	m.selectStart(1, 50)
	m.selectMove(1, 55)
	if got := m.selectedText(); got != "TARGET" {
		t.Errorf("got %q, want TARGET", got)
	}

	// offset is clamped to the visible content
	m.scrollHorizontal(1000)
	m.UpdateVp()
	if want := 86 - 40; m.xOffset != want {
		t.Errorf("clamped xOffset = %d, want %d", m.xOffset, want)
	}
}

// While filtering, sent and info lines are always shown in their normal
// colors, received lines are filtered and highlighted.
func TestFilterKeepsSentLines(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	sendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
	m := New(false, false, sendStyle, lipgloss.NewStyle(), lipgloss.NewStyle(), nil, 100)
	m.SetSize(40+2, 6+2)
	m, _ = m.Update(events.SerialRxMsgReceived("temp=21"))
	m, _ = m.Update(events.SerialRxMsgReceived("hum=40"))
	m, _ = m.Update(events.SendMsg{Data: "get hum"})

	// existing log is filtered, new lines are filtered on arrival
	m, _ = m.Update(events.MsgLogFilterStringMsg("temp"))
	m, _ = m.Update(events.SendMsg{Data: "get voltage"})
	m, _ = m.Update(events.InfoMsg("port reconnected"))
	m, _ = m.Update(events.SerialRxMsgReceived("hum=41"))

	view := m.Vp.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"temp=21", "get hum", "get voltage", "INFO: port reconnected"} {
		if !strings.Contains(plain, want) {
			t.Errorf("%q missing in filtered view:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"hum=40", "hum=41"} {
		if strings.Contains(plain, hidden) {
			t.Errorf("%q shown in filtered view", hidden)
		}
	}
	if !strings.Contains(view, sendStyle.Render("get voltage")) {
		t.Error("sent line lost its color while filtering")
	}
}
