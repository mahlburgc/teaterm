package msglog

import (
	"log"
	"os"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// copyToClipboard returns a command that copies text to the clipboard.
// It is a variable so tests can replace it.
var copyToClipboard = func(text string) tea.Cmd {
	return func() tea.Msg {
		// System clipboard (needs wl-copy, xclip or xsel on Linux).
		if err := clipboard.WriteAll(text); err != nil {
			log.Printf("system clipboard: %v", err)
		}

		// OSC 52: the terminal sets the clipboard, also works over SSH.
		seq := osc52.New(text)
		if os.Getenv("TMUX") != "" {
			seq = seq.Tmux()
		}
		if _, err := seq.WriteTo(os.Stderr); err != nil {
			log.Printf("osc52 clipboard: %v", err)
		}
		return nil
	}
}

// SetCopyFunc replaces the clipboard function (for tests outside this
// package) and returns a function that restores the previous one.
func SetCopyFunc(f func(text string) tea.Cmd) (restore func()) {
	orig := copyToClipboard
	copyToClipboard = f
	return func() { copyToClipboard = orig }
}
