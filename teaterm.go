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

	var port io.ReadWriteCloser
	var mode serial.Mode
	if len(os.Getenv("TEATERM_MOCK_PORT")) > 0 {
		port, mode = internal.OpenFakePort()
	} else {
		port, mode = internal.OpenPort(flags.Port)
	}
	defer port.Close()

	if len(os.Getenv("TEATERM_DBG_LOG")) > 0 {
		closeDbgLogger := internal.StartDbgLogger()
		defer closeDbgLogger()
	} else {
		log.SetOutput(io.Discard)
	}

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
