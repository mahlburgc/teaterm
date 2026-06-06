package internal

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"go.bug.st/serial"
)

// measInterval is the cycle time of the mocked power measurement.
const measInterval = 200 * time.Millisecond

// Subtle SGR colors for the mocked device output. teaterm's message log
// sanitizer preserves color sequences, so they show up in the TUI.
const (
	colReset = "\x1b[0m"
	colDim   = "\x1b[2m"
	colBold  = "\x1b[1m"
	colCyan  = "\x1b[36m"
	colGreen = "\x1b[32m"
	colRed   = "\x1b[31m"
)

// mockPort simulates a serial port connected to a small battery/power
// monitor IoT device with a line based CLI. Supported commands:
//
//	help          show available commands
//	info          show device information
//	meas on|off   start/stop cyclic power measurement
//
// All output is deterministic (no randomness, no wall clock) so demo
// recordings (vhs tapes) are reproducible.
type mockPort struct {
	// Channel to send data to the reading process
	rxChan chan []byte
	// Channel to pass commands from Write to the device goroutine
	cmdChan chan string
	// Context to handle closing the port
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMockPort creates and starts a new mock serial port.
func NewMockPort() io.ReadWriteCloser {
	// A context is used to gracefully shut down the goroutine.
	ctx, cancel := context.WithCancel(context.Background())

	m := &mockPort{
		rxChan:  make(chan []byte),
		cmdChan: make(chan string),
		ctx:     ctx,
		cancel:  cancel,
	}

	go m.runDevice()

	return m
}

// runDevice is the device goroutine. It owns all device state and reacts to
// commands from Write and to the measurement ticker.
func (m *mockPort) runDevice() {
	ticker := time.NewTicker(measInterval)
	defer ticker.Stop()

	measOn := false
	sample := 0 // measurement sample counter, only advances while meas is on
	ticks := 0  // uptime counter

	m.sendBanner()

	for {
		select {
		case cmd := <-m.cmdChan:
			switch cmd {
			case "":
				// ignore empty lines

			case "help":
				m.send(
					"Available commands:",
					"  help          show this help",
					"  info          show device information",
					"  meas on|off   start/stop cyclic power measurement",
				)

			case "info":
				m.send(
					"SYS: Power Monitoring IoT Device",
					"SYS: Version 2.4.1",
					"SYS: battery monitor "+colGreen+"OK"+colReset,
					fmt.Sprintf("SYS: uptime: %s", uptime(ticks)),
				)

			case "meas on":
				if measOn {
					m.send("PWR: measurement already running")
				} else {
					measOn = true
					m.send("PWR: " + colGreen + "OK" + colReset + ", measurement started (200 ms interval)")
				}

			case "meas off":
				if !measOn {
					m.send("PWR: measurement not running")
				} else {
					measOn = false
					m.send("PWR: " + colGreen + "OK" + colReset + ", measurement stopped")
				}

			default:
				m.send(colRed + "ERR: unknown command '" + cmd + "'" + colReset)
			}

		case <-ticker.C:
			ticks++
			if measOn {
				m.sendMeasSample(sample)
				sample++
			}

		case <-m.ctx.Done():
			return
		}
	}
}

// sendBanner emits the boot message of the mocked device.
func (m *mockPort) sendBanner() {
	m.send(
		colGreen+"**************************************"+colReset,
		colGreen+"* "+colReset+"Mocked Power Monitoring IoT Device"+colGreen+" *"+colReset,
		colGreen+"* "+colReset+"Type 'help' for available commands"+colGreen+" *"+colReset,
		colGreen+"**************************************"+colReset,
	)
}

// sendMeasSample emits one measurement block. The values are pure functions
// of the sample counter, so every run produces identical output.
func (m *mockPort) sendMeasSample(sample int) {
	t := float64(sample) * measInterval.Seconds()

	vbat := 4.02 - 0.004*t + 0.008*math.Sin(1.3*t) // slow discharge + ripple
	ibat := 145 + 35*math.Sin(0.9*t) + 8*math.Sin(3.7*t)
	temp := 23.8 + 0.05*t + 0.15*math.Sin(0.6*t)

	m.send(
		fmt.Sprintf("\nPWR: meas sample #%d", sample),
		fmt.Sprintf("PWR:  vbat: %.2f V", vbat),
		fmt.Sprintf("PWR:  ibat: %.0f mA", ibat),
		fmt.Sprintf("PWR:  temp: %.1f °C", temp),
	)
}

// uptime formats the tick counter as hh:mm:ss.
func uptime(ticks int) string {
	d := time.Duration(ticks) * measInterval
	return fmt.Sprintf("%02d:%02d:%02d",
		int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

// send pushes the given lines as one chunk to the reading process. The
// ctx.Done guard keeps the device goroutine from leaking if nobody reads
// (e.g. in tests) and the port gets closed.
func (m *mockPort) send(lines ...string) {
	data := []byte(strings.Join(lines, "\n") + "\n")
	select {
	case m.rxChan <- data:
	case <-m.ctx.Done():
	}
}

// Read blocks until a message is available on the rxChan or the context is done.
func (m *mockPort) Read(p []byte) (n int, err error) {
	select {
	case data := <-m.rxChan:
		// We have data, copy it to the buffer p.
		n = copy(p, data)
		return n, nil
	case <-m.ctx.Done():
		// The port was closed, return EOF.
		return 0, io.EOF
	}
}

// Write parses the incoming data as a CLI command and passes it to the
// device goroutine. Commands arrive with a trailing line ending.
func (m *mockPort) Write(p []byte) (n int, err error) {
	log.Printf("MOCK PORT WRITE: %s", string(p))
	cmd := strings.ToLower(strings.TrimSpace(string(p)))
	select {
	case m.cmdChan <- cmd:
	case <-m.ctx.Done():
	}
	return len(p), nil
}

// Close stops the mock port's internal goroutine.
func (m *mockPort) Close() error {
	log.Println("MOCK PORT: Closing")
	m.cancel() // This will trigger the ctx.Done() in Read and the goroutine.
	return nil
}

// OpenFakePort is a convenient wrapper for creating the mock.
func OpenFakePort() (io.ReadWriteCloser, serial.Mode) {
	mode := serial.Mode{
		BaudRate: 115200, // TODO make configurable
	}

	return NewMockPort(), mode
}
