package main

import (
	"io"
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

	if len(os.Getenv("DEBUG_TEATERM")) > 0 {
		port, mode = internal.OpenFakePort()
	} else {
		port, mode = internal.OpenPort(flags.Port)
	}

	defer port.Close()

	logger := internal.StartLogger("teaterm_debug.log")
	if logger != nil {
		defer logger.Close()
	}

	internal.RunTui(port, mode, flags, config)
}
