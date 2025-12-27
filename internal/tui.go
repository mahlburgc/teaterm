package internal

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/cmdhist"
	"github.com/mahlburgc/teaterm/internal/input"
	"github.com/mahlburgc/teaterm/internal/msglog"
	"github.com/mahlburgc/teaterm/internal/styles"
	"go.bug.st/serial"
)

type StartNextReconnectTryMsg bool

const (
	conStatus_disconnected = iota
	conStatus_connecting   = iota
	conStatus_connected    = iota
)

type model struct {
	msglog       msglog.Model
	cmdhist      cmdhist.Model
	input        input.Model
	err          error
	port         *io.ReadWriteCloser
	scanner      *bufio.Scanner
	selectedPort string
	selectedMode *serial.Mode
	restartApp   bool
	width        int
	height       int
	conStatus    int
	spinner      spinner.Model
}

func initialModel(port *io.ReadWriteCloser, showTimestamp bool, cmdHist []string,
	selectedPort string, selectedMode *serial.Mode, serialLog *log.Logger, showEscapes bool,
) model {
	input := input.New()
	cmdhist := cmdhist.New(cmdHist)
	msglog := msglog.New(showTimestamp, showEscapes, styles.VpTxMsgStyle, serialLog)

	// Spinner symbol runs during port reconnect.
	reconnectSpinner := spinner.New()
	reconnectSpinner.Spinner = spinner.Dot
	reconnectSpinner.Style = styles.SpinnerStyle

	// Scanner searches for incomming serial messages
	scanner := bufio.NewScanner(*port)

	return model{
		msglog:       msglog,
		cmdhist:      cmdhist,
		input:        input,
		err:          nil,
		port:         port,
		scanner:      scanner,
		selectedPort: selectedPort,
		selectedMode: selectedMode,
		width:        0,
		height:       0,
		conStatus:    conStatus_connected,
		spinner:      reconnectSpinner,
		restartApp:   false,
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

	m.msglog, cmd = m.msglog.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		HandleNewWindowSize(&m, msg)

	case tea.KeyMsg:
		cmds = append(cmds, HandleKeys(&m, msg))

	case events.SerialRxMsg:
		cmds = append(cmds, readFromPort(m.scanner))

	case input.CmdExecuted:
		cmds = append(cmds, SendToPort(*m.port, msg.Cmd))

	case PortReconnectStatusMsg:
		if msg.ok {
			cmd1, cmd2 := HandlePortReconnect(&m, msg.port)
			cmds = append(cmds, tea.Batch(cmd1, cmd2))
		} else {
			cmd := func() tea.Msg {
				time.Sleep(1 * time.Second)
				return StartNextReconnectTryMsg(true)
			}
			cmds = append(cmds, cmd)

		}

	case StartNextReconnectTryMsg:
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
			(*m.port).Close()
			m.input.SetDisconnectet()
		}

	case spinner.TickMsg:
		if m.conStatus == conStatus_connecting {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case events.HistCmdExecuted:
		cmds = append(cmds, SendToPort(*m.port, string(msg)))

	case msglog.EditorFinishedMsg:
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

	footer := CreateFooter(&m)

	serialVpFooter := fmt.Sprintf("%v, %3.f%%", m.msglog.GetLen(), m.msglog.GetScrollPercent())
	serialVp := styles.AddBorder(m.msglog.Vp, "Messages", serialVpFooter)

	cmdVp := styles.AddBorder(m.cmdhist.Vp, "Commands", "")

	// Arrange viewports side by side
	viewports := lipgloss.JoinHorizontal(
		lipgloss.Top,
		serialVp,
		cmdVp,
	)

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		viewports,
		m.input.View(),
		footer,
	)

	return zone.Scan(lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		screen))
}

func HandleKeys(m *model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+x":
		return func() tea.Msg {
			return PortManualConnectMsg(true)
		}
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		StoreConfig(m.cmdhist.GetCmdHist())
		return tea.Quit
	}
	return nil
}

// Returns the footer string
func CreateFooter(m *model) string {
	helpText := m.selectedPort + " | ↑/↓: cmds · PgUp/PgDn: scroll · ctrl+e: open editor"
	if m.cmdhist.GetIndex() != m.cmdhist.GetHistLen() {
		helpText += " · ctrl+d: del"
	}
	if m.msglog.Vp.Height > 0 {
		helpText += " · ctrl+l: clear"
	}

	var connectionSymbol string

	switch m.conStatus {
	case conStatus_connected:
		connectionSymbol = fmt.Sprintf(" %s ", styles.ConnectSymbolStyle.Render("●"))
		helpText += " · ctrl+x: disconnect"

	case conStatus_disconnected:
		connectionSymbol = fmt.Sprintf(" %s ", styles.DisconnectedSymbolStyle.Render("●"))
		helpText += " · ctrl+x: connect"

	case conStatus_connecting:
		connectionSymbol = fmt.Sprintf(" %s", m.spinner.View())
		helpText += " · ctrl+x: disconnect"
	}
	connectionSymbol = zone.Mark("consymbol", connectionSymbol)

	return lipgloss.NewStyle().MaxWidth(m.input.Ta.Width()). // TODO check width
									Render(connectionSymbol + styles.FooterStyle.Render(helpText))
}

func HandleNewWindowSize(m *model, msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	borderWidth, borderHight := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).GetFrameSize()

	m.msglog.Vp.Width = m.width / 4 * 3
	m.cmdhist.Vp.Width = m.width - m.msglog.Vp.Width

	m.msglog.Vp.Width -= borderHight
	m.cmdhist.Vp.Width -= borderWidth

	const footerHight = 1
	m.msglog.Vp.Height = m.height - lipgloss.Height(m.input.View()) - borderHight - footerHight
	m.cmdhist.Vp.Height = m.msglog.Vp.Height

	m.input.Ta.SetWidth(m.width)

	// log.Printf("margin v, h:     %v, %v\n", borderHight, borderWidth)
	// log.Printf("serial vp  w, h: %v, %v\n", m.serialVp.Width, m.serialVp.Height)
	// log.Printf("cmd vp w, h:     %v, %v\n", m.cmdhist.Vp.Width, m.cmdhist.Vp.Height)
	// log.Printf("input ta w, h:   %v, %v\n", m.inputTa.Width(), lipgloss.Height(m.inputTa.View()))

	m.msglog.UpdateVp()
	m.cmdhist.ResetVp()
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
	m.input.SetReconnecting()
	m.conStatus = conStatus_connecting
	(*m.port).Close()
	reconnectCmd := reconnectToPort(m.selectedPort, m.selectedMode)
	spinnerCmd := m.spinner.Tick
	return reconnectCmd, spinnerCmd
}

// Handle port reconnected event.
func HandlePortReconnect(m *model, port Port) (tea.Cmd, tea.Cmd) {
	log.Println("Successfully reconnected to port " + m.selectedPort)
	m.input.Ta.Placeholder = "Send a message..." // TODO remove duplicated code
	cursorBlinkCmd := m.input.Ta.Focus()
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
