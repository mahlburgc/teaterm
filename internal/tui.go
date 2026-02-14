package internal

import (
	"io"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/internal/cmdhist"
	"github.com/mahlburgc/teaterm/internal/footer"
	help "github.com/mahlburgc/teaterm/internal/help-overlay"
	"github.com/mahlburgc/teaterm/internal/input"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/msglog"
	"github.com/mahlburgc/teaterm/internal/session"
	"github.com/mahlburgc/teaterm/internal/styles"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"go.bug.st/serial"
)

type model struct {
	msglog     msglog.Model
	cmdhist    cmdhist.Model
	input      input.Model
	footer     footer.Model
	session    session.Model
	help       help.Model
	showCmdLog bool
	showHelp   bool
	restartApp bool
	width      int
	height     int
}

func initialModel(port *io.ReadWriteCloser, showTimestamp bool, cmdHist []string,
	selectedPort string, selectedMode *serial.Mode, serialLog *log.Logger, showEscapes bool,
) model {
	input := input.New()
	cmdhist := cmdhist.New(cmdHist)
	msglog := msglog.New(showTimestamp, showEscapes, styles.VpTxMsgStyle,
		styles.ErrMsgStyle, styles.InfoMsgStyle, serialLog, 50000)
	footer := footer.New()
	session := session.New(port, selectedPort, selectedMode)
	help := help.New()

	return model{
		msglog:     msglog,
		cmdhist:    cmdhist,
		input:      input,
		footer:     footer,
		session:    session,
		help:       help,
		showCmdLog: true,
		showHelp:   false,
		width:      0,
		height:     0,
		restartApp: false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.session.Init())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	DbgLogMsgType(msg)

	m.cmdhist, cmd = m.cmdhist.Update(msg)
	cmds = append(cmds, cmd)

	m.msglog, cmd = m.msglog.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.session, cmd = m.session.Update(msg)
	cmds = append(cmds, cmd)

	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		cmds = append(cmds, m.handleKeys(msg))

	case msglog.EditorFinishedMsg:
		// workaround bubbletea v1 bug: after executing external command,
		// mouse support is not restored correctly. Therefore we restart bubbletea.
		m.restartApp = true
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	viewports := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.msglog.View(),
		m.cmdhist.View(),
	)

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		viewports,
		m.input.View(),
		m.footer.View(m.session.View()),
	)

	output := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		screen,
	)

	if m.showHelp {
		output = overlay.Composite(m.help.View(), output, overlay.Center, overlay.Center, 0, 0)
	}

	return zone.Scan(output)
}

func (m *model) handleKeys(keyMsg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(keyMsg, keymap.Default.QuitKey):
		StoreConfig(m.cmdhist.GetCmdHist())
		return tea.Quit

	case key.Matches(keyMsg, keymap.Default.ToggleHistKey):
		m.showCmdLog = !m.showCmdLog
		m.updateLayout()

	case key.Matches(keyMsg, keymap.Default.HelpKey):
		m.showHelp = !m.showHelp

	case key.Matches(keyMsg, keymap.Default.CloseKey, keymap.Default.ResetKey):
		m.showHelp = false
	}

	return nil
}

func (m *model) updateLayout() {
	footerHeight := m.footer.GetHeight()
	inputHeight := m.input.GetHeight()
	viewportsHeight := m.height - inputHeight - footerHeight

	// 75% width for Message Log, 25% for Command History
	msgLogWidth := (m.width / 4) * 3

	if !m.showCmdLog {
		msgLogWidth = m.width
	}

	cmdHistWidth := m.width - msgLogWidth

	m.footer.SetWidth(m.width)
	m.input.SetWidth(m.width)

	m.msglog.SetSize(msgLogWidth, viewportsHeight)
	m.cmdhist.SetSize(cmdHistWidth, viewportsHeight)
}

func RunTui(port *io.ReadWriteCloser, mode serial.Mode, flags Flags, config Config, serialLog *log.Logger) {
	zone.NewGlobal()

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
