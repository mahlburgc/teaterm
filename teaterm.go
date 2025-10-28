package main

import (
	"io"
	"log"
	"os"

	"github.com/mahlburgc/teaterm/internal"
	"go.bug.st/serial"
)

func main() {
	config := internal.GetConfig()
	flags := internal.GetFlags()

	if flags.List {
		internal.ListPorts()
		return
	}

	if len(os.Getenv("TEATERM_DBG_LOG")) > 0 {
		closeDbgLogger := internal.StartDbgLogger()
		defer closeDbgLogger()
	} else {
		log.SetOutput(io.Discard)
	}

	var initialPort io.ReadWriteCloser
	var mode serial.Mode
	if len(os.Getenv("TEATERM_MOCK_PORT")) > 0 {
		initialPort, mode = internal.OpenFakePort()
	} else {
		initialPort, mode = internal.OpenPort(flags.Port)
	}

	// During program execution it might happen that the serial port is closed and opened
	// again due to physical connection lost. So we pass a pointer to the programm
	// to make sure we close the correct port after program termination.
	port := &initialPort

	// Defer a function that safely closes the port object pointed to by the pointer.
	// The anonymous function is evaluated immediately, but the body runs at 'defer' time.
	defer func() {
		if *port != nil {
			(*port).Close()
			log.Println("Deferred port close executed.")
		}
	}()

	log.Printf("Logfile %v", flags.Logfile)
	var serialLog *log.Logger
	if flags.Logfile {
		log.Println("Create Serial Logger")
		var closeSerialLogger func()
		serialLog, closeSerialLogger = internal.StartSerialLogger(flags.Logfilepath)
		if closeSerialLogger == nil {
			log.Println("ERROR: closeSerialLogger is nil. Serial logger setup failed.")
		} else {
			log.Println("SUCCESS: closeSerialLogger is not nil.")
		}
		defer closeSerialLogger()
	}

	internal.RunTui(port, mode, flags, config, serialLog)
}
