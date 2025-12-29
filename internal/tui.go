package internal

import (
	"fmt"
	"io"
	"log"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/cmdhist"
	"github.com/mahlburgc/teaterm/internal/footer"
	"github.com/mahlburgc/teaterm/internal/input"
	"github.com/mahlburgc/teaterm/internal/msglog"
	"github.com/mahlburgc/teaterm/internal/session"
	"github.com/mahlburgc/teaterm/internal/styles"
	"go.bug.st/serial"
)

type model struct {
	msglog     msglog.Model
	cmdhist    cmdhist.Model
	input      input.Model
	footer     footer.Model
	session    session.Model
	err        error
	restartApp bool
	width      int
	height     int
}

func initialModel(port *io.ReadWriteCloser, showTimestamp bool, cmdHist []string,
	selectedPort string, selectedMode *serial.Mode, serialLog *log.Logger, showEscapes bool,
) model {
	input := input.New()
	cmdhist := cmdhist.New(cmdHist)
	msglog := msglog.New(showTimestamp, showEscapes, styles.VpTxMsgStyle, serialLog)
	footer := footer.New()
	session := session.New(port, selectedPort, selectedMode)

	return model{
		msglog:     msglog,
		cmdhist:    cmdhist,
		input:      input,
		footer:     footer,
		session:    session,
		err:        nil,
		width:      0,
		height:     0,
		restartApp: false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.session.ReadFromPort())
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		HandleNewWindowSize(&m, msg)

	case tea.KeyMsg:
		cmds = append(cmds, HandleKeys(&m, msg))

	case events.ErrMsg:
		switch msg := msg.(type) {
		case *serial.PortError:
			// TODO move error handling to session module
			cmds = append(cmds, m.session.HandleSerialPortErr(msg))
		default:
			m.err = error(msg)
		}

	case msglog.EditorFinishedMsg:
		// workaround bubbletea v1 bug: after executing external command,
		// mouse support is not restored correctly. Therefore we restart bubbletea.
		m.restartApp = true
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nWe had some trouble: %v\n\n", m.err)
	}

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

	return zone.Scan(lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		screen))
}

func HandleKeys(m *model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+q":
		StoreConfig(m.cmdhist.GetCmdHist())
		return tea.Quit
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		StoreConfig(m.cmdhist.GetCmdHist())
		return tea.Quit
	}
	return nil
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
	m.footer.SetWidth(m.width)

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
