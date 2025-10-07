package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davecgh/go-spew/spew"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

type serialMsg string
type errMsg error
type portConnectedMsg serial.Port

var (
	cursorStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	connectSymbolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("77"))
	focusedPlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) //99
	focusedBorderStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("242"))
	selectedCmdStyle        = cursorStyle
	spinnerStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	vpTxMsgStyle            = cursorStyle
	footerStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	focusedPromtStyle       = cursorStyle
	blurredPromtStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	//selectedCmdStyle      = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("230"))
)

type model struct {
	ready         bool
	dump          io.Writer
	serialVp      viewport.Model
	histVp        viewport.Model
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

func initialModel(port serial.Port, showTimestamp bool, cmdHist []string, cmdHistFile string, dump io.Writer, selectedPort string, selectedMode *serial.Mode) model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 256
	ta.Cursor.Style = cursorStyle
	ta.FocusedStyle.Placeholder = focusedPlaceholderStyle
	ta.FocusedStyle.Prompt = focusedPromtStyle
	ta.BlurredStyle.Prompt = blurredPromtStyle
	ta.FocusedStyle.Base = focusedBorderStyle
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // Remove cursor line styling
	ta.BlurredStyle.Base = focusedBorderStyle
	ta.ShowLineNumbers = false
	ta.SetWidth(30)
	ta.SetHeight(1)

	// We want to create a viewport without border and later manually add the border to inject
	// a title into the border
	// Calculate the vertical and horizontal space taken by the border.
	serialVp := viewport.New(30, 5)
	serialVp.SetContent(`Welcome to teaterm!`)
	serialVp.Style = lipgloss.NewStyle()

	histVp := viewport.New(30, 5)
	histVp.Style = lipgloss.NewStyle()

	// Disable the viewport's default up/down key handling so it doesn't scroll
	// when we are navigating command history.
	serialVp.KeyMap.Up.SetEnabled(false)
	serialVp.KeyMap.Down.SetEnabled(false)

	ta.KeyMap.InsertNewline.SetEnabled(false) //TODO check
	serialVp.KeyMap.PageUp.SetEnabled(false)
	serialVp.KeyMap.PageDown.SetEnabled(false)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	scanner := bufio.NewScanner(port)

	return model{
		ready:         false,
		dump:          dump,
		inputTa:       ta,
		serMessages:   []string{},
		serialVp:      serialVp,
		histVp:        histVp,
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
		spinner:       s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, readFromPort(m.scanner))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// switch msg := msg.(type) {
	// case cursor.BlinkMsg:
	// 	//avoid print cursor blink
	// default:
	// 	spew.Fdump(m.dump, msg)
	// }

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
		verticalMargin, horizontalMargin := focusedBorderStyle.GetFrameSize()

		// The window has been resized, so update the viewport's dimensions.
		m.serialVp.Width = screenWidth/4*3 - horizontalMargin
		m.serialVp.Height = screenHight - m.inputTa.Height() - verticalMargin

		//m.serialVp.Width = screenWidth / 4 * 3
		m.histVp.Width = screenWidth - m.serialVp.Width - horizontalMargin
		m.histVp.Height = m.serialVp.Height

		m.inputTa.SetWidth(m.width)

		//m.serialVp.Height = screenHight - m.inputTa.Height()

		if len(m.serMessages) > 0 {
			m.serialVp.SetContent(lipgloss.NewStyle().Width(m.serialVp.Width).Render(strings.Join(m.serMessages, "\n")))
		}

		if m.serialVp.Height > 0 {
			m.serialVp.GotoBottom()
		}

		historyContent := strings.Join(m.cmdHist, "\n")
		m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(historyContent))

		if m.serialVp.Height > 0 {
			m.histVp.GotoBottom()
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

			err := os.WriteFile(m.cmdHistFile, []byte(fileContent), 0644)
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
				m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(strings.Join(m.cmdHist, "\n")))
			}

		case tea.KeyUp:
			if m.cmdHistIndex > 0 {
				m.cmdHistIndex--
				m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
				m.inputTa.SetCursor(len(m.inputTa.Value()))

				var cmdHistLines []string //TODO remove duplicated code
				for i, cmd := range m.cmdHist {
					if i == m.cmdHistIndex {
						cmdHistLines = append(cmdHistLines, selectedCmdStyle.Render("> "+cmd))
					} else {
						cmdHistLines = append(cmdHistLines, cmd)
					}
				}
				spew.Fdump(m.dump, "cmd history len:", len(m.cmdHist))
				spew.Fdump(m.dump, "history hight:", m.histVp.Height)
				spew.Fdump(m.dump, "command index:", m.cmdHistIndex)
				spew.Fdump(m.dump, "diff:", len(m.cmdHist)-m.histVp.Height)
				if len(m.cmdHist) > m.histVp.Height {
					spew.Fdump(m.dump, "here we are")
					if m.cmdHistIndex <= len(m.cmdHist)-m.histVp.Height+1 {
						spew.Fdump(m.dump, "now we are outside")
						cmdHistLines = cmdHistLines[m.cmdHistIndex : m.cmdHistIndex+m.histVp.Height-2]
						spew.Fdump(m.dump, cmdHistLines)
					}
				}

				cmdHistContent := strings.Join(cmdHistLines, "\n")
				m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(cmdHistContent))
				m.histVp.GotoBottom()
			}

		case tea.KeyDown:
			if m.cmdHistIndex < len(m.cmdHist) {
				m.cmdHistIndex++
				if m.cmdHistIndex < len(m.cmdHist) {
					m.inputTa.SetValue(m.cmdHist[m.cmdHistIndex])
					m.inputTa.SetCursor(len(m.inputTa.Value()))

					var cmdHistLines []string //TODO remove duplicated code
					for i, cmd := range m.cmdHist {
						if i == m.cmdHistIndex {
							cmdHistLines = append(cmdHistLines, selectedCmdStyle.Render("> "+cmd))
						} else {
							cmdHistLines = append(cmdHistLines, cmd)
						}
					}

					//TODO remove duplicated code
					spew.Fdump(m.dump, "cmd history len:", len(m.cmdHist))
					spew.Fdump(m.dump, "history hight:", m.histVp.Height)
					spew.Fdump(m.dump, "command index:", m.cmdHistIndex)
					spew.Fdump(m.dump, "diff:", len(m.cmdHist)-m.histVp.Height)
					if len(m.cmdHist) > m.histVp.Height {
						spew.Fdump(m.dump, "here we are")
						if m.cmdHistIndex <= len(m.cmdHist)-m.histVp.Height+1 {
							spew.Fdump(m.dump, "now we are outside")
							cmdHistLines = cmdHistLines[m.cmdHistIndex : m.cmdHistIndex+m.histVp.Height-2]
							spew.Fdump(m.dump, cmdHistLines)
						}
					}

					cmdHistContent := strings.Join(cmdHistLines, "\n")
					m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(cmdHistContent))
					m.histVp.GotoBottom()

				} else {
					// Cleared history, reset to empty
					m.inputTa.Reset()
					m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(strings.Join(m.cmdHist, "\n")))
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
				spew.Fdump(m.dump, "%v, %s, %s\n", i, cmd, userInput)
				if cmd == userInput {
					spew.Sdump("found")
					foundIndex = i
					break
				}
			}

			if foundIndex != -1 {
				spew.Fdump(m.dump, m.cmdHist)
				m.cmdHist = append(m.cmdHist[:foundIndex], m.cmdHist[foundIndex+1:]...)
				spew.Fdump(m.dump, m.cmdHist)
				m.cmdHist = append(m.cmdHist, userInput)
			} else {
				m.cmdHist = append(m.cmdHist, userInput)
			}

			// Send command from ta to serial port -> TODO should not be done in update routine
			stringToSend := userInput + "\r\n" //TODO add custom Lineending
			_, err := m.port.Write([]byte(stringToSend))
			if err != nil {
				spew.Fdump(m.dump, "write error:", err)
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
			m.serMessages = append(m.serMessages, vpTxMsgStyle.Render(line.String())) // TODO directly use style for var()
			m.serialVp.SetContent(strings.Join(m.serMessages, "\n"))
			m.serialVp.GotoBottom()
			m.inputTa.Reset()

			// Update command history viewport after sending a command
			// TODO create method and use also in window size message
			historyContent := strings.Join(m.cmdHist, "\n")
			spew.Fdump(m.dump, "history list", m.cmdHist)
			spew.Fdump(m.dump, "history content", historyContent)
			m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(historyContent))
			m.histVp.GotoBottom()
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
		m.inputTa.Placeholder = "Send a message..." //TODO remove duplicated code
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
	connectSymbol := connectSymbolStyle.Render("●")
	//connectSymbol := connectSymbolStyle.Render("⇄")
	if m.conStatus {
		connectionText = fmt.Sprintf(" %s ", connectSymbol)
	} else {
		connectionText = fmt.Sprintf(" %s", m.spinner.View())
	}

	helpText := m.selectedPort + " | ↑/↓: cmds · PgUp/PgDn: scroll"
	if m.cmdHistIndex != len(m.cmdHist) {
		helpText += " · ctrl+d: del"
	}

	footer := lipgloss.NewStyle().MaxWidth(m.inputTa.Width()).Render(connectionText + footerStyle.Render(helpText))

	border := focusedBorderStyle.GetBorderStyle()

	serialVpTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.Top + border.MiddleRight + " Messages " + border.MiddleLeft)
	if lipgloss.Width(serialVpTitle) > m.serialVp.Width {
		serialVpTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("")
	}

	histVpTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.Top + border.MiddleRight + " Commands " + border.MiddleLeft)
	if lipgloss.Width(histVpTitle) > m.histVp.Width {
		histVpTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("")
	}

	// 3. Manually construct the top line of the border with the title inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	serialVpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopLeft),
		serialVpTitle,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(strings.Repeat(border.Top, max(0, m.serialVp.Width-lipgloss.Width(serialVpTitle)+focusedBorderStyle.GetHorizontalPadding()))),
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopRight),
	)

	histVpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopLeft),
		histVpTitle,
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(strings.Repeat(border.Top, max(0, m.histVp.Width-lipgloss.Width(histVpTitle)+focusedBorderStyle.GetHorizontalPadding()))),
		lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(border.TopRight),
	)

	// 4. Render the viewport content inside a box that has NO top border.
	serialVpContent := focusedBorderStyle.Copy().
		BorderTop(false).
		Render(m.serialVp.View())

	histVpContent := focusedBorderStyle.Copy().
		BorderTop(false).
		Render(m.histVp.View())

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

	var dump *os.File
	if _, ok := os.LookupEnv("DEBUG"); ok {
		var err error
		dump, err = os.OpenFile("messages.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(1)
		}
	}

	listPtr := flag.Bool("l", false, "list available ports")
	portPtr := flag.String("p", "/dev/ttyUSB0", "serial port")
	timestampPtr := flag.Bool("t", false, "show timestamp")

	flag.Parse()

	listFlag := *listPtr
	showTimestamp := *timestampPtr

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	commandHistoryFilePath := homedir + "/.config/teaterm/"
	commandHistoryFileName := "cmdhistroy.conf"
	commandHistoryFile := commandHistoryFilePath + commandHistoryFileName

	err = os.MkdirAll(commandHistoryFilePath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	cmdhist, err := os.ReadFile(commandHistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			cmdhist = nil
		} else {
			log.Fatal(err)
		}
	}

	var cmdHistoryLines []string

	if cmdhist != nil {
		// Trim leading/trailing whitespace (including newlines)
		trimmedContent := strings.TrimSpace(string(cmdhist))
		// Split the string by the newline character
		cmdHistoryLines = strings.Split(trimmedContent, "\n")
	}

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

	m := initialModel(port, showTimestamp, cmdHistoryLines, commandHistoryFile, dump, *portPtr, mode)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	//fmt.Printf("Command history saved under %s!\n", commandHistoryFile)
}
