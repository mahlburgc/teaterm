package internal

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
)

// Open (or create if no exist) a log file for debug logging.
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

// Log messsage type to debug file
func LogMsgType(msg any) {
	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		// avoid logging on cursor blink messages
	default:
		log.Printf("Update Msg: Type: %T Value: %v\n", msg, msg)
	}
}
