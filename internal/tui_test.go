package internal

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
)

// processCmd executes a tea.Cmd and feeds resulting messages back into the
// model, simulating the bubbletea event loop. Emitted HistCmdSelected
// messages are recorded in selectedLog before being dispatched.
func processCmd(m model, cmd tea.Cmd, selectedLog *[]string, depth int) model {
	if cmd == nil || depth > 8 {
		return m
	}
	return processMsg(m, cmd(), selectedLog, depth)
}

func processMsg(m model, msg tea.Msg, selectedLog *[]string, depth int) model {
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = processCmd(m, c, selectedLog, depth+1)
		}
		return m
	}
	// Don't dispatch cursor blink ticks: each one makes the model emit the
	// next BlinkCmd, which sleeps for the blink interval when executed.
	if _, ok := msg.(cursor.BlinkMsg); ok {
		return m
	}
	if selected, ok := msg.(events.HistCmdSelected); ok && selectedLog != nil {
		*selectedLog = append(*selectedLog, string(selected))
	}
	nm, cmd := m.Update(msg)
	m = nm.(model)
	return processCmd(m, cmd, selectedLog, depth+1)
}

// TestCmdHistEnterSelectsHighlighted verifies that pressing Enter while the
// command history popup is open selects the highlighted command, not the
// last one. Regression test: the Enter intercept used to read the selection
// after updateLayout, whose SetSize -> ResetVp had already reset it to the
// last item.
func TestCmdHistEnterSelectsHighlighted(t *testing.T) {
	zone.NewGlobal()
	var port io.ReadWriteCloser
	port, mode := OpenFakePort()
	defer port.Close()
	m := initialModel(&port, false, []string{"alpha", "bravo", "charlie"}, "mock", &mode, nil, false, DefaultSettings())

	m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)

	// Open the popup: the last command must be auto-selected.
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)
	if got := m.cmdhist.GetSelectedCmd(); got != "charlie" {
		t.Fatalf("after open: selected = %q, want %q", got, "charlie")
	}

	// Scroll up twice: selection must follow, but it must NOT be mirrored
	// into the input (no HistCmdSelected emissions while scrolling).
	var scrollLog []string
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp}, &scrollLog, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp}, &scrollLog, 0)
	if got := m.cmdhist.GetSelectedCmd(); got != "alpha" {
		t.Fatalf("after scrolling up twice: selected = %q, want %q", got, "alpha")
	}
	if len(scrollLog) > 0 {
		t.Errorf("scrolling mirrored selection into input: HistCmdSelected emissions = %q, want none", scrollLog)
	}

	// Enter must close the popup and emit the highlighted command.
	var selectedLog []string
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyEnter}, &selectedLog, 0)
	if m.showCmdLog {
		t.Error("popup still open after enter")
	}
	if len(selectedLog) == 0 || selectedLog[len(selectedLog)-1] != "alpha" {
		t.Errorf("HistCmdSelected emissions on enter = %q, want last to be %q", selectedLog, "alpha")
	}

	// Reopen the popup: the input now contains "alpha", so the history must
	// open filtered by it ("alpha" is the only fuzzy match and is selected).
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)
	if !m.showCmdLog {
		t.Fatal("popup not open after second ctrl+r")
	}
	if got := m.cmdhist.GetSelectedCmd(); got != "alpha" {
		t.Errorf("after reopen with %q in input: selected = %q, want %q", "alpha", got, "alpha")
	}
}

// TestAutoCompleteUpdatesHistFilter verifies that accepting an input
// suggestion (tab) re-filters the command history with the completed value.
// Regression test: SetValue bypasses ta.Update, so no PartialTxMsg was
// broadcast and the filter stayed on the typed prefix.
func TestAutoCompleteUpdatesHistFilter(t *testing.T) {
	zone.NewGlobal()
	var port io.ReadWriteCloser
	port, mode := OpenFakePort()
	defer port.Close()
	// "a" fuzzy-matches both; the completed "ab" only matches itself.
	m := initialModel(&port, false, []string{"axc", "ab"}, "mock", &mode, nil, false, DefaultSettings())

	m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)

	// Type "a": both commands stay listed, suggestion becomes "ab".
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, nil, 0)
	if view := m.cmdhist.View(); !strings.Contains(view, "axc") {
		t.Fatalf("after typing %q: %q not listed in cmd hist view", "a", "axc")
	}

	// Accept the suggestion: the filter must update to "ab".
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyTab}, nil, 0)
	view := m.cmdhist.View()
	if strings.Contains(view, "axc") {
		t.Errorf("cmd hist filter not updated after accepting suggestion: %q still listed", "axc")
	}
	if got := m.cmdhist.GetSelectedCmd(); got != "ab" {
		t.Errorf("after accepting suggestion: selected = %q, want %q", got, "ab")
	}
}

// TestClosedPopupRecall verifies shell-style command recall while the
// command history popup is closed: Up/Down navigates the history and mirrors
// the command directly into the input (events.HistCmdSelected), starting from
// the most recent command. Down past the newest recalls an empty input.
func TestClosedPopupRecall(t *testing.T) {
	zone.NewGlobal()
	var port io.ReadWriteCloser
	port, mode := OpenFakePort()
	defer port.Close()
	m := initialModel(&port, false, []string{"alpha", "bravo", "charlie"}, "mock", &mode, nil, false, DefaultSettings())

	m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)
	if m.showCmdLog {
		t.Fatal("popup should start closed")
	}

	var log []string
	// First Up recalls the most recent command, each further Up an older one.
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp}, &log, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp}, &log, 0)
	// Down steps back towards newer commands, then onto the empty prompt.
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyDown}, &log, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyDown}, &log, 0)

	if m.showCmdLog {
		t.Error("popup must stay closed during recall")
	}

	want := []string{"charlie", "bravo", "charlie", ""}
	if strings.Join(log, "|") != strings.Join(want, "|") {
		t.Errorf("recall mirrored %q into input, want %q", log, want)
	}
}

func panelSettings() Settings {
	s := DefaultSettings()
	s.CmdHist.Mode = HistModePanel
	return s
}

// TestPanelModeEnterSends verifies the persistent panel mode: the history
// stays open, Enter sends the input instead of selecting a command, and
// Esc does not hide the panel.
func TestPanelModeEnterSends(t *testing.T) {
	zone.NewGlobal()
	var port io.ReadWriteCloser
	port, mode := OpenFakePort()
	defer port.Close()
	m := initialModel(&port, false, []string{"alpha", "bravo"}, "mock", &mode, nil, false, panelSettings())

	m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)
	if !m.showCmdLog {
		t.Fatal("panel not shown on start")
	}
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)
	if m.showCmdLog {
		t.Fatal("panel not hidden after ctrl+r")
	}
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)
	if !m.showCmdLog {
		t.Fatal("panel not shown after second ctrl+r")
	}

	// Up recalls shell-style into the input, like a closed popup.
	var recalled []string
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp}, &recalled, 0)
	if len(recalled) != 1 || recalled[0] != "bravo" {
		t.Fatalf("up recalled %q, want [bravo]", recalled)
	}

	m = processMsg(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("new")}, nil, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyEnter}, nil, 0)
	if !m.showCmdLog {
		t.Error("panel closed after enter")
	}
	hist := m.cmdhist.GetCmdHist()
	if last := hist[len(hist)-1]; last != "bravonew" {
		t.Errorf("last sent command = %q, want %q", last, "bravonew")
	}

	m = processMsg(m, tea.KeyMsg{Type: tea.KeyEsc}, nil, 0)
	if !m.showCmdLog {
		t.Error("esc must not hide the panel")
	}

	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)
	if m.showCmdLog {
		t.Error("ctrl+r must hide the panel")
	}
}

// TestCmdHistResize verifies the configured height and alt+up/down resizing,
// clamped so the message log keeps its minimum height.
func TestCmdHistResize(t *testing.T) {
	zone.NewGlobal()
	var port io.ReadWriteCloser
	port, mode := OpenFakePort()
	defer port.Close()
	s := DefaultSettings()
	s.CmdHist.Height = Size{Value: 50, Percent: true}
	m := initialModel(&port, false, nil, "mock", &mode, nil, false, s)

	m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyCtrlR}, nil, 0)
	if got := m.cmdLogHeight(); got != 15 {
		t.Fatalf("50%% of 30 rows: got %d, want 15", got)
	}

	m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp, Alt: true}, nil, 0)
	if got := m.cmdLogHeight(); got != 16 {
		t.Errorf("after alt+up: got %d, want 16", got)
	}
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyDown, Alt: true}, nil, 0)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyDown, Alt: true}, nil, 0)
	if got := m.cmdLogHeight(); got != 14 {
		t.Errorf("after 2x alt+down: got %d, want 14", got)
	}

	for range 50 {
		m = processMsg(m, tea.KeyMsg{Type: tea.KeyDown, Alt: true}, nil, 0)
	}
	if got := m.cmdLogHeight(); got != minCmdLogHeight {
		t.Errorf("shrunk to min: got %d, want %d", got, minCmdLogHeight)
	}

	for range 50 {
		m = processMsg(m, tea.KeyMsg{Type: tea.KeyUp, Alt: true}, nil, 0)
	}
	maxH := 30 - m.input.GetHeight() - m.footer.GetHeight() - minMsgLogHeight
	if got := m.cmdLogHeight(); got != maxH {
		t.Errorf("grown to max: got %d, want %d", got, maxH)
	}
	// one alt+down from max must shrink immediately (no hidden overshoot)
	m = processMsg(m, tea.KeyMsg{Type: tea.KeyDown, Alt: true}, nil, 0)
	if got := m.cmdLogHeight(); got != maxH-1 {
		t.Errorf("alt+down from max: got %d, want %d", got, maxH-1)
	}
}
