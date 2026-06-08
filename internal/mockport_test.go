package internal

import (
	"io"
	"strings"
	"testing"
	"time"
)

// readChunk reads one chunk from the port with a timeout so a failing test
// doesn't hang.
func readChunk(t *testing.T, port io.ReadWriteCloser) string {
	t.Helper()
	type result struct {
		data string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 4096)
		n, err := port.Read(buf)
		ch <- result{string(buf[:n]), err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("read failed: %v", r.err)
		}
		return r.data
	case <-time.After(2 * time.Second):
		t.Fatal("read timed out")
		return ""
	}
}

// readUntil reads chunks until one contains want, skipping unrelated chunks
// (e.g. measurement samples interleaving with command responses).
func readUntil(t *testing.T, port io.ReadWriteCloser, want string) string {
	t.Helper()
	for i := 0; i < 20; i++ {
		chunk := readChunk(t, port)
		if strings.Contains(chunk, want) {
			return chunk
		}
	}
	t.Fatalf("did not receive chunk containing %q", want)
	return ""
}

func sendCmd(t *testing.T, port io.ReadWriteCloser, cmd string) {
	t.Helper()
	if _, err := port.Write([]byte(cmd + "\r\n")); err != nil {
		t.Fatalf("write %q failed: %v", cmd, err)
	}
}

func TestMockPortCli(t *testing.T) {
	port := NewMockPort()
	defer port.Close()

	// Startup banner is emitted first.
	if banner := readChunk(t, port); !strings.Contains(banner, "Power Monitoring IoT Device") {
		t.Errorf("banner = %q, want it to contain %q", banner, "Power Monitoring IoT Device")
	}

	// help returns the command list.
	sendCmd(t, port, "help")
	if help := readChunk(t, port); !strings.Contains(help, "meas on|off") {
		t.Errorf("help = %q, want it to contain %q", help, "meas on|off")
	}

	// info returns device information.
	sendCmd(t, port, "info")
	if info := readChunk(t, port); !strings.Contains(info, "Version 2.4.1") {
		t.Errorf("info = %q, want it to contain the version", info)
	}

	// Unknown commands return an error.
	sendCmd(t, port, "reboot")
	if errResp := readChunk(t, port); !strings.Contains(errResp, "unknown command 'reboot'") {
		t.Errorf("error response = %q, want it to contain %q", errResp, "unknown command 'reboot'")
	}

	// meas on acks and emits deterministic samples: sample #0 must always
	// hold the t=0 values (vbat 4.02 V, ibat 145 mA, temp 23.8 °C).
	sendCmd(t, port, "meas on")
	readUntil(t, port, "measurement started")
	sample := readUntil(t, port, "sample #0")
	for _, want := range []string{"vbat: 4.02", "ibat: 145", "temp: 23.8"} {
		if !strings.Contains(sample, want) {
			t.Errorf("sample #0 = %q, want it to contain %q", sample, want)
		}
	}

	// meas off acks and stops the cyclic output.
	sendCmd(t, port, "meas off")
	readUntil(t, port, "measurement stopped")
}
