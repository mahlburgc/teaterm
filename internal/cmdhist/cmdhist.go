package cmdhist

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	list        list.Model
	SelectStyle lipgloss.Style
	active      bool
	height      int
	width       int
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// cmdDelegate implements list.ItemDelegate to support bubblezone marking.
type cmdDelegate struct {
	list.DefaultDelegate
}

func (d cmdDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	// To preserve the default filtering highlights, we capture the output of the
	// standard Render method into a buffer, then wrap the result in a zone.
	var buf strings.Builder
	d.DefaultDelegate.Render(&buf, m, index, listItem)

	// Mark the rendered string with a unique ID for this index.
	fmt.Fprint(w, zone.Mark(fmt.Sprintf("hist-item-%d", index), buf.String()))
}

func New(cmdHist []string) Model {
	items := make([]list.Item, len(cmdHist))
	for i, cmd := range cmdHist {
		items[i] = item{title: cmd}
	}

	normalTitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"}).
		Padding(0, 0, 0, 2) //nolint:mnd

	border := lipgloss.NormalBorder()
	border.Left = ">"
	selectedTitleStyle := lipgloss.NewStyle().
		Border(border, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
		Padding(0, 0, 0, 1)

	// Create our custom delegate that wraps the default styles
	baseDelegate := list.NewDefaultDelegate()
	baseDelegate.ShowDescription = false
	baseDelegate.SetSpacing(0)
	baseDelegate.Styles.SelectedTitle = selectedTitleStyle
	baseDelegate.Styles.NormalTitle = normalTitleStyle

	delegate := cmdDelegate{DefaultDelegate: baseDelegate}

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
	m.height = 0
	m.width = 0

	m.GoToLastItem()

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

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonLeft {
			for i := 0; i < len(m.list.Items()); i++ {
				if zone.Get(fmt.Sprintf("hist-item-%d", i)).InBounds(msg) {
					if msg.Action == tea.MouseActionRelease {
						if itm, ok := m.list.Items()[i].(item); ok {
							return m, SendCmdExecutedMsg(itm.title)
						}
					} else {
						m.list.Select(i)
						return m, nil
					}
				}
			}
		}
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
	case events.SendMsg:
		if !msg.FromCmdHist {
			m.AddCmd(msg.Data)
		}

	case events.PartialTxMsg:
		return m, m.findCmd(string(msg))

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.DeleteCmdKey):
			m.list.RemoveItem(m.list.Index())
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

	vp.Height = m.height - 2
	vp.Width = m.width - 2
	vp.SetContent(lipgloss.NewStyle().Width(m.width - 2).Render(m.list.View()))
	return styles.AddBorder(vp, "History", "")
}

// Add a new command to the command history. The command will only be added, if not
// already exisiting in the hist. If cmd is found, it will be moved to the end.
func (m *Model) AddCmd(newCmd string) (c tea.Cmd) {
	if newCmd == "" {
		return nil
	}

	items := m.list.Items()
	// Check if already in the list
	for i, itm := range items {
		if cmdItem, ok := itm.(item); ok && cmdItem.title == newCmd {
			// Remove the existing item
			m.list.RemoveItem(i)
			break
		}
	}

	// Insert at the end (the current length is the last index)
	m.list.InsertItem(len(m.list.Items()), item{title: newCmd})

	// Ensure we jump to the new last item
	m.GoToLastItem()

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

func (m *Model) SetSize(width int, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width-2, height-2)
}

func (m *Model) GoToLastItem() {
	items := m.list.Items()
	if len(items) > 0 {
		m.list.Select(len(items) - 1)
	}
}
