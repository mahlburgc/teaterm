package internal

import (
	"fmt"
	"log"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

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

func OpenPort(portname string) (serial.Port, serial.Mode) {
	mode := serial.Mode{
		BaudRate: 115200, // TODO make configurable
	}
	port, err := serial.Open(portname, &mode)
	if err != nil {
		log.Fatal(err)
	}
	return port, mode
}
