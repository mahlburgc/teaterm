package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Define the messages we'll use for communication
type serialMsg string
type errMsg struct{ err error }

var (
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230"))

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	endOfBufferStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235"))

	focusedPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder())
)

const gap = "\n"

type model struct {
	viewport       viewport.Model
	messages       []string
	textarea       textarea.Model
	senderStyle    lipgloss.Style
	err            error
	port           serial.Port
	showTimestamp  bool
	commandHistory []string // To store sent commands
	historyIndex   int      // Current position in command history
}

func initialModel(port serial.Port, showTimestamp bool) model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 256
	ta.Cursor.Style = cursorStyle
	ta.FocusedStyle.Placeholder = focusedPlaceholderStyle
	ta.BlurredStyle.Placeholder = placeholderStyle
	ta.FocusedStyle.CursorLine = cursorLineStyle
	ta.FocusedStyle.Base = focusedBorderStyle
	ta.BlurredStyle.Base = blurredBorderStyle
	ta.FocusedStyle.EndOfBuffer = endOfBufferStyle
	ta.BlurredStyle.EndOfBuffer = endOfBufferStyle
	ta.SetHeight(1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // Remove cursor line styling
	ta.ShowLineNumbers = false

	ta.SetWidth(30)
	ta.SetHeight(1)

	vp := viewport.New(30, 5)
	vp.SetContent(`Welcome to the serial monitor!
Waiting for data...`)
	vp.Style = focusedBorderStyle

	// Disable the viewport's default up/down key handling so it doesn't scroll
	// when we are navigating command history.
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea:       ta,
		messages:       []string{},
		viewport:       vp,
		senderStyle:    focusedPlaceholderStyle,
		err:            nil,
		port:           port,
		showTimestamp:  showTimestamp,
		commandHistory: []string{},
		historyIndex:   0,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	// Note: The viewport's update is still called, but it will ignore
	// the up/down keys because we disabled them in its KeyMap.
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Account for the border on the viewport and textarea
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		if len(m.messages) > 0 {
			// Wrap content before setting it.
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.viewport.GotoBottom()

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.ScrollUp(1)
		case tea.MouseButtonWheelDown:
			m.viewport.ScrollDown(1)
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlUp:
			m.viewport.ScrollUp(3)
		case tea.KeyCtrlDown:
			m.viewport.ScrollDown(3)
		case tea.KeyUp:
			if m.historyIndex > 0 {
				m.historyIndex--
				m.textarea.SetValue(m.commandHistory[m.historyIndex])
				m.textarea.SetCursor(len(m.textarea.Value()))
			}
		case tea.KeyDown:
			if m.historyIndex < len(m.commandHistory) {
				m.historyIndex++
				if m.historyIndex < len(m.commandHistory) {
					m.textarea.SetValue(m.commandHistory[m.historyIndex])
					m.textarea.SetCursor(len(m.textarea.Value()))
				} else {
					// Cleared history, reset to empty
					m.textarea.Reset()
				}
			}
		case tea.KeyEnter:
			userInput := m.textarea.Value()
			if userInput == "" {
				return m, nil
			}

			// Add to history
			m.commandHistory = append(m.commandHistory, userInput)
			m.historyIndex = len(m.commandHistory)

			// Send to serial port
			stringToSend := userInput + "\r\n"
			_, err := m.port.Write([]byte(stringToSend))
			if err != nil {
				m.err = fmt.Errorf("error writing to serial port: %w", err)
				return m, nil
			}

			// Log the sent message to the viewport
			var line strings.Builder
			if m.showTimestamp {
				t := time.Now().Format("15:04:05.000")
				line.WriteString(fmt.Sprintf("[%s] ", t))
			}
			line.WriteString("> ")
			line.WriteString(userInput)

			m.messages = append(m.messages, m.senderStyle.Render(line.String()))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			m.textarea.Reset()
		}

	case serialMsg:
		var line strings.Builder
		if m.showTimestamp {
			t := time.Now().Format("15:04:05.000")
			line.WriteString(fmt.Sprintf("[%s] ", t))
		}
		line.WriteString("< ")
		line.WriteString(string(msg))

		m.messages = append(m.messages, line.String())

		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	// We handle errors just like any other message
	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s%s%s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}

// readFromPort continuously reads from the serial port and sends messages to the bubbletea program.
func readFromPort(p *tea.Program, port serial.Port) {
	scanner := bufio.NewScanner(port)
	for scanner.Scan() {
		line := scanner.Text()
		p.Send(serialMsg(line))
	}
	if err := scanner.Err(); err != nil {
		if err != io.EOF && err != context.Canceled {
			p.Send(errMsg{err})
		}
	}
}

// Main routine opens the serial port and starts the transmit
// and receive go routines.
func main() {

	listPtr := flag.Bool("l", false, "list available ports")
	portPtr := flag.String("p", "/dev/ttyUSB0", "serial port")
	timestampPtr := flag.Bool("t", false, "show timestamp")

	flag.Parse()

	listFlag := *listPtr
	showTimestamp := *timestampPtr

	// fmt.Println("list:", listFlag)
	// fmt.Println("port:", *portPtr)

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}
	if len(ports) == 0 {
		fmt.Println("No serial ports found!")
		return
	}

	if listFlag {
		for _, port := range ports {
			fmt.Printf("Found port: %s\n", port.Name)
			if port.IsUSB {
				fmt.Printf("   USB ID     %s:%s\n", port.VID, port.PID)
				fmt.Printf("   USB serial %s\n", port.SerialNumber)
			}
		}
		return
	}

	mode := &serial.Mode{
		BaudRate: 115200,
	}
	port, err := serial.Open(*portPtr, mode)
	if err != nil {
		log.Fatal(err)
	}

	defer port.Close()

	p := tea.NewProgram(initialModel(port, showTimestamp), tea.WithMouseCellMotion())

	go readFromPort(p, port)

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
