package internal

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.bug.st/serial"
)

type model struct {
	serialVp      viewport.Model
	cmdVp         viewport.Model
	inputTa       textarea.Model
	serMsg        []string
	err           error
	port          Port
	scanner       *bufio.Scanner
	selectedPort  string
	selectedMode  *serial.Mode
	showTimestamp bool
	cmdHist       []string
	cmdHistIndex  int
	width         int
	height        int
	conStatus     bool
	spinner       spinner.Model
}

func initialModel(port Port, showTimestamp bool, cmdHist []string,
	selectedPort string, selectedMode *serial.Mode,
) model {
	// Input text area contains text field to send commands to the serial port.
	inputTa := textarea.New()
	inputTa.SetWidth(30)
	inputTa.SetHeight(1)
	inputTa.Placeholder = "Send a message..."
	inputTa.Focus()
	inputTa.Prompt = "> "
	inputTa.CharLimit = 256
	inputTa.ShowLineNumbers = false
	inputTa.KeyMap.InsertNewline.SetEnabled(false)
	inputTa.Cursor.Style = CursorStyle
	inputTa.FocusedStyle.CursorLine = lipgloss.NewStyle()
	inputTa.FocusedStyle.Placeholder = FocusedPlaceholderStyle
	inputTa.FocusedStyle.Prompt = FocusedPromtStyle
	inputTa.BlurredStyle.Prompt = BlurredPromtStyle
	inputTa.FocusedStyle.Base = FocusedBorderStyle
	inputTa.BlurredStyle.Base = BlurredBorderStyle

	// Serial viewport contains all sent and received messages.
	// We will create a viewport without border and later manually
	// add the border to inject a title into the border.
	serialVp := viewport.New(30, 5)
	serialVp.SetContent(`Welcome to teaterm!`)
	serialVp.Style = lipgloss.NewStyle()
	// Disable the viewport's default up/down key handling so it doesn't scroll
	// when we are navigating through the command history.
	serialVp.KeyMap.Up.SetEnabled(false)
	serialVp.KeyMap.Down.SetEnabled(false)
	serialVp.KeyMap.PageUp.SetEnabled(false)
	serialVp.KeyMap.PageDown.SetEnabled(false)

	// Command viewport contains the command history.
	cmdVp := viewport.New(30, 5)
	cmdVp.Style = lipgloss.NewStyle()

	// Spinner symbol runs during port reconnect.
	reconnectSpinner := spinner.New()
	reconnectSpinner.Spinner = spinner.Dot
	reconnectSpinner.Style = SpinnerStyle

	// Scanner searches for incomming serial messages
	scanner := bufio.NewScanner(port)

	return model{
		serialVp:      serialVp,
		cmdVp:         cmdVp,
		inputTa:       inputTa,
		serMsg:        []string{},
		err:           nil,
		port:          port,
		scanner:       scanner,
		selectedPort:  selectedPort,
		selectedMode:  selectedMode,
		showTimestamp: showTimestamp,
		cmdHist:       cmdHist,
		cmdHistIndex:  len(cmdHist),
		width:         0,
		height:        0,
		conStatus:     true,
		spinner:       reconnectSpinner,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, readFromPort(m.scanner))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	LogMsgType(msg)

	m.inputTa, cmd = m.inputTa.Update(msg)
	cmds = append(cmds, cmd)
	m.serialVp, cmd = m.serialVp.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		HandleNewWindowSize(&m, msg)

	case tea.KeyMsg:
		cmd = HandleKeys(&m, msg)
		cmds = append(cmds, cmd)

	case SerialTxMsg:
		HandleSerialTxMsg(&m, string(msg))

	case SerialRxMsg:
		cmd = HandleSerialRxMsg(&m, string(msg))
		cmds = append(cmds, cmd)

	case *serial.PortError:
		reconnectCmd, spinnerCmd := HandleSerialPortErr(&m, msg)
		cmds = append(cmds, reconnectCmd, spinnerCmd)

	case PortConnectedMsg:
		HandlePortReconnect(&m, msg.port)

	case ErrMsg:
		m.err = msg.err

	case spinner.TickMsg:
		if !m.conStatus {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// If there's an error, print it out and don't do anything else.
	if m.err != nil {
		return fmt.Sprintf("\nWe had some trouble: %v\n\n", m.err)
	}

	footer := CreateFooter(&m)

	serialVp := AddBorderAndTitle(m.serialVp, "Messages")
	cmdVp := AddBorderAndTitle(m.cmdVp, "Commands")

	// Arrange viewports side by side
	viewports := lipgloss.JoinHorizontal(
		lipgloss.Top,
		serialVp,
		cmdVp,
	)

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		viewports,
		m.inputTa.View(),
		footer,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		screen,
	)
}

// Adds a border with title to viewport and returns viewport string.
func AddBorderAndTitle(vp viewport.Model, title string) string {
	border := FocusedBorderStyle.GetBorderStyle()

	vpTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
		Render(border.Top + border.MiddleRight + " " + title + " " + border.MiddleLeft)

	// Remove title if width is too low
	if lipgloss.Width(vpTitle) > vp.Width {
		vpTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("")
	}

	// Manually construct the top line of the border with the title inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	serialVpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopLeft),
		vpTitle,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
			Render(strings.Repeat(border.Top, max(0, vp.Width-lipgloss.
				Width(vpTitle)+FocusedBorderStyle.GetHorizontalPadding()))),
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopRight),
	)

	// Render the viewport content inside a box that has NO top border.
	vpBody := FocusedBorderStyle.BorderTop(false).Render(vp.View())

	// Join the title bar and the main content vertically.
	return lipgloss.JoinVertical(lipgloss.Left, serialVpTitleBar, vpBody)
}

// Returns the footer string
func CreateFooter(m *model) string {
	var connectionStatus string
	if m.conStatus {
		connectionStatus = fmt.Sprintf(" %s ", ConnectSymbolStyle.Render("●"))
	} else {
		connectionStatus = fmt.Sprintf(" %s", m.spinner.View())
	}

	helpText := m.selectedPort + " | ↑/↓: cmds · PgUp/PgDn: scroll · ctrl+e: open editor"
	if m.cmdHistIndex != len(m.cmdHist) {
		helpText += " · ctrl+d: del"
	}

	return lipgloss.NewStyle().MaxWidth(m.inputTa.Width()). // TODO check width
								Render(connectionStatus + FooterStyle.Render(helpText))
}

func HandleNewWindowSize(m *model, msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	borderWidth, borderHight := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).GetFrameSize()

	m.serialVp.Width = m.width / 4 * 3
	m.cmdVp.Width = m.width - m.serialVp.Width

	m.serialVp.Width -= borderHight
	m.cmdVp.Width -= borderWidth

	const footerHight = 1
	m.serialVp.Height = m.height - lipgloss.Height(m.inputTa.View()) - borderHight - footerHight
	m.cmdVp.Height = m.serialVp.Height

	m.inputTa.SetWidth(m.width)

	log.Printf("margin v, h:     %v, %v\n", borderHight, borderWidth)
	log.Printf("serial vp  w, h: %v, %v\n", m.serialVp.Width, m.serialVp.Height)
	log.Printf("cmd vp w, h:     %v, %v\n", m.cmdVp.Width, m.cmdVp.Height)
	log.Printf("input ta w, h:   %v, %v\n", m.inputTa.Width(), lipgloss.Height(m.inputTa.View()))

	resetVp(&m.serialVp, &m.serMsg, true)
	resetVp(&m.cmdVp, &m.cmdHist, false)
}

func resetVp(vp *viewport.Model, content *[]string, updateWidth bool) {
	log.Printf("reset vp: vp height, msg len:   %v, %v\n", vp.Height, len(*content))

	if vp.Height > 0 && len(*content) > 0 {
		if updateWidth {
			vp.SetContent(lipgloss.NewStyle().Width(vp.Width).
				Render(strings.Join(*content, "\n")))
		} else {
			vp.SetContent(lipgloss.NewStyle().Render(strings.Join(*content, "\n")))
		}
		vp.GotoBottom()
	}
}

// Handle incomming serial messages.
func HandleSerialRxMsg(m *model, msg string) tea.Cmd {
	var line strings.Builder

	if m.showTimestamp {
		t := time.Now().Format("15:04:05.000")
		line.WriteString(fmt.Sprintf("[%s] ", t))
	}
	//line.WriteString("< ")
	line.WriteString(string(msg))

	// TODO set serial message histrory limit, remove oldest if exceed
	m.serMsg = append(m.serMsg, line.String())
	resetVp(&m.serialVp, &m.serMsg, true)

	// restart msg scanner
	return readFromPort(m.scanner)
}

// A serial messages was successfully sent to the serial port.
// So we log the serial message to the message view and the command view.
func HandleSerialTxMsg(m *model, msg string) {
	// Add command to history.
	// If command is already found in the command histroy, just move command to end to avoid
	// duplicated commands.
	foundIndex := -1
	for i, cmd := range m.cmdHist {
		if cmd == msg {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		m.cmdHist = append(m.cmdHist[:foundIndex], m.cmdHist[foundIndex+1:]...)
		m.cmdHist = append(m.cmdHist, msg)
	} else {
		m.cmdHist = append(m.cmdHist, msg)
	}

	// Reset command history viewport and input text area after sending a command.
	m.inputTa.Reset()
	resetVp(&m.cmdVp, &m.cmdHist, false)
	m.cmdHistIndex = len(m.cmdHist)

	// Log the sent message to the viewport
	var line strings.Builder
	if m.showTimestamp {
		t := time.Now().Format("15:04:05.000")
		line.WriteString(fmt.Sprintf("[%s] ", t))
	}
	// line.WriteString("> ")
	line.WriteString(msg)

	// TODO set serial message histrory limit, remove oldest if exceed
	m.serMsg = append(m.serMsg, VpTxMsgStyle.Render(line.String())) // TODO directly use style for var()
	resetVp(&m.serialVp, &m.serMsg, true)
}

// Handle serial port errors.
// If serial port was closed for any reason, start trying to reconnect to the port
// and start the reconnect spinner symbol.
func HandleSerialPortErr(m *model, msg *serial.PortError) (tea.Cmd, tea.Cmd) {
	if msg.Code() == serial.PortClosed {
		m.inputTa.Reset()
		m.inputTa.Blur()
		m.conStatus = false
		m.port.Close()
		m.inputTa.Placeholder = "Reconnecting..."
		reconnectCmd := reconnectToPort(m.selectedPort, m.selectedMode)
		spinnerCmd := m.spinner.Tick

		return reconnectCmd, spinnerCmd
	}
	return nil, nil
}

// Handle port reconnected event.
func HandlePortReconnect(m *model, port Port) (tea.Cmd, tea.Cmd) {
	m.inputTa.Placeholder = "Send a message..." // TODO remove duplicated code
	cursorBlinkCmd := m.inputTa.Focus()
	m.conStatus = true
	m.port = port
	m.scanner = bufio.NewScanner(m.port)
	readCmd := readFromPort(m.scanner)

	return cursorBlinkCmd, readCmd
}

func RunTui(port Port, mode serial.Mode, flags Flags, config Config) {
	m := initialModel(port, flags.Timestamp, config.CmdHistoryLines, flags.Port, &mode)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
