package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

type (
	SerialRxMsg      string
	SerialTxMsg      string
	PortConnectedMsg struct{ port Port }
)

// Port is an interface that matches io.ReadWriteCloser.
// This interface is used to utilize either a real serial port
// of a mocked serial port for development.
// Both serial.Port and our mockPort implement this interface.
type Port io.ReadWriteCloser

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
		fmt.Println(err)
		os.Exit(1)
	}
	return port, mode
}

// Try to reconnect to the serial port we connected to on startup.
func reconnectToPort(selectedPort string, selectedMode *serial.Mode) tea.Cmd {
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

// Returns a Tea command to scan for a new receive message on the serial port.
// The tea command returns the received message or error, if occured.
func readFromPort(scanner *bufio.Scanner) tea.Cmd {
	return func() tea.Msg {
		log.Println("Starting read from port")
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

// Returns a Tea command to send a message string to the serial port.
// The tea command returns the transmitted message or error, if occured.
func SendToPort(port Port, msg string) tea.Cmd {
	return func() tea.Msg {
		stringToSend := msg + "\r\n" // TODO add custom Lineending
		_, err := port.Write([]byte(stringToSend))
		if err != nil {
			return ErrMsg{
				err: err,
			}
		}
		return SerialTxMsg(msg)
	}
}
