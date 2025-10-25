package internal

import "flag"

type Flags struct {
	List        bool
	Port        string
	Timestamp   bool
	Logfile     bool
	Logfilepath string
}

// Get all command line arguments.
func GetFlags() Flags {
	listArg := flag.Bool("l", false, "list available ports")
	portArg := flag.String("p", "/dev/ttyUSB0", "serial port")
	timestampArg := flag.Bool("t", false, "show timestamp")
	logfileArg := flag.Bool("log", false, "create log file")
	logfilePathArg := flag.String("logpath", ".", "specify logfile dir")

	flag.Parse()

	return Flags{
		List:        *listArg,
		Port:        *portArg,
		Timestamp:   *timestampArg,
		Logfile:     *logfileArg,
		Logfilepath: *logfilePathArg,
	}
}
