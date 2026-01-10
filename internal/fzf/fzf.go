package fzf

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	list        list.Model
	SelectStyle lipgloss.Style
}

var docStyle = styles.FocusedBorderStyle

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
	m.list.SetShowTitle(true)
	m.list.DisableQuitKeybindings()

	m.list.ResetSelected()

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// h, v := docStyle.GetFrameSize()
		m.list.SetSize(20, 10)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return docStyle.Render(m.list.View())
}

func (m *Model) GoToLastItem() {
	items := m.list.Items()
	if len(items) > 0 {
		m.list.Select(len(items) - 1)
	}
}
