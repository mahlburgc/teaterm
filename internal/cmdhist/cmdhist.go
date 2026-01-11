package cmdhist

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
)

type Model struct {
	list        list.Model
	SelectStyle lipgloss.Style
	active      bool
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func New(cmdHist []string) Model {
	items := make([]list.Item, len(cmdHist))
	for i, cmd := range cmdHist {
		items[i] = item{title: cmd}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	m := Model{
		list: list.New(items, delegate, 0, 0),
	}
	m.list.Title = "My Fave Things"
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.SetShowPagination(false)
	m.list.SetShowTitle(false)
	m.list.DisableQuitKeybindings()

	m.list.ResetSelected()
	m.active = true

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case events.ConnectionStatusMsg:
		switch msg.Status {
		case events.Disconnected:
			m.active = false

		case events.Connected:
			m.active = true

		case events.Connecting:
			m.active = false
		}
		return m, nil
	}

	// do not handle any other events during inactive state
	if m.active == false {
		return m, nil
	}

	m.list, cmd = m.list.Update(msg)
	if cmd != nil {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// h, v := docStyle.GetFrameSize()
		m.list.SetSize(20, 10)

	case events.SendMsg:
		if !msg.FromCmdHist {
			m.AddCmd(msg.Data)
		}

	case events.PartialTxMsg:
		return m, m.findCmd(string(msg))

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.DeleteCmdKey):
			// TODO
			return m, nil

		case key.Matches(msg, keymap.Default.ToggleHistKey, keymap.Default.ResetKey):
			m.GoToLastItem()
			return m, nil

		default:
			return m, nil
		}

	default:
		return m, nil
	}

	return m, nil
}

func (m *Model) findCmd(msg string) tea.Cmd {
	return nil
}

// Returns a Tea command to send a message with the arrow selected cmd to the event loop.
func SendCmdExecutedMsg(cmd string) tea.Cmd {
	return func() tea.Msg {
		return events.SendMsg{Data: cmd, FromCmdHist: true}
	}
}

// View renders the model's view.
func (m Model) View() string {
	var vp viewport.Model

	// TODO HIER WEITER ANSICHT IN VIEWPORT PACKEN
	return docStyle.Render(m.list.View())
}

// Add a new command to the command history. The command will only be added, if not
// already exisiting in the hist. If cmd is found, it will be moved to the end.
func (m *Model) AddCmd(newCmd string) (c tea.Cmd) {
	// log.Printf("add command: %s\n", newCmd)
	// foundIndex := -1
	// for i, cmd := range m.cmdHist {
	// 	if cmd == newCmd {
	// 		foundIndex = i
	// 		break
	// 	}
	// }

	// if foundIndex != -1 {
	// 	m.cmdHist = append(m.cmdHist[:foundIndex], m.cmdHist[foundIndex+1:]...)
	// 	m.cmdHist = append(m.cmdHist, newCmd)
	// } else {
	// 	m.cmdHist = append(m.cmdHist, newCmd)
	// }

	// return m.ResetVp()
	return nil
}

// GetCmdHist returns the current list of commands as a string slice.
func (m Model) GetCmdHist() []string {
	items := m.list.Items()
	cmds := make([]string, len(items))
	for i, itm := range items {
		// Type assert the list.Item back to our internal item struct
		if cmdItem, ok := itm.(item); ok {
			cmds[i] = cmdItem.title
		}
	}
	return cmds
}

func (m *Model) GoToLastItem() {
	items := m.list.Items()
	if len(items) > 0 {
		m.list.Select(len(items) - 1)
	}
}
