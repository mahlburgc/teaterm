package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fatih/color"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

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

const gap = "\n\n"

type (
	errMsg error
)

type model struct {
	viewport      viewport.Model
	messages      []string
	textarea      textarea.Model
	senderStyle   lipgloss.Style
	err           error
	port          serial.Port
	showTimestamp bool
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
	//ta.Blur()

	ta.SetWidth(30)
	ta.SetHeight(1)

	vp := viewport.New(30, 5)
	vp.SetContent(`Welcome to the serial monitor!
Waiting for data...`)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea:      ta,
		messages:      []string{},
		viewport:      vp,
		senderStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:           nil,
		port:          port,
		showTimestamp: showTimestamp,
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

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		if len(m.messages) > 0 {
			// Wrap content before setting it.
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.viewport.GotoBottom()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.textarea.Value())
			return m, tea.Quit
		case tea.KeyEnter:
			m.messages = append(m.messages, m.senderStyle.Render("You: ")+m.textarea.Value())
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
			m.textarea.Reset()
			m.viewport.GotoBottom()
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
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

// ReadFromPort should be called as go routine.
// The function is intended to run continuously
// and read the input stream from a serial port.
func ReadFromPort(port serial.Port, showTimestamp bool, showColoredOutput bool) {
	scanner := bufio.NewScanner(port)

	for scanner.Scan() {
		line := scanner.Text()
		var msg strings.Builder
		if showTimestamp {
			t := time.Now().Format("15:04:05.000")
			fmt.Fprintf(&msg, "[%s] ", t)
		}
		// msg.WriteString("<-- ")
		msg.WriteString(line)
		if showColoredOutput {
			color.RGB(0, 128, 255).Println(msg.String())
		} else {
			fmt.Println(msg.String())
		}
	}

	if err := scanner.Err(); err != nil {
		// io.EOF is a "normal" error when the port is closed.
		if err != io.EOF {
			log.Printf("Error reading from serial port: %v", err)
		}
	}
}

// WriteToPort should be called as go routine.
// The function is intended to run continuously, read from
// user input stream and write the user input to the serial port.
func WriteToPort(port serial.Port, showTimestamp bool, showColoredOutput bool) {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		userInput := scanner.Text()
		stringToSend := userInput + "\r\n" // append line ending, TODO make configurable

		_, err := port.Write([]byte(stringToSend))
		if err != nil {
			log.Printf("Error writing to serial port: %v", err)
			return // TODO what to do on error, e.g. if the serial port is closed
		} else {
			var msg strings.Builder
			if showTimestamp {
				t := time.Now().Format("15:04:05.000")
				fmt.Fprintf(&msg, "[%s] ", t)
			}
			// msg.WriteString("--> ")
			msg.WriteString(userInput)
			if showColoredOutput {
				color.RGB(128, 0, 255).Println(msg.String())
			} else {
				fmt.Println(msg.String())
			}
		}
	}

	// This part is reached if scanner.Scan() returns false, usually on an error or EOF.
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from user input: %v", err)
	}
}

// Main routine opens the serial port and starts the transmit
// and receive go routines.
func main() {

	listPtr := flag.Bool("l", false, "list available ports")
	portPtr := flag.String("p", "/dev/ttyUSB0", "serial port")
	timestampPtr := flag.Bool("t", false, "show timestamp")
	coloredOutputPtr := flag.Bool("c", false, "show colored output")

	flag.Parse()

	listFlag := *listPtr
	showTimestamp := *timestampPtr
	showColoredOutput := *coloredOutputPtr

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

	fmt.Println("Start send and receive go rountine.")
	go ReadFromPort(port, showTimestamp, showColoredOutput)
	// go WriteToPort(port, showTimestamp, showColoredOutput)

	p := tea.NewProgram(initialModel(port, showTimestamp))

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
