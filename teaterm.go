package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// ReadFromPort should be called as go routine.
// The function is intended to run continuously
// and read the input stream from a serial port.
func ReadFromPort(port serial.Port, showTimestamp bool, showColoredOutput bool) {
	scanner := bufio.NewScanner(port)

	for scanner.Scan() {
		line := scanner.Text()
		var msg strings.Builder
		if showTimestamp {
			t := time.Now().Format("15:04:05.000")
			fmt.Fprintf(&msg, "[%s] ", t)
		}
		// msg.WriteString("<-- ")
		msg.WriteString(line)
		if showColoredOutput {
			color.RGB(0, 128, 255).Println(msg.String())
		} else {
			fmt.Println(msg.String())
		}
	}

	if err := scanner.Err(); err != nil {
		// io.EOF is a "normal" error when the port is closed.
		if err != io.EOF {
			log.Printf("Error reading from serial port: %v", err)
		}
	}
}

// WriteToPort should be called as go routine.
// The function is intended to run continuously, read from
// user input stream and write the user input to the serial port.
func WriteToPort(port serial.Port, showTimestamp bool, showColoredOutput bool) {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		userInput := scanner.Text()
		stringToSend := userInput + "\r\n" // append line ending, TODO make configurable

		_, err := port.Write([]byte(stringToSend))
		if err != nil {
			log.Printf("Error writing to serial port: %v", err)
			return // TODO what to do on error, e.g. if the serial port is closed
		} else {
			var msg strings.Builder
			if showTimestamp {
				t := time.Now().Format("15:04:05.000")
				fmt.Fprintf(&msg, "[%s] ", t)
			}
			// msg.WriteString("--> ")
			msg.WriteString(userInput)
			if showColoredOutput {
				color.RGB(128, 0, 255).Println(msg.String())
			} else {
				fmt.Println(msg.String())
			}
		}
	}

	// This part is reached if scanner.Scan() returns false, usually on an error or EOF.
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from user input: %v", err)
	}
}

// Main routine opens the serial port and starts the transmit
// and receive go routines.
func main() {

	listPtr := flag.Bool("l", false, "list available ports")
	portPtr := flag.String("p", "/dev/ttyUSB0", "serial port")
	timestampPtr := flag.Bool("t", false, "show timestamp")
	coloredOutputPtr := flag.Bool("c", false, "show colored output")

	flag.Parse()

	listFlag := *listPtr
	showTimestamp := *timestampPtr
	showColoredOutput := *coloredOutputPtr

	// fmt.Println("list:", listFlag)
	// fmt.Println("port:", *portPtr)

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}
	if len(ports) == 0 {
		fmt.Println("No serial ports found!")
		return
	}

	if listFlag {
		for _, port := range ports {
			fmt.Printf("Found port: %s\n", port.Name)
			if port.IsUSB {
				fmt.Printf("   USB ID     %s:%s\n", port.VID, port.PID)
				fmt.Printf("   USB serial %s\n", port.SerialNumber)
			}
		}
		return
	}

	mode := &serial.Mode{
		BaudRate: 115200,
	}
	port, err := serial.Open(*portPtr, mode)
	if err != nil {
		log.Fatal(err)
	}

	defer port.Close()

	fmt.Println("Start send and receive go rountine.")
	go ReadFromPort(port, showTimestamp, showColoredOutput)
	go WriteToPort(port, showTimestamp, showColoredOutput)

	for {
		time.Sleep(2 * time.Second)
	}
}
