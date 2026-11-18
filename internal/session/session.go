package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Port is an interface that matches io.ReadWriteCloser.
// This interface is used to utilize either a real serial port
// of a mocked serial port for development.
// Both serial.Port and our mockPort implement this interface.
type Port io.ReadWriteCloser

const (
	disconnected = iota
	connecting
	connected
)

type (
	portReconnectedStatusMsg struct {
		port Port
		ok   bool
	}
	startNextReconnectTryMsg bool
)

type Model struct {
	port             *io.ReadWriteCloser
	rx               *rxBuffer
	selectedPort     string
	selectedMode     *serial.Mode
	status           int
	sp               spinner.Model
	ctx              context.Context
	cancel           context.CancelFunc
	showFullPortName bool
}

func New(port *io.ReadWriteCloser, selectedPort string, selectedMode *serial.Mode) (m Model) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.SpinnerStyle

	rx := startReader(*port)
	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		port:             port,
		rx:               rx,
		selectedPort:     selectedPort,
		selectedMode:     selectedMode,
		status:           connected,
		sp:               sp,
		ctx:              ctx,
		cancel:           cancel,
		showFullPortName: false,
	}
}

func (m Model) Init() tea.Cmd {
	if m.status == connected {
		return m.ReadFromPort(m.ctx)
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.ToggleSessionKey):
			if m.status == disconnected {
				cmd = func() tea.Msg {
					return events.ConnectionStatusMsg{Status: events.Connecting}
				}
				return m, tea.Batch(m.prepareReconnect(), cmd)
			} else {
				m.status = disconnected
				if m.cancel != nil {
					m.cancel()
				}
				cmd = func() tea.Msg {
					return events.ConnectionStatusMsg{Status: events.Disconnected}
				}
				(*m.port).Close()
				return m, cmd
			}
		}

	case events.SerialRxMsgReceived:
		m.rx.ack(string(msg))
		return m, m.ReadFromPort(m.ctx)

	case spinner.TickMsg:
		if m.status == connecting {
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}

	case events.SendMsg:
		return m, m.sendToPort(msg.Data)

	case portReconnectedStatusMsg:
		if msg.ok {
			return m, m.handlePortReconnected(msg.port)
		} else {
			cmd = func() tea.Msg {
				time.Sleep(1 * time.Second)
				return startNextReconnectTryMsg(true)
			}
			return m, cmd
		}

	case startNextReconnectTryMsg:
		if m.status != disconnected {
			return m, tea.Batch(m.prepareReconnect())
		}

	case events.ErrMsg:
		switch msg := msg.(type) {
		case *serial.PortError:
			return m, m.HandleSerialPortErr(msg)
		}

	case tea.MouseMsg:
		if zone.Get("session").InBounds(msg) && msg.Action == tea.MouseActionRelease {
			m.showFullPortName = !m.showFullPortName
		}

	default:
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	var status string

	switch m.status {
	case connected:
		status = fmt.Sprintf(" %s ", styles.ConnectSymbolStyle.Render("●"))

	case disconnected:
		status = fmt.Sprintf(" %s ", styles.DisconnectedSymbolStyle.Render("●"))

	case connecting:
		status = fmt.Sprintf(" %s", m.sp.View())
	}

	portname := m.selectedPort
	if !m.showFullPortName && len(m.selectedPort) > 14 {
		portname = "..." + m.selectedPort[len(m.selectedPort)-11:]
	}

	status += styles.FooterStyle.Render(portname)

	return zone.Mark("session", status)
}

func ListPorts() {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}

	if len(ports) == 0 {
		fmt.Println("No serial ports found!")
		return
	}

	portMap := make(map[string]*enumerator.PortDetails)
	for _, p := range ports {
		portMap[p.Name] = p
	}

	byIdDir := "/dev/serial/by-id"
	entries, _ := os.ReadDir(byIdDir)

	fmt.Printf("%-15s  %-70s\n", "Device", "By-id")
	fmt.Println(strings.Repeat("-", 15) + "  " + strings.Repeat("-", 70))

	for _, entry := range entries {
		fullPath := filepath.Join(byIdDir, entry.Name())
		realPath, _ := filepath.EvalSymlinks(fullPath)

		fmt.Printf("%-15s  %-70s\n", realPath, fullPath)
	}
}

// Open a port and return the port and the serial mode.
func OpenPort(portname string) (Port, serial.Mode) {
	mode := serial.Mode{
		BaudRate: 115200, // TODO make configurable
	}
	port, err := serial.Open(portname, &mode)
	if err != nil {
		fmt.Printf("%s: %s\n", portname, err.Error())
		os.Exit(1)
	}
	return port, mode
}

// Returns a tea command that tries to reconnect to the serial port we connected
// to on startup.
func reconnectToPort(selectedPort string, selectedMode *serial.Mode) tea.Cmd {
	return func() tea.Msg {
		port, err := serial.Open(selectedPort, selectedMode)
		if err != nil {
			log.Println("Failed to reconnect to port " + selectedPort)
		}
		return portReconnectedStatusMsg{port: port, ok: err == nil}
	}
}

// rxBuffer decouples reading the serial port from the bubbletea event loop.
// A single goroutine (see startReader) owns the bufio.Scanner and appends every
// received line to the queue. This keeps the port drained even while the TUI is
// suspended (e.g. external editor) and guarantees that the scanner is never used
// by more than one goroutine, also across bubbletea program restarts.
type rxBuffer struct {
	mu     sync.Mutex
	lines  []string
	signal chan struct{} // closed and replaced on every state change (broadcast)
	done   bool
	err    error
}

// startReader starts the reader goroutine for the given port and returns the
// buffer it feeds. The goroutine ends when the port returns an error or EOF.
func startReader(port io.Reader) *rxBuffer {
	rx := &rxBuffer{signal: make(chan struct{})}

	go func() {
		scn := bufio.NewScanner(port)
		for scn.Scan() {
			rx.mu.Lock()
			rx.lines = append(rx.lines, scn.Text())
			rx.broadcastLocked()
			rx.mu.Unlock()
		}
		rx.mu.Lock()
		rx.done = true
		rx.err = scn.Err()
		rx.broadcastLocked()
		rx.mu.Unlock()
	}()

	return rx
}

func (rx *rxBuffer) broadcastLocked() {
	close(rx.signal)
	rx.signal = make(chan struct{})
}

// ack removes the oldest line from the queue once it has been delivered to the
// event loop. Lines are only peeked by ReadFromPort and removed here, so a line
// handed to an already terminated bubbletea program (app restart after closing
// the editor) is not lost but delivered again by the next program.
func (rx *rxBuffer) ack(line string) {
	rx.mu.Lock()
	defer rx.mu.Unlock()
	if len(rx.lines) > 0 && rx.lines[0] == line {
		rx.lines = rx.lines[1:]
	}
}

// Returns a Tea command that waits for the next received line on the serial port.
// The tea command returns the received message or error, if occured.
func (m Model) ReadFromPort(ctx context.Context) tea.Cmd {
	rx := m.rx

	return func() tea.Msg {
		for {
			rx.mu.Lock()
			if len(rx.lines) > 0 {
				line := rx.lines[0]
				rx.mu.Unlock()
				return events.SerialRxMsgReceived(line)
			}
			if rx.done {
				err := rx.err
				rx.mu.Unlock()

				// Check if we manually canceled the context (Manual Disconnect)
				select {
				case <-ctx.Done():
					return events.InfoMsg("Port closed manually")
				default:
					// Context is still active, proceed to check actual errors
				}

				if err != nil && err != io.EOF && err != context.Canceled {
					return events.ErrMsg(err)
				}
				return nil
			}
			signal := rx.signal
			rx.mu.Unlock()
			<-signal
		}
	}
}

// Returns a Tea command to send a message string to the serial port.
// The tea command returns the transmitted message or error, if occured.
func (m Model) sendToPort(msg string) tea.Cmd {
	return func() tea.Msg {
		stringToSend := msg + "\r\n" // TODO add custom Lineending
		_, err := (*m.port).Write([]byte(stringToSend))
		if err != nil {
			return events.ErrMsg(err)
		}
		return nil
	}
}

// Prepare TUI to reconnect
func (m *Model) prepareReconnect() tea.Cmd {
	m.status = connecting
	(*m.port).Close()
	startReconnectCmd := reconnectToPort(m.selectedPort, m.selectedMode)
	spinnerCmd := m.sp.Tick
	return tea.Batch(startReconnectCmd, spinnerCmd)
}

// Handle port reconnected event.
func (m *Model) handlePortReconnected(port Port) tea.Cmd {
	log.Println("Port reconnected")
	m.status = connected
	*m.port = port
	m.rx = startReader(*m.port)

	if m.cancel != nil {
		m.cancel()
	}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	broadcastConStatusCmd := func() tea.Msg {
		return events.ConnectionStatusMsg{Status: events.Connected}
	}

	broadcastInfoMsgCmd := func() tea.Msg {
		return events.InfoMsg("Port reconnected")
	}

	return tea.Batch(m.ReadFromPort(m.ctx), broadcastConStatusCmd, broadcastInfoMsgCmd)
}

// Handle serial port errors.
// If serial port was closed for any reason, start trying to reconnect to the port
// and start the reconnect spinner symbol.
func (m *Model) HandleSerialPortErr(msg *serial.PortError) tea.Cmd {
	if msg.Code() == serial.PortClosed {
		if m.status != disconnected {
			cmd := func() tea.Msg {
				return events.ConnectionStatusMsg{Status: events.Connecting}
			}
			return tea.Batch(m.prepareReconnect(), cmd)
		}
	}
	return nil
}
