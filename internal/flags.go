package internal

import "flag"

type Flags struct {
	List      bool
	Port      string
	Timestamp bool
}

// Get all command line arguments.
func GetFlags() Flags {
	listArg := flag.Bool("l", false, "list available ports")
	portArg := flag.String("p", "/dev/ttyUSB0", "serial port")
	timestampArg := flag.Bool("t", false, "show timestamp")

	flag.Parse()

	return Flags{
		List:      *listArg,
		Port:      *portArg,
		Timestamp: *timestampArg,
	}
}
