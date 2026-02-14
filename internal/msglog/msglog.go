package msglog

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/icza/gox/stringsx"
	"github.com/mahlburgc/teaterm/events"
	"github.com/mahlburgc/teaterm/internal/keymap"
	"github.com/mahlburgc/teaterm/internal/styles"
)

type Model struct {
	Vp            viewport.Model
	sendStyle     lipgloss.Style
	errStyle      lipgloss.Style
	infoStyle     lipgloss.Style
	log           []string
	logFiltered   []string
	showTimestamp bool
	serialLog     *log.Logger
	txPrefix      string
	rxPrefix      string
	errPrefix     string
	infoPrefix    string
	showEscapes   bool
	logLimit      int
	msgCnt        int // rx and tx messages during one session
	filterString  string
	scrollIndex   int
	needsUpdate   bool
}

// This message is sent when the editor is closed.
type EditorFinishedMsg struct {
	err error
}

const (
	rxMsg = iota
	txMsg
	errMsg
	infoMsg
)

// New creates a new model with default settings.
func New(showTimestamp bool, showEscapes bool, sendStyle lipgloss.Style,
	errStyle lipgloss.Style, infoStyle lipgloss.Style, serialLog *log.Logger, logLimit int,
) (m Model) {
	// Serial viewport contains all sent and received messages.
	// We will create a viewport without border and later manually
	// add the border to inject a title into the border.
	m.Vp = viewport.New(30, 5)
	m.Vp.SetContent(`Welcome to teaterm!`)
	m.Vp.Style = lipgloss.NewStyle()
	// Disable the viewport's default up/down key handling so it doesn't scroll
	// when we are navigating through the command history.
	m.Vp.KeyMap.Up.SetEnabled(false)
	m.Vp.KeyMap.Down.SetEnabled(false)
	m.Vp.KeyMap.PageUp.SetEnabled(false)
	m.Vp.KeyMap.PageDown.SetEnabled(false)

	m.log = []string{}
	m.logFiltered = []string{}

	m.log = append(m.log, m.startMsg())

	m.txPrefix = ""
	m.rxPrefix = ""
	m.errPrefix = "ERROR: "
	m.infoPrefix = "INFO: "
	m.serialLog = serialLog
	m.showEscapes = showEscapes
	m.logLimit = logLimit
	m.msgCnt = 0
	m.filterString = ""
	m.needsUpdate = false

	m.sendStyle = sendStyle
	m.errStyle = errStyle
	m.infoStyle = infoStyle
	m.showTimestamp = showTimestamp

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// viewport will be managed completely manually,
	// so viewports update function will not be called.

	switch msg := msg.(type) {

	case events.MsgLogFilterStringMsg:
		m.filterString = string(msg)
		m.scrollIndex = 0           // reset scrolling
		m.filterLog(m.filterString) // only filter whole log if new filter string was set

	case events.SendMsg:
		m.addMsg(msg.Data, txMsg)

	case events.SerialRxMsgReceived:
		m.addMsg(string(msg), rxMsg)

	case events.ErrMsg:
		if msg != nil {
			m.addMsg(msg.Error(), errMsg)
		}

	case events.InfoMsg:
		m.addMsg(string(msg), infoMsg)

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollUp(1)

		case tea.MouseButtonWheelDown:
			m.scrollDown(1)
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Default.LogLeftKey):
			m.Vp.ScrollLeft(3)

		case key.Matches(msg, keymap.Default.LogRightKey):
			m.Vp.ScrollRight(3)

		case key.Matches(msg, keymap.Default.LogUpKey):
			m.scrollUp(1)

		case key.Matches(msg, keymap.Default.LogDownKey):
			m.scrollDown(1)

		case key.Matches(msg, keymap.Default.LogUpFastKey):
			m.scrollUp(10)

		case key.Matches(msg, keymap.Default.LogDownFastKey):
			m.scrollDown(10)

		case key.Matches(msg, keymap.Default.LogTopKey):
			m.scrollToTop()

		case key.Matches(msg, keymap.Default.LogBottomKey):
			m.scrollToBottom()

		case key.Matches(msg, keymap.Default.OpenEditorKey):
			return m, openEditorCmd(m.logFiltered)

		case key.Matches(msg, keymap.Default.ClearLogKey):
			if m.Vp.Height > 0 {
				m.log = nil /* reset serial message log */
				m.logFiltered = nil
				m.msgCnt = 0
				m.Vp.SetContent("")
				m.scrollToBottom()
			}
		}

	default:
		return m, nil
	}

	if m.needsUpdate {
		m.needsUpdate = false
		m.UpdateVp()
	}

	return m, nil
}

func (m Model) View() string {
	// mark scroll percentage if we are not at bottom
	// also, if we are not at bottom but at 100 percent scroll
	// map 100 percent to 99 percent for better user experience
	borderStyle := lipgloss.NewStyle().Foreground(styles.AdaptiveBorderColor)
	var percentRenderStyle lipgloss.Style
	scrollPercentage := m.GetScrollPercent()
	if m.atBottom() {
		percentRenderStyle = borderStyle
	} else {
		percentRenderStyle = styles.PercentRenderStyle
	}

	scrollPercentageString := percentRenderStyle.Render(fmt.Sprintf("%3d%%", int(scrollPercentage)))

	footer := borderStyle.Render(fmt.Sprintf("%d ", m.msgCnt)) + scrollPercentageString
	return styles.AddBorder(m.Vp, "Messages", footer, true)
}

func (m *Model) SetSize(width, height int) {
	borderWidth, borderHeight := styles.BorderStyle.GetFrameSize()

	m.Vp.Width = width - borderWidth
	m.Vp.Height = height - borderHeight

	m.scrollIndex = 0

	m.needsUpdate = true
}

func (m *Model) scrollUp(n int) {
	if m.atTop() {
		return
	}

	if m.maxScrollIndex()-m.scrollIndex > n {
		m.scrollIndex = m.scrollIndex + n
	} else {
		m.scrollIndex = m.maxScrollIndex()
	}

	m.needsUpdate = true
}

func (m *Model) maxScrollIndex() int {
	return len(m.logFiltered) - m.Vp.Height
}

func (m *Model) scrollToTop() {
	if m.atTop() {
		return
	}

	m.scrollIndex = m.maxScrollIndex()

	m.needsUpdate = true
}

func (m *Model) scrollToBottom() {
	if m.atBottom() {
		return
	}

	m.scrollIndex = 0

	m.needsUpdate = true
}

func (m *Model) scrollDown(n int) {
	if m.atBottom() {
		return
	}

	if m.scrollIndex-n > 0 {
		m.scrollIndex = m.scrollIndex - n
	} else {
		m.scrollIndex = 0
	}

	m.needsUpdate = true
}

func (m *Model) atTop() bool {
	if len(m.logFiltered) > m.Vp.Height {
		return m.scrollIndex == m.maxScrollIndex()
	} else {
		return true
	}
}

func (m *Model) atBottom() bool {
	if len(m.logFiltered) > m.Vp.Height {
		return m.scrollIndex == 0
	} else {
		return true
	}
}

// Log a message to the viewport
func (m *Model) addMsg(msg string, msgType int) {
	var line strings.Builder
	if m.showTimestamp {
		t := time.Now().Format("15:04:05.000")
		line.WriteString(fmt.Sprintf("[%s] ", t))
	}

	switch msgType {
	case txMsg:
		line.WriteString(m.txPrefix)
		m.msgCnt++
	case errMsg:
		line.WriteString(m.errPrefix)
	case infoMsg:
		line.WriteString(m.infoPrefix)
	default:
		line.WriteString(m.rxPrefix)
		m.msgCnt++
	}

	if m.showEscapes {
		line.WriteString(stringsx.Clean(msg))
	} else {
		line.WriteString(msg) // fmt.Printf("%q", msg")
	}

	if m.serialLog != nil {
		m.serialLog.Println(line.String())
	}

	atBottom := m.atBottom()

	var renderedString string
	switch msgType {
	case txMsg:
		renderedString = m.sendStyle.Render(line.String())
	case errMsg:
		renderedString = m.errStyle.Render(line.String())
	case infoMsg:
		renderedString = m.infoStyle.Render(line.String())
	case rxMsg:
		renderedString = line.String()
	default:
		renderedString = line.String()
	}

	m.log = append(m.log, renderedString)

	// message histrory limit, remove oldest if exceed
	if len(m.log) > m.logLimit {
		m.log = m.log[len(m.log)-m.logLimit:]
		m.log[0] = m.startMsg()
	}

	m.filterLog(m.filterString)

	// always reset vp to bottom if we send new messages or receive info or error messages
	if msgType != rxMsg {
		m.scrollToBottom()
	} else if atBottom == false {
		m.scrollUp(1)
	}
}

func (m *Model) startMsg() string {
	return styles.MsgLogStartRenderStyle.Render(
		fmt.Sprintf("Message log start (limit: %d lines)", m.logLimit))
}

func (m *Model) UpdateVp() {
	if m.Vp.Height <= 0 {
		return
	}

	startIndex := m.getFirstViewableElementIndex()
	stopIndex := m.getLastViewableElementIndex()
	content := strings.Join(m.logFiltered[startIndex:stopIndex], "\n")
	m.Vp.SetContent(content)
}

func (m Model) GetLen() int {
	return len(m.log)
}

func (m *Model) contentFitsInVp() bool {
	return len(m.logFiltered) <= m.Vp.Height
}

func (m *Model) getFirstViewableElementIndex() int {
	if m.contentFitsInVp() {
		return 0
	}
	return m.maxScrollIndex() - m.scrollIndex
}

func (m *Model) getLastViewableElementIndex() int {
	if m.contentFitsInVp() {
		return len(m.logFiltered)
	}
	return len(m.logFiltered) - m.scrollIndex
}

func (m Model) GetScrollPercent() float64 {
	if m.atBottom() {
		return 100
	}

	return 100 - (float64(m.scrollIndex) * 100 / float64(m.maxScrollIndex()))
}

// openEditorCmd creates a tea.Cmd that runs the editor.
func openEditorCmd(content []string) tea.Cmd {
	// Get the editor from the environment variable. Default to vim.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Create a temporary file to store the content.
	tmpFile, err := os.CreateTemp("", "bubbletea-edit-*.txt")
	if err != nil {
		return func() tea.Msg {
			return EditorFinishedMsg{err: err}
		}
	}

	// Write the viewport content to the temp file.
	for _, line := range content {
		if _, err = tmpFile.WriteString(stripansi.Strip(line) + "\n"); err != nil {
			return func() tea.Msg {
				return EditorFinishedMsg{err: err}
			}
		}
	}

	// Close the file so the editor can access it.
	if err := tmpFile.Close(); err != nil {
		return func() tea.Msg {
			return EditorFinishedMsg{err: err}
		}
	}

	// This is the command that will be executed.
	c := exec.Command(editor, tmpFile.Name())

	// The magic is here: tea.ExecProcess handles suspending the Bubble Tea
	// app, running the command, and then sending a message back.
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return EditorFinishedMsg{err: err}
		}

		// Clean up the temporary file.
		err = os.Remove(tmpFile.Name())

		return EditorFinishedMsg{err: err}
	})
}

// FILTER METHODS

// TODO: CURRENTLY NOT USED
// Could be used to filter new receive messages without always filter the whole log!
//
// FilterSingleMsg is the easy-to-use method for checking a single line against a query.
// It handles the regex compilation overhead internally.
func (m *Model) appendToFilteredLog(line string, query string) {
	if query == "" {
		m.logFiltered = append(m.logFiltered, line)
	} else {
		searchWords := strings.Fields(strings.ToLower(query))
		searchRegexps := getRegexSearch(searchWords)

		if highlighted, matches := m.filterMsg(line, searchWords, searchRegexps); matches {
			m.logFiltered = append(m.logFiltered, highlighted)
		}
	}
	m.needsUpdate = true
}

// filterLog processes the entire log.
func (m *Model) filterLog(query string) {
	if query == "" {
		m.logFiltered = m.log
	} else {
		searchWords := strings.Fields(strings.ToLower(query))
		searchRegexps := getRegexSearch(searchWords)

		filtered := make([]string, 0)
		for i, line := range m.log {
			if i == 0 {
				// first element is start message and should always be included and not be filtered
				filtered = append(filtered, line)
			} else if highlighted, matches := m.filterMsg(line, searchWords, searchRegexps); matches {
				filtered = append(filtered, highlighted)
			}
		}
		m.logFiltered = filtered
	}
	m.needsUpdate = true
}

// filterString performs the actual matching and highlighting.
// It accepts pre-compiled regexps to ensure high performance during loops.
func (m *Model) filterMsg(line string, searchWords []string, searchRegexps []*regexp.Regexp) (string, bool) {
	lowerLine := strings.ToLower(line)

	// Fast Check: Basic string matching first (cheap)
	for _, word := range searchWords {
		if !strings.Contains(lowerLine, word) {
			return "", false
		}
	}

	// Highlighting: Regex replacement (expensive, but only done on matches)
	highlightedLine := stripansi.Strip(line)

	for _, re := range searchRegexps {
		highlightedLine = re.ReplaceAllStringFunc(highlightedLine, func(s string) string {
			return styles.SearchHighlightStyle.Render(s)
		})
	}

	return highlightedLine, true
}

func getRegexSearch(searchWords []string) (searchRegexps []*regexp.Regexp) {
	for _, word := range searchWords {
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(word))
		searchRegexps = append(searchRegexps, re)
	}
	return searchRegexps
}
