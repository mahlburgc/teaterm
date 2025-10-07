package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mahlburgc/teaterm/internal"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"go.bug.st/serial"
)

type (
	serialMsg        string
	errMsg           error
	portConnectedMsg serial.Port
)

type model struct {
	serialVp      viewport.Model
	cmdVp         viewport.Model
	serMessages   []string
	inputTa       textarea.Model
	err           error
	port          serial.Port
	scanner       *bufio.Scanner
	selectedPort  string
	selectedMode  *serial.Mode
	showTimestamp bool
	cmdHist       []string
	cmdHistFile   string
	cmdHistIndex  int
	width         int
	height        int
	conStatus     bool
	spinner       spinner.Model
}

func initialModel(port serial.Port, showTimestamp bool, cmdHist []string,
	cmdHistFile string, selectedPort string, selectedMode *serial.Mode,
) model {
	// Command text area contains text field to send commands to the serial port
	inputTa := textarea.New()
	inputTa.SetWidth(30)
	inputTa.SetHeight(1)
	inputTa.Placeholder = "Send a message..."
	inputTa.Focus()
	inputTa.Prompt = "> "
	inputTa.CharLimit = 256
	inputTa.ShowLineNumbers = false
	inputTa.KeyMap.InsertNewline.SetEnabled(false)
	inputTa.Cursor.Style = internal.CursorStyle
	inputTa.FocusedStyle.CursorLine = lipgloss.NewStyle()
	inputTa.FocusedStyle.Placeholder = internal.FocusedPlaceholderStyle
	inputTa.FocusedStyle.Prompt = internal.FocusedPromtStyle
	inputTa.BlurredStyle.Prompt = internal.BlurredPromtStyle
	inputTa.FocusedStyle.Base = internal.FocusedBorderStyle
	inputTa.BlurredStyle.Base = internal.BlurredBorderStyle

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
	reconnectSpinner.Style = internal.SpinnerStyle

	// Scanner searches for incomming serial messages
	scanner := bufio.NewScanner(port)

	return model{
		inputTa:       inputTa,
		serMessages:   []string{},
		serialVp:      serialVp,
		cmdVp:         cmdVp,
		err:           nil,
		port:          port,
		scanner:       scanner,
		selectedPort:  selectedPort,
		selectedMode:  selectedMode,
		showTimestamp: showTimestamp,
		cmdHist:       cmdHist,
		cmdHistIndex:  len(cmdHist),
		cmdHistFile:   cmdHistFile,
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
	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		// avoid logging on cursor blink
	default:
		log.Printf("Update Msg: Type: %T Value: %v\n", msg, msg)
	}

	// tea commands that should be executed after Update call
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.inputTa, cmd = m.inputTa.Update(msg)
	cmds = append(cmds, cmd)
	m.serialVp, cmd = m.serialVp.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		screenWidth := msg.Width - 2
		screenHight := msg.Height - 3

		// Calculate the vertical and horizontal space taken by the border.
		verticalMargin, horizontalMargin := internal.FocusedBorderStyle.GetFrameSize()

		// The window has been resized, so update the viewport's dimensions.
		m.serialVp.Width = screenWidth/4*3 - horizontalMargin
		m.serialVp.Height = screenHight - m.inputTa.Height() - verticalMargin

		// m.serialVp.Width = screenWidth / 4 * 3
		m.cmdVp.Width = screenWidth - m.serialVp.Width - horizontalMargin
		m.cmdVp.Height = m.serialVp.Height

		m.inputTa.SetWidth(m.width)

		// m.serialVp.Height = screenHight - m.inputTa.Height()

		if len(m.serMessages) > 0 {
			m.serialVp.SetContent(lipgloss.NewStyle().Width(m.serialVp.Width).Render(strings.Join(m.serMessages, "\n")))
		}

		if m.serialVp.Height > 0 {
			m.serialVp.GotoBottom()
		}

		historyContent := strings.Join(m.cmdHist, "\n")
		m.cmdVp.SetContent(lipgloss.NewStyle().Width(m.cmdVp.Width).Render(historyContent))

		if m.serialVp.Height > 0 {
			m.cmdVp.GotoBottom()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "alt+m":
			// nothing to do for now
		}

		// TODO use keymap
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// save last x elemets of current command history for next session
			const maxStoredCmds = 100
			if len(m.cmdHist) > maxStoredCmds {
				start := len(m.cmdHist) - maxStoredCmds
				m.cmdHist = m.cmdHist[start:]
			}

			fileContent := strings.Join(m.cmdHist, "\n") + "\n"

			err := os.WriteFile(m.cmdHistFile, []byte(fileContent), 0o644)
			if err != nil {
				log.Fatal(err)
			}
			return m, tea.Quit
		case tea.KeyPgUp:
			m.serialVp.ScrollUp(3)
			return m, nil
		case tea.KeyPgDown:
			m.serialVp.ScrollDown(3)
			return m, nil
		case tea.KeyCtrlD:
			if m.cmdHistIndex != len(m.cmdHist) {
				// delete cmd from command history
				m.cmdHist = append(m.cmdHist[:m.cmdHistIndex], m.cmdHist[m.cmdHistIndex+1:]...)
				m.cmdHistIndex = len(m.cmdHist) // TODO create style for cmd history, remove duplicated code
				m.cmdVp.SetContent(lipgloss.NewStyle().Width(m.cmdVp.Width).Render(strings.Join(m.cmdHist, "\n")))
			}

		case tea.KeyUp:
			if m.cmdHistIndex > 0 {
				m.cmdHistIndex--
				m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
				m.inputTa.SetCursor(len(m.inputTa.Value()))

				var cmdHistLines []string // TODO remove duplicated code
				for i, cmd := range m.cmdHist {
					if i == m.cmdHistIndex {
						cmdHistLines = append(cmdHistLines, internal.SelectedCmdStyle.Render("> "+cmd))
					} else {
						cmdHistLines = append(cmdHistLines, cmd)
					}
				}

				if len(m.cmdHist) > m.cmdVp.Height {
					if m.cmdHistIndex <= len(m.cmdHist)-m.cmdVp.Height+1 {
						cmdHistLines = cmdHistLines[m.cmdHistIndex : m.cmdHistIndex+m.cmdVp.Height-2]
					}
				}

				cmdHistContent := strings.Join(cmdHistLines, "\n")
				m.cmdVp.SetContent(lipgloss.NewStyle().Width(m.cmdVp.Width).Render(cmdHistContent))
				m.cmdVp.GotoBottom()
			}

		case tea.KeyDown:
			if m.cmdHistIndex < len(m.cmdHist) {
				m.cmdHistIndex++
				if m.cmdHistIndex < len(m.cmdHist) {
					m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
					m.inputTa.SetCursor(len(m.inputTa.Value()))

					var cmdHistLines []string // TODO remove duplicated code
					for i, cmd := range m.cmdHist {
						if i == m.cmdHistIndex {
							cmdHistLines = append(cmdHistLines, internal.SelectedCmdStyle.Render("> "+cmd))
						} else {
							cmdHistLines = append(cmdHistLines, cmd)
						}
					}

					// TODO remove duplicated code
					if len(m.cmdHist) > m.cmdVp.Height {
						if m.cmdHistIndex <= len(m.cmdHist)-m.cmdVp.Height+1 {
							cmdHistLines = cmdHistLines[m.cmdHistIndex : m.cmdHistIndex+m.cmdVp.Height-2]
						}
					}

					cmdHistContent := strings.Join(cmdHistLines, "\n")
					m.cmdVp.SetContent(lipgloss.NewStyle().Width(m.cmdVp.Width).Render(cmdHistContent))
					m.cmdVp.GotoBottom()

				} else {
					// Cleared history, reset to empty
					m.inputTa.Reset()
					m.cmdVp.SetContent(lipgloss.NewStyle().Width(m.cmdVp.Width).Render(strings.Join(m.cmdHist, "\n")))
				}
			}
		case tea.KeyEnter:
			userInput := m.inputTa.Value()
			if userInput == "" {
				return m, nil
			}

			// Add command to history
			// if command is already found in the command histroy, just move command to end to avoid
			// duplicated commands in command history
			foundIndex := -1
			for i, cmd := range m.cmdHist {
				if cmd == userInput {
					foundIndex = i
					break
				}
			}

			if foundIndex != -1 {
				m.cmdHist = append(m.cmdHist[:foundIndex], m.cmdHist[foundIndex+1:]...)
				m.cmdHist = append(m.cmdHist, userInput)
			} else {
				m.cmdHist = append(m.cmdHist, userInput)
			}

			// Send command from ta to serial port -> TODO should not be done in update routine
			stringToSend := userInput + "\r\n" // TODO add custom Lineending
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
			// line.WriteString("> ")
			line.WriteString(userInput)

			// TODO set serial message histrory limit, remove oldest if exceed
			m.serMessages = append(m.serMessages, internal.VpTxMsgStyle.Render(line.String())) // TODO directly use style for var()
			m.serialVp.SetContent(strings.Join(m.serMessages, "\n"))
			m.serialVp.GotoBottom()
			m.inputTa.Reset()

			// Update command history viewport after sending a command
			// TODO create method and use also in window size message
			historyContent := strings.Join(m.cmdHist, "\n")
			m.cmdVp.SetContent(lipgloss.NewStyle().Width(m.cmdVp.Width).Render(historyContent))
			m.cmdVp.GotoBottom()
			m.cmdHistIndex = len(m.cmdHist)
		}

	case serialMsg:
		cmd = readFromPort(m.scanner)
		cmds = append(cmds, cmd)
		var line strings.Builder
		if m.showTimestamp {
			t := time.Now().Format("15:04:05.000")
			line.WriteString(fmt.Sprintf("[%s] ", t))
		}
		//line.WriteString("< ")
		line.WriteString(string(msg))

		// TODO set serial message histrory limit, remove oldest if exceed
		m.serMessages = append(m.serMessages, line.String())
		m.serialVp.SetContent(lipgloss.NewStyle().Width(m.serialVp.Width).Render(strings.Join(m.serMessages, "\n")))
		m.serialVp.GotoBottom()

	case *serial.PortError:
		switch msg.Code() {
		case serial.PortClosed:
			m.inputTa.Reset()
			m.inputTa.Blur()
			m.conStatus = false
			m.port.Close()
			m.inputTa.Placeholder = "Reconnecting..."
			cmd = reconnectPort(m.selectedPort, m.selectedMode)
			cmds = append(cmds, cmd)
			cmd = m.spinner.Tick
			cmds = append(cmds, cmd)
		}

	case portConnectedMsg:
		m.inputTa.Placeholder = "Send a message..." // TODO remove duplicated code
		cmd = m.inputTa.Focus()
		cmds = append(cmds, cmd)
		m.conStatus = true
		m.port = msg
		m.scanner = bufio.NewScanner(m.port)
		cmd = readFromPort(m.scanner)
		cmds = append(cmds, cmd)

	case errMsg:
		m.err = error(msg)

	case spinner.TickMsg:
		if !m.conStatus {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var connectionText string
	connectSymbol := internal.ConnectSymbolStyle.Render("●")
	if m.conStatus {
		connectionText = fmt.Sprintf(" %s ", connectSymbol)
	} else {
		connectionText = fmt.Sprintf(" %s", m.spinner.View())
	}

	helpText := m.selectedPort + " | ↑/↓: cmds · PgUp/PgDn: scroll"
	if m.cmdHistIndex != len(m.cmdHist) {
		helpText += " · ctrl+d: del"
	}

	footer := lipgloss.NewStyle().MaxWidth(m.inputTa.Width()).
		Render(connectionText + internal.FooterStyle.Render(helpText))

	border := internal.FocusedBorderStyle.GetBorderStyle()

	serialVpTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
		Render(border.Top + border.MiddleRight + " Messages " + border.MiddleLeft)
	if lipgloss.Width(serialVpTitle) > m.serialVp.Width {
		serialVpTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("")
	}

	histVpTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
		Render(border.Top + border.MiddleRight + " Commands " + border.MiddleLeft)
	if lipgloss.Width(histVpTitle) > m.cmdVp.Width {
		histVpTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("")
	}

	// 3. Manually construct the top line of the border with the title inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	serialVpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
			Render(border.TopLeft),
		serialVpTitle,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
			Render(strings.Repeat(border.Top, max(0, m.serialVp.Width-lipgloss.
				Width(serialVpTitle)+internal.FocusedBorderStyle.GetHorizontalPadding()))),
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopRight),
	)

	histVpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopLeft),
		histVpTitle,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).
			Render(strings.Repeat(border.Top, max(0, m.cmdVp.Width-lipgloss.
				Width(histVpTitle)+internal.FocusedBorderStyle.GetHorizontalPadding()))),
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopRight),
	)

	// 4. Render the viewport content inside a box that has NO top border.
	serialVpContent := internal.FocusedBorderStyle.Copy().
		BorderTop(false).
		Render(m.serialVp.View())

	histVpContent := internal.FocusedBorderStyle.Copy().
		BorderTop(false).
		Render(m.cmdVp.View())

	// 5. Join the title bar and the main content vertically.
	serialViewportString := lipgloss.JoinVertical(lipgloss.Left, serialVpTitleBar, serialVpContent)

	histViewportString := lipgloss.JoinVertical(lipgloss.Left, histVpTitleBar, histVpContent)

	// Arrange viewports side by side
	viewports := lipgloss.JoinHorizontal(
		lipgloss.Top,
		serialViewportString,
		histViewportString,
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

// try to reconnect to the serial port we connected on startup
func reconnectPort(selectedPort string, selectedMode *serial.Mode) tea.Cmd {
	return func() tea.Msg {
		var port serial.Port
		var err error

		for {
			port, err = serial.Open(selectedPort, selectedMode)
			if err != nil {
				time.Sleep(1000)
			} else {
				break
			}
		}
		return portConnectedMsg(port)
	}
}

func readFromPort(scanner *bufio.Scanner) tea.Cmd {
	return func() tea.Msg {
		for scanner.Scan() {
			line := scanner.Text()
			return serialMsg(line)
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF && err != context.Canceled {
				return errMsg(err)
			}
		}
		return nil
	}
}

func main() {
	config := internal.GetConfig()
	flags := internal.GetFlags()

	if flags.List {
		internal.ListPorts()
		return
	}

	port, mode := internal.OpenPort(flags.Port)
	defer port.Close()

	m := initialModel(port, flags.Timestamp, config.CmdHistoryLines, config.CmdHistFile, flags.Port, &mode)
	p := tea.NewProgram(m, tea.WithAltScreen())

	logger := internal.StartLogger("teaterm_debug.log")
	if logger != nil {
		defer logger.Close()
	}

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Command history saved under %s!\n", config.CmdHistFile)
}
