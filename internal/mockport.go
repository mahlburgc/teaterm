package internal

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"go.bug.st/serial"
)

// mockPort simulates a serial port for development.
type mockPort struct {
	// Channel to send data to the reading process
	rxChan chan []byte
	// Context to handle closing the port
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMockPort creates and starts a new mock serial port.
func NewMockPort() io.ReadWriteCloser {
	// A context is used to gracefully shut down the goroutine.
	ctx, cancel := context.WithCancel(context.Background())

	m := &mockPort{
		rxChan: make(chan []byte),
		ctx:    ctx,
		cancel: cancel,
	}

	// This goroutine simulates a device sending data periodically.
	go func() {
		// Ticker will fire every 2 seconds.
		// ticker := time.NewTicker(2 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)

		defer ticker.Stop()

		count := 0
		for {
			select {
			case <-ticker.C:
				// The message we want to "receive" in our app.
				msg := []byte(fmt.Sprintf("Hello from mock port! Count: %d\n", count))
				m.rxChan <- msg
				count++
			case <-ctx.Done():
				// If the context is cancelled (by Close()), exit the goroutine.
				return
			}
		}
	}()

	return m
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

// Write simulates sending data. For this mock, we just log it.
func (m *mockPort) Write(p []byte) (n int, err error) {
	log.Printf("MOCK PORT WRITE: %s", string(p))
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
