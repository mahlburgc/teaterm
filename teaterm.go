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

	"github.com/charmbracelet/bubbles/cursor"
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
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	focusedPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	//selectedCmdStyle = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("230"))
	selectedCmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
)

type model struct {
	dump           io.Writer
	serialVp       viewport.Model
	histVp         viewport.Model
	serMessages    []string
	inputTa        textarea.Model
	rxMessageStyle lipgloss.Style
	err            error
	port           serial.Port
	scanner        *bufio.Scanner
	selectedPort   string
	selectedMode   *serial.Mode
	showTimestamp  bool
	cmdHist        []string
	cmdHistFile    string
	cmdHistIndex   int
	width          int
	height         int
	conStatus      bool
}

func initialModel(port serial.Port, showTimestamp bool, cmdHist []string, cmdHistFile string, dump io.Writer, selectedPort string, selectedMode *serial.Mode) model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 256
	ta.Cursor.Style = cursorStyle
	ta.FocusedStyle.Placeholder = focusedPlaceholderStyle
	ta.FocusedStyle.Base = focusedBorderStyle
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // Remove cursor line styling
	ta.ShowLineNumbers = false
	ta.SetWidth(30)
	ta.SetHeight(1)

	serialVp := viewport.New(30, 5)
	serialVp.SetContent(`Welcome to teaterm!`)
	serialVp.Style = focusedBorderStyle

	histVp := viewport.New(30, 5)
	histVp.Style = focusedBorderStyle

	// Disable the viewport's default up/down key handling so it doesn't scroll
	// when we are navigating command history.
	serialVp.KeyMap.Up.SetEnabled(false)
	serialVp.KeyMap.Down.SetEnabled(false)

	ta.KeyMap.InsertNewline.SetEnabled(false) //TODO check
	serialVp.KeyMap.PageUp.SetEnabled(false)
	serialVp.KeyMap.PageDown.SetEnabled(false)

	scanner := bufio.NewScanner(port)

	return model{
		dump:           dump,
		inputTa:        ta,
		serMessages:    []string{},
		serialVp:       serialVp,
		histVp:         histVp,
		rxMessageStyle: focusedPlaceholderStyle,
		err:            nil,
		port:           port,
		scanner:        scanner,
		selectedPort:   selectedPort,
		selectedMode:   selectedMode,
		showTimestamp:  showTimestamp,
		cmdHist:        cmdHist,
		cmdHistIndex:   len(cmdHist),
		cmdHistFile:    cmdHistFile,
		width:          0,
		height:         0,
		conStatus:      true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, readFromPort(m.scanner, m.dump))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		//avoid print cursor blink
	default:
		spew.Fdump(m.dump, msg)
	}

	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.inputTa, tiCmd = m.inputTa.Update(msg)
	m.serialVp, vpCmd = m.serialVp.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		screenWidth := msg.Width - 2
		screenHight := msg.Height - 4

		m.serialVp.Width = screenWidth / 4 * 3
		m.histVp.Width = screenWidth - m.serialVp.Width
		m.inputTa.SetWidth(screenWidth)

		m.serialVp.Height = screenHight - m.inputTa.Height()
		m.histVp.Height = m.serialVp.Height

		if len(m.serMessages) > 0 {
			m.serialVp.SetContent(lipgloss.NewStyle().Width(m.serialVp.Width).Render(strings.Join(m.serMessages, "\n")))
		}
		m.serialVp.GotoBottom()

		historyContent := strings.Join(m.cmdHist, "\n")
		m.histVp.SetContent(lipgloss.NewStyle().Width(m.histVp.Width).Render(historyContent))
		m.histVp.GotoBottom()

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
				t := time.Now().Format("00:00:00.000")
				line.WriteString(fmt.Sprintf("[%s] ", t))
			}
			// line.WriteString("> ")
			line.WriteString(userInput)

			// TODO set serial message histrory limit, remove oldest if exceed
			m.serMessages = append(m.serMessages, m.rxMessageStyle.Render(line.String())) // TODO directly use style for var()
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
		vpCmd = readFromPort(m.scanner, m.dump)

	case *serial.PortError:
		switch msg.Code() {
		case serial.PortClosed:
			m.conStatus = false
			m.port.Close()
			tiCmd = reconnectPort(m.selectedPort, m.selectedMode)
		}

	case portConnectedMsg:
		m.conStatus = true
		m.port = msg
		m.scanner = bufio.NewScanner(m.port)
		vpCmd = readFromPort(m.scanner, m.dump)

	case errMsg:
		m.err = error(msg)
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {

	var connectionText string
	if m.conStatus {
		connectionText = "connected | "
	} else {
		connectionText = "disconnected | "
	}

	footerText := connectionText + "↑/↓: scroll commands · PageUp/PageDown: scroll messages"
	if m.cmdHistIndex != len(m.cmdHist) {
		footerText += " · ctrl+d: delete command"
	}

	footer := footerStyle.Render(footerText)

	// Arrange viewports side by side
	viewports := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.serialVp.View(),
		m.histVp.View(),
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

func readFromPort(scanner *bufio.Scanner, dump io.Writer) tea.Cmd {
	return func() tea.Msg {
		for scanner.Scan() {
			line := scanner.Text()
			return serialMsg(line)
		}

		if err := scanner.Err(); err != nil {
			//spew.Fdump(dump, "read error:", err)
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
