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
	m := initialModel(&port, false, []string{"alpha", "bravo", "charlie"}, "mock", &mode, nil, false)

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
	m := initialModel(&port, false, []string{"axc", "ab"}, "mock", &mode, nil, false)

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
