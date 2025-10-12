package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

type (
	SerialRxMsg      string
	PortConnectedMsg struct{ port Port }
)

// Port is an interface that matches io.ReadWriteCloser.
// Both serial.Port and our mockPort implement this.
type Port io.ReadWriteCloser

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

func OpenPort(portname string) (Port, serial.Mode) {
	mode := serial.Mode{
		BaudRate: 115200, // TODO make configurable
	}
	port, err := serial.Open(portname, &mode)
	if err != nil {
		log.Fatal(err)
	}
	return port, mode
}

// try to reconnect to the serial port we connected on startup
func reconnectPort(selectedPort string, selectedMode *serial.Mode) tea.Cmd {
	return func() tea.Msg {
		var port serial.Port
		var err error

		for {
			port, err = serial.Open(selectedPort, selectedMode)
			if err != nil {
				time.Sleep(1000)
			} else {
				break
			}
		}
		return PortConnectedMsg{
			port: port,
		}
	}
}

func readFromPort(scanner *bufio.Scanner) tea.Cmd {
	return func() tea.Msg {
		for scanner.Scan() {
			line := scanner.Text()
			return SerialRxMsg(line)
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF && err != context.Canceled {
				return ErrMsg{
					err: err,
				}
			}
		}
		return nil
	}
}
