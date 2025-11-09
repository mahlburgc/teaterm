package internal

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/icza/gox/stringsx"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/internal/cmdhist"
	"go.bug.st/serial"
)

type StartMextReconnectTryMsg bool

const (
	conStatus_disconnected = iota
	conStatus_connecting   = iota
	conStatus_connected    = iota
)

type model struct {
	serialVp      viewport.Model
	cmdhist       cmdhist.Model
	inputTa       textarea.Model
	serMsg        []string
	err           error
	port          *io.ReadWriteCloser
	scanner       *bufio.Scanner
	selectedPort  string
	selectedMode  *serial.Mode
	showTimestamp bool
	restartApp    bool
	width         int
	height        int
	conStatus     int
	spinner       spinner.Model
	serialLog     *log.Logger
	showEscapes   bool
}

func initialModel(port *io.ReadWriteCloser, showTimestamp bool, cmdHist []string,
	selectedPort string, selectedMode *serial.Mode, serialLog *log.Logger, showEscapes bool,
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
	cmdhist := cmdhist.New(cmdHist)

	// Spinner symbol runs during port reconnect.
	reconnectSpinner := spinner.New()
	reconnectSpinner.Spinner = spinner.Dot
	reconnectSpinner.Style = SpinnerStyle

	// Scanner searches for incomming serial messages
	scanner := bufio.NewScanner(*port)

	return model{
		serialVp:      serialVp,
		cmdhist:       cmdhist,
		inputTa:       inputTa,
		serMsg:        []string{},
		err:           nil,
		port:          port,
		scanner:       scanner,
		selectedPort:  selectedPort,
		selectedMode:  selectedMode,
		showTimestamp: showTimestamp,
		width:         0,
		height:        0,
		conStatus:     conStatus_connected,
		spinner:       reconnectSpinner,
		restartApp:    false,
		serialLog:     serialLog,
		showEscapes:   showEscapes,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, readFromPort(m.scanner))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	DbgLogMsgType(msg)

	if m.conStatus == conStatus_connected {
		m.cmdhist, cmd = m.cmdhist.Update(msg)
		cmds = append(cmds, cmd)
	}
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

	case PortReconnectStatusMsg:
		if msg.ok {
			cmd1, cmd2 := HandlePortReconnect(&m, msg.port)
			cmds = append(cmds, tea.Batch(cmd1, cmd2))
		} else {
			cmd := func() tea.Msg {
				time.Sleep(1 * time.Second)
				return StartMextReconnectTryMsg(true)
			}
			cmds = append(cmds, cmd)

		}

	case StartMextReconnectTryMsg:
		if m.conStatus != conStatus_disconnected {
			reconnectCmd, spinnerCmd := PrepareReconnect(&m)
			cmds = append(cmds, reconnectCmd, spinnerCmd)
		}

	case ErrMsg:
		switch msg.err.(type) {
		case *serial.PortError:
			reconnectCmd, spinnerCmd := HandleSerialPortErr(&m, msg.err.(*serial.PortError))
			cmds = append(cmds, reconnectCmd, spinnerCmd)
		default:
			m.err = msg.err
		}

	case PortManualConnectMsg:
		if m.conStatus == conStatus_disconnected {
			reconnectCmd, spinnerCmd := PrepareReconnect(&m)
			cmds = append(cmds, reconnectCmd, spinnerCmd)
		} else {
			m.conStatus = conStatus_disconnected
			m.inputTa.Reset()
			m.inputTa.Blur()
			(*m.port).Close()
			m.inputTa.Placeholder = "Disconnected"
		}

	case spinner.TickMsg:
		if m.conStatus == conStatus_connecting {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case cmdhist.CmdHistMsg:
		switch msg.Type {
		case cmdhist.CmdSelected:
			m.inputTa.SetValue(msg.Cmd)

		case cmdhist.CmdExecuted:
			cmd = SendToPort(*m.port, msg.Cmd)
			cmds = append(cmds, cmd)
		}

	case editorFinishedMsg:
		// workaround bubbletea v1 bug: after executing external command,
		// mouse support is not restored correctly. Therefore we restart bubbletea.
		m.restartApp = true
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// If there's an error, print it out and don't do anything else.
	if m.err != nil {
		return fmt.Sprintf("\nWe had some trouble: %v\n\n", m.err)
	}

	serialVpFooter := fmt.Sprintf("%v, %3.f%%", len(m.serMsg), m.serialVp.ScrollPercent()*100)

	footer := CreateFooter(&m)
	serialVp := AddBorder(m.serialVp, "Messages", serialVpFooter)
	cmdVp := AddBorder(m.cmdhist.Vp, "Commands", "")

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

	return zone.Scan(lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		screen))
}

// Adds a border with title to viewport and returns viewport string.
func AddBorder(vp viewport.Model, title string, footer string) string {
	border := FocusedBorderStyle.GetBorderStyle()
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

	var vpTitle string

	if title == "" {
		vpTitle = ""
	} else {
		vpTitle = borderStyle.Render(border.Top + border.MiddleRight + " " + title + " " + border.MiddleLeft)
		// Remove title if width is too low
		if lipgloss.Width(vpTitle) > vp.Width {
			vpTitle = ""
		}
	}

	// Manually construct the top line of the border with the title inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	vpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		borderStyle.Render(border.TopLeft),
		vpTitle,
		borderStyle.
			Render(strings.Repeat(border.Top, max(0, vp.Width-lipgloss.
				Width(vpTitle)+FocusedBorderStyle.GetHorizontalPadding()))),
		borderStyle.Render(border.TopRight),
	)

	var vpFooter string

	if footer == "" {
		vpFooter = ""
	} else {
		vpFooter = borderStyle.Render(border.MiddleRight + " " + footer + " " +
			border.MiddleLeft + border.Bottom)
		// Remove footer if width is too low
		if lipgloss.Width(vpFooter) > vp.Width {
			vpFooter = ""
		}
	}

	// Manually construct the bottom line of the border with the scroll percentage inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	vpFooterBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		borderStyle.Render(border.BottomLeft),
		borderStyle.
			Render(strings.Repeat(border.Top, max(0, vp.Width-lipgloss.
				Width(vpFooter)+FocusedBorderStyle.GetHorizontalPadding()))),
		vpFooter,
		borderStyle.Render(border.BottomRight),
	)

	// Render the viewport content inside a box that has NO top and bottom border.
	vpBody := FocusedBorderStyle.BorderTop(false).BorderBottom(false).Render(vp.View())

	// Join the title bar and the main content vertically.
	return lipgloss.JoinVertical(lipgloss.Left, vpTitleBar, vpBody, vpFooterBar)
}

// Returns the footer string
func CreateFooter(m *model) string {
	helpText := m.selectedPort + " | ↑/↓: cmds · PgUp/PgDn: scroll · ctrl+e: open editor"
	if m.cmdhist.GetIndex() != m.cmdhist.GetHistLen() {
		helpText += " · ctrl+d: del"
	}

	var connectionSymbol string

	switch m.conStatus {
	case conStatus_connected:
		connectionSymbol = fmt.Sprintf(" %s ", ConnectSymbolStyle.Render("●"))
		helpText += " · ctrl+x: disconnect"

	case conStatus_disconnected:
		connectionSymbol = fmt.Sprintf(" %s ", DisconnectedSymbolStyle.Render("●"))
		helpText += " · ctrl+x: connect"

	case conStatus_connecting:
		connectionSymbol = fmt.Sprintf(" %s", m.spinner.View())
		helpText += " · ctrl+x: disconnect"
	}
	connectionSymbol = zone.Mark("consymbol", connectionSymbol)

	return lipgloss.NewStyle().MaxWidth(m.inputTa.Width()). // TODO check width
								Render(connectionSymbol + FooterStyle.Render(helpText))
}

func HandleNewWindowSize(m *model, msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	borderWidth, borderHight := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).GetFrameSize()

	m.serialVp.Width = m.width / 4 * 3
	m.cmdhist.Vp.Width = m.width - m.serialVp.Width

	m.serialVp.Width -= borderHight
	m.cmdhist.Vp.Width -= borderWidth

	const footerHight = 1
	m.serialVp.Height = m.height - lipgloss.Height(m.inputTa.View()) - borderHight - footerHight
	m.cmdhist.Vp.Height = m.serialVp.Height

	m.inputTa.SetWidth(m.width)

	// log.Printf("margin v, h:     %v, %v\n", borderHight, borderWidth)
	// log.Printf("serial vp  w, h: %v, %v\n", m.serialVp.Width, m.serialVp.Height)
	// log.Printf("cmd vp w, h:     %v, %v\n", m.cmdhist.Vp.Width, m.cmdhist.Vp.Height)
	// log.Printf("input ta w, h:   %v, %v\n", m.inputTa.Width(), lipgloss.Height(m.inputTa.View()))

	updateVp(&m.serialVp, &m.serMsg, false, true)
	m.cmdhist.ResetVp()
}

func updateVp(vp *viewport.Model, content *[]string, updateWidth bool, gotoBottom bool) {
	// log.Printf("reset vp: vp height, msg len:   %v, %v\n", vp.Height, len(*content))

	if vp.Height > 0 && len(*content) > 0 {
		if updateWidth {
			vp.SetContent(lipgloss.NewStyle().Width(vp.Width).
				Render(strings.Join(*content, "\n")))
		} else {
			vp.SetContent(lipgloss.NewStyle().Render(strings.Join(*content, "\n")))
		}
		if gotoBottom {
			vp.GotoBottom()
		}
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
	if m.showEscapes {
		// also print escape characters
		line.WriteString(fmt.Sprintf("%q", msg))
	} else {
		// ignore escape characters
		line.WriteString(stringsx.Clean(msg))
	}

	// TODO set serial message histrory limit, remove oldest if exceed
	m.serMsg = append(m.serMsg, line.String())

	// Add command to serial logger, if logger is active
	if m.serialLog != nil {
		m.serialLog.Println(line.String())
	}

	// reset viewport only if we did not scrolled up in msg history
	goToBottom := m.serialVp.ScrollPercent() == 1
	updateVp(&m.serialVp, &m.serMsg, false, goToBottom)

	// restart msg scanner
	return readFromPort(m.scanner)
}

// A serial messages was successfully sent to the serial port.
// So we log the serial message to the message view and the command view.
func HandleSerialTxMsg(m *model, msg string) {
	// Reset input text area.
	m.inputTa.Reset()

	// Log the sent message to the viewport
	var line strings.Builder
	if m.showTimestamp {
		t := time.Now().Format("15:04:05.000")
		line.WriteString(fmt.Sprintf("[%s] ", t))
	}
	// line.WriteString("> ")
	line.WriteString(msg)

	// Add command to serial logger, if logger is active
	if m.serialLog != nil {
		m.serialLog.Println(line.String())
	}

	// TODO set serial message histrory limit, remove oldest if exceed
	m.serMsg = append(m.serMsg, VpTxMsgStyle.Render(line.String())) // TODO directly use style for var()

	// reset viewport only if we did not scrolled up in msg history
	goToBottom := m.serialVp.ScrollPercent() == 1
	updateVp(&m.serialVp, &m.serMsg, false, goToBottom)
}

// Handle serial port errors.
// If serial port was closed for any reason, start trying to reconnect to the port
// and start the reconnect spinner symbol.
func HandleSerialPortErr(m *model, msg *serial.PortError) (tea.Cmd, tea.Cmd) {
	if msg.Code() == serial.PortClosed {
		if m.conStatus != conStatus_disconnected {
			return PrepareReconnect(m)
		}
	}
	return nil, nil
}

// Prepare TUI to reconnect
func PrepareReconnect(m *model) (tea.Cmd, tea.Cmd) {
	m.inputTa.Reset()
	m.inputTa.Blur()
	m.conStatus = conStatus_connecting
	(*m.port).Close()
	m.inputTa.Placeholder = "Reconnecting..."
	reconnectCmd := reconnectToPort(m.selectedPort, m.selectedMode)
	spinnerCmd := m.spinner.Tick
	return reconnectCmd, spinnerCmd
}

// Handle port reconnected event.
func HandlePortReconnect(m *model, port Port) (tea.Cmd, tea.Cmd) {
	log.Println("Successfully reconnected to port " + m.selectedPort)
	m.inputTa.Placeholder = "Send a message..." // TODO remove duplicated code
	cursorBlinkCmd := m.inputTa.Focus()
	m.conStatus = conStatus_connected
	*m.port = port
	m.scanner = bufio.NewScanner(*m.port)
	readCmd := readFromPort(m.scanner)

	return cursorBlinkCmd, readCmd
}

func RunTui(port *io.ReadWriteCloser, mode serial.Mode, flags Flags, config Config, serialLog *log.Logger) {
	zone.NewGlobal()

	// log.Printf("Cmd history on startup %v\n", config.CmdHistoryLines)
	m := initialModel(port, flags.Timestamp, config.CmdHistoryLines, flags.Port, &mode, serialLog, flags.ShowEscapes)

	for {
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		finalModel, err := p.Run()
		if err != nil {
			log.Fatal(err)
		}

		var ok bool
		m, ok = finalModel.(model)
		if !ok {
			log.Fatal("Could not cast final model to model type")
		}

		if !m.restartApp {
			break
		}
		m.restartApp = false
	}
}
