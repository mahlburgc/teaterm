package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mahlburgc/teaterm/events"
)

// Returns a function to close a specific logger and check for errors.
func CloseLogger(f *os.File, closemsg string) func() {
	return func() {
		if err := f.Close(); err != nil {
			fmt.Printf("Warning: Failed to close log file %s: %v\n", f.Name(), err)
		} else if closemsg != "" {
			fmt.Println(closemsg)
		}
	}
}

// Open (or create if no exist) a log file for debug logging.
// Returns a close function.
// Debug logging uses the standard logger.
func StartDbgLogger() (close func()) {
	filepath := "debug.log"
	f, err := tea.LogToFile(filepath, "debug")
	if err != nil {
		fmt.Printf("fatal: Failed to open log file %s: %v\n", filepath, err)
		os.Exit(1)
	}
	return CloseLogger(f, "")
}

// Log messsage type to debug file
func DbgLogMsgType(msg any) {
	switch msg := msg.(type) {
	case cursor.BlinkMsg, spinner.TickMsg, events.SerialRxMsg:
		// avoid logging on spamming messages
	default:
		log.Printf("Update Msg: Type: %T Value: %v\n", msg, msg)
	}
}

// Open (or create if no exist) a log file for serial logging.
// Returns a close function and the serial logger.
func StartSerialLogger(logDirPath string) (*log.Logger, func()) {
	// Generate the unique file name: <date>-<time>-teamterm.log
	// Format: YYYYMMDD-HHMMSS-teamterm.log
	fileName := "teaterm-" + time.Now().Format("2006-01-02T15:04:05") + ".log"
	// fileName := "teaterm.log"
	fullPath := filepath.Join(logDirPath, fileName)

	// Check if the directory exists.
	dirInfo, err := os.Stat(logDirPath)
	if os.IsNotExist(err) {
		fmt.Printf("fatal: Serial log directory does not exist: %s\n", logDirPath)
		os.Exit(1)
	}

	// Check if the path is actually a directory
	if err == nil && !dirInfo.IsDir() {
		fmt.Printf("fatal: Path exists but is not a directory: %s\n", logDirPath)
		os.Exit(1)
	}

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		fmt.Printf("fatal: Failed to open log file %s: %v\n", fullPath, err)
		os.Exit(1)
	}

	log.Println("Create serial logger at " + fullPath)

	serialLogger := log.New(f, "", 0) // flags could be log.Ldate|log.Ltime|log.Lmicroseconds

	return serialLogger, CloseLogger(f, "Logfile created under "+fullPath)
}
