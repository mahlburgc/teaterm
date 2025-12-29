package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
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
	port         *io.ReadWriteCloser
	scanner      *bufio.Scanner
	selectedPort string
	selectedMode *serial.Mode
	status       int
	sp           spinner.Model
}

func New(port *io.ReadWriteCloser, selectedPort string, selectedMode *serial.Mode) (m Model) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.SpinnerStyle

	scanner := bufio.NewScanner(*port)

	return Model{
		port:         port,
		scanner:      scanner,
		selectedPort: selectedPort,
		selectedMode: selectedMode,
		status:       connected,
		sp:           sp,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.String() {
		case "ctrl+x":
			if m.status == disconnected {
				cmd = func() tea.Msg {
					return events.ConnectionStatusMsg{Status: events.Connecting}
				}
				return m, tea.Batch(m.prepareReconnect(), cmd)
			} else {
				cmd = func() tea.Msg {
					return events.ConnectionStatusMsg{Status: events.Disconnected}
				}
				m.status = disconnected
				(*m.port).Close()
				return m, cmd
			}
		}

	case events.SerialRxMsgReceived:
		return m, m.ReadFromPort()

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

	status += styles.FooterStyle.Render(m.selectedPort)

	return zone.Mark("session", status)
}

// Print out a list of all available ports.
func ListPorts() {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}

	if len(ports) == 0 {
		fmt.Println("No serial ports found!")
		return
	}

	for _, port := range ports {
		fmt.Printf("Found port: %s\n", port.Name)
		if port.IsUSB {
			fmt.Printf("   USB ID     %s:%s\n", port.VID, port.PID)
			fmt.Printf("   USB serial %s\n", port.SerialNumber)
		}
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

// Returns a Tea command to scan for a new receive message on the serial port.
// The tea command returns the received message or error, if occured.
func (m Model) ReadFromPort() tea.Cmd {
	return func() tea.Msg {
		log.Println("Starting read from port")
		for m.scanner.Scan() {
			line := m.scanner.Text()
			return events.SerialRxMsgReceived(line)
		}

		if err := m.scanner.Err(); err != nil {
			if err != io.EOF && err != context.Canceled {
				return events.ErrMsg(err)
			}
		}
		return nil
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
	log.Println("Successfully reconnected to port " + m.selectedPort)
	m.status = connected
	*m.port = port
	m.scanner = bufio.NewScanner(*m.port)

	broadcastConStatusCmd := func() tea.Msg {
		return events.ConnectionStatusMsg{Status: events.Connected}
	}

	return tea.Batch(m.ReadFromPort(), broadcastConStatusCmd)
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
