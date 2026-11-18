package internal

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/msglog"
	"github.com/mahlburgc/teaterm/internal/styles"
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

// TestMinimalBordersLayout verifies the minimal border layout: no side
// borders, nothing wider than the terminal and the component heights add up
// to the terminal height.
func TestMinimalBordersLayout(t *testing.T) {
	for _, minimal := range []bool{false, true} {
		t.Run(fmt.Sprintf("minimal=%v", minimal), func(t *testing.T) {
			styles.SetMinimalBorders(minimal)
			t.Cleanup(func() { styles.SetMinimalBorders(false) })

			zone.NewGlobal()
			var port io.ReadWriteCloser
			port, mode := OpenFakePort()
			defer port.Close()
			m := initialModel(&port, false, []string{"alpha", "bravo"}, "mock", &mode, nil, false, panelSettings())

			const width, height = 80, 30
			m = processMsg(m, tea.WindowSizeMsg{Width: width, Height: height}, nil, 0)

			view := ansi.Strip(m.View())
			lines := strings.Split(view, "\n")
			if len(lines) != height {
				t.Errorf("view has %d lines, want %d", len(lines), height)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("line %d is %d wide, want <= %d: %q", i, w, width, line)
				}
			}
			if hasSide := strings.Contains(view, "│"); hasSide == minimal {
				t.Errorf("side borders present = %v, want %v", hasSide, !minimal)
			}
			for _, title := range []string{"Messages", "Commands"} {
				if !strings.Contains(view, title) {
					t.Errorf("title %q missing", title)
				}
			}
		})
	}
}

// TestMouseSelectionCoordinates verifies the mapping from screen coordinates
// to message log text in both border styles: dragging over the first log
// line ("Message log start ...") copies exactly the dragged characters.
func TestMouseSelectionCoordinates(t *testing.T) {
	for _, minimal := range []bool{false, true} {
		t.Run(fmt.Sprintf("minimal=%v", minimal), func(t *testing.T) {
			styles.SetMinimalBorders(minimal)
			t.Cleanup(func() { styles.SetMinimalBorders(false) })

			var copied []string
			restore := msglog.SetCopyFunc(func(text string) tea.Cmd {
				copied = append(copied, text)
				return nil
			})
			t.Cleanup(restore)

			zone.NewGlobal()
			var port io.ReadWriteCloser
			port, mode := OpenFakePort()
			defer port.Close()
			m := initialModel(&port, false, nil, "mock", &mode, nil, false, DefaultSettings())
			m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)

			// Render once so bubblezone knows where the message log is.
			m.View()
			deadline := time.Now().Add(time.Second)
			for zone.Get("msglog").IsZero() {
				if time.Now().After(deadline) {
					t.Fatal("msglog zone not registered")
				}
				time.Sleep(5 * time.Millisecond)
			}

			// First content row is screen row 1, first column is 1 with
			// full borders (left border) and 0 with minimal borders.
			x0 := 1
			if minimal {
				x0 = 0
			}
			press := tea.MouseMsg{X: x0, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
			move := tea.MouseMsg{X: x0 + 6, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}
			release := tea.MouseMsg{X: x0 + 6, Y: 1, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease}
			for _, msg := range []tea.Msg{press, move, release} {
				m = processMsg(m, msg, nil, 0)
			}

			if len(copied) != 1 || copied[0] != "Message" {
				t.Errorf("copied %q, want [Message]", copied)
			}
		})
	}
}

// TestFilterStaysActive verifies that a message log filter can be kept while
// sending commands: Enter / ctrl+f leave filter mode and keep the filter,
// Esc in filter mode clears it, Esc in input mode does not touch it.
func TestFilterStaysActive(t *testing.T) {
	zone.NewGlobal()
	var port io.ReadWriteCloser
	port, mode := OpenFakePort()
	defer port.Close()
	m := initialModel(&port, false, nil, "mock", &mode, nil, false, DefaultSettings())
	m = processMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}, nil, 0)

	rx := func(line string) {
		m.msglog, _ = m.msglog.Update(events.SerialRxMsgReceived(line))
	}
	key := func(k tea.KeyMsg) { m = processMsg(m, k, nil, 0) }
	typeText := func(s string) { key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}) }
	logView := func() string { return ansi.Strip(m.msglog.View()) }
	inputView := func() string { return ansi.Strip(m.input.View()) }

	rx("temp=21")
	rx("hum=40")

	// Filter and keep it with Enter.
	key(tea.KeyMsg{Type: tea.KeyCtrlF})
	typeText("temp")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(logView(), "filter: temp") {
		t.Fatalf("filter not shown in title after enter:\n%s", logView())
	}
	if strings.Contains(logView(), "hum=40") {
		t.Error("filter not applied: hum=40 visible")
	}
	if !strings.Contains(inputView(), "> ") || strings.Contains(inputView(), "Filter:") {
		t.Errorf("not back in input mode: %q", inputView())
	}

	// Send while filtered: sent lines are always shown, the filter stays.
	typeText("xyz")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(logView(), "xyz") {
		t.Error("sent line hidden by filter")
	}
	if !strings.Contains(logView(), "filter: temp") {
		t.Error("filter lost after sending")
	}

	// New rx lines are still filtered.
	rx("hum=41")
	rx("temp=22")
	if strings.Contains(logView(), "hum=41") || !strings.Contains(logView(), "temp=22") {
		t.Errorf("rx lines not filtered:\n%s", logView())
	}

	// Esc in input mode only resets the input (ctrl+c would clear the filter).
	typeText("abc")
	key(tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(logView(), "filter: temp") {
		t.Error("esc in input mode cleared the filter")
	}

	// Connection changes keep the filter.
	for _, s := range []events.ConnectionStatus{events.Disconnected, events.Connecting, events.Connected} {
		m = processMsg(m, events.ConnectionStatusMsg{Status: s}, nil, 0)
	}
	if !strings.Contains(logView(), "filter: temp") {
		t.Error("reconnect cleared the filter")
	}

	// ctrl+f pre-fills the active filter, ctrl+f again keeps it.
	key(tea.KeyMsg{Type: tea.KeyCtrlF})
	if !strings.Contains(inputView(), "Filter: temp") {
		t.Errorf("filter not pre-filled: %q", inputView())
	}
	key(tea.KeyMsg{Type: tea.KeyCtrlF})
	if !strings.Contains(logView(), "filter: temp") {
		t.Error("ctrl+f in filter mode cleared the filter")
	}

	// Esc in filter mode clears the filter.
	key(tea.KeyMsg{Type: tea.KeyCtrlF})
	key(tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(logView(), "filter:") || !strings.Contains(logView(), "hum=41") {
		t.Errorf("filter not cleared by esc:\n%s", logView())
	}

	// ctrl+c in input mode clears input and filter.
	key(tea.KeyMsg{Type: tea.KeyCtrlF})
	typeText("temp")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	typeText("abc")
	key(tea.KeyMsg{Type: tea.KeyCtrlC})
	if strings.Contains(logView(), "filter:") {
		t.Error("ctrl+c in input mode did not clear the filter")
	}
	if strings.Contains(inputView(), "abc") {
		t.Error("ctrl+c did not clear the input")
	}
}
