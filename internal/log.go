package internal

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Opens (or create if no exist) a log file for debug logging.
func StartLogger(logfile string) *os.File {
	if len(os.Getenv("DEBUG_TEATERM")) > 0 {
		logfile, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		return logfile
	} else {
		log.SetOutput(io.Discard)
	}

	return nil
}
