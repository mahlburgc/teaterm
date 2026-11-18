package session

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mahlburgc/teaterm/events"
	"go.bug.st/serial"
)

type pipePort struct {
	*io.PipeReader
	w *io.PipeWriter
}

func (p pipePort) Write(b []byte) (int, error) { return len(b), nil }
func (p pipePort) Close() error                { p.w.Close(); return p.PipeReader.Close() }

func runCmd(t *testing.T, m Model) events.SerialRxMsgReceived {
	t.Helper()
	ch := make(chan any, 1)
	go func() { ch <- m.ReadFromPort(m.ctx)() }()
	select {
	case msg := <-ch:
		line, ok := msg.(events.SerialRxMsgReceived)
		if !ok {
			t.Fatalf("unexpected msg %T: %v", msg, msg)
		}
		return line
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for rx line")
	}
	return ""
}

// Simulates the app restart after closing the external editor: the read cmd of
// the old bubbletea program is still pending (orphan) when the new program
// starts its own read cmd. No line may be lost, duplicated or corrupted.
func TestReadFromPortSurvivesProgramRestart(t *testing.T) {
	r, w := io.Pipe()
	var port io.ReadWriteCloser = pipePort{r, w}
	m := New(&port, "mock", &serial.Mode{})
	defer port.Close()

	// Orphan cmd of the old program: its result goes nowhere.
	orphan := m.Init()
	orphanDone := make(chan struct{})
	go func() { orphan(); close(orphanDone) }()

	const n = 200
	go func() {
		for i := 0; i < n; i++ {
			fmt.Fprintf(w, "line %03d\n", i)
		}
	}()

	// Make sure the orphan has picked up (and "lost") a line.
	select {
	case <-orphanDone:
	case <-time.After(2 * time.Second):
		t.Fatal("orphan cmd did not return")
	}

	// New program: drive cmd -> Update loop.
	for i := 0; i < n; i++ {
		line := runCmd(t, m)
		if want := fmt.Sprintf("line %03d", i); string(line) != want {
			t.Fatalf("got %q, want %q", line, want)
		}
		m, _ = m.Update(line)
	}
}
