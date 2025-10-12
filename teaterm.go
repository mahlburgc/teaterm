package main

import (
	"github.com/mahlburgc/teaterm/internal"
)

func main() {
	config := internal.GetConfig()
	flags := internal.GetFlags()

	if flags.List {
		internal.ListPorts()
		return
	}

	port, mode := internal.OpenPort(flags.Port)
	defer port.Close()

	logger := internal.StartLogger("teaterm_debug.log")
	if logger != nil {
		defer logger.Close()
	}

	internal.RunTui(port, mode, flags, config)
}
