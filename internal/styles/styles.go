package styles

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	AdaptiveGray        = lipgloss.AdaptiveColor{Light: "#545454", Dark: "#989898"}
	AdaptiveGrayTwo     = lipgloss.AdaptiveColor{Light: "#858585", Dark: "#5f5f5f"}
	AdaptivePink        = lipgloss.AdaptiveColor{Light: "#9f008f", Dark: "#f943e3"}
	AdaptiveCyan        = lipgloss.AdaptiveColor{Light: "#006362", Dark: "#43f9c6"}
	AdaptiveGreen       = lipgloss.AdaptiveColor{Light: "#41ab00", Dark: "#6cff11"}
	AdaptiveRed         = lipgloss.AdaptiveColor{Light: "#8f0000", Dark: "#be0000"}
	AdaptiveSelectedBg  = lipgloss.AdaptiveColor{Light: "#dcdcdc", Dark: "#303030"}
	AdaptiveBorderColor = AdaptiveGray
	// pastel pink "text marker" background with dark text for selections
	AdaptiveSelectionBg = lipgloss.AdaptiveColor{Light: "#ffb0ee", Dark: "#ff9de8"}
	AdaptiveSelectionFg = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"}

	CursorStyle       = lipgloss.NewStyle().Foreground(AdaptivePink)
	CursorFilterStyle = lipgloss.NewStyle().Foreground(AdaptiveCyan)

	ConnectSymbolStyle      = lipgloss.NewStyle().Foreground(AdaptiveGreen)
	DisconnectedSymbolStyle = lipgloss.NewStyle().Foreground(AdaptiveRed)
	FocusedPlaceholderStyle = lipgloss.NewStyle().Foreground(AdaptiveGray)
	BorderStyle             = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(AdaptiveBorderColor)
	SelectedCmdStyle        = lipgloss.NewStyle().Foreground(AdaptivePink).Background(AdaptiveSelectedBg)
	SelectionStyle          = lipgloss.NewStyle().Background(AdaptiveSelectionBg).Foreground(AdaptiveSelectionFg)
	VersionStyle            = lipgloss.NewStyle().Foreground(AdaptiveGray)
	SpinnerStyle            = lipgloss.NewStyle().Foreground(AdaptivePink)
	VpTxMsgStyle            = lipgloss.NewStyle().Foreground(AdaptivePink)
	ErrMsgStyle             = lipgloss.NewStyle().Foreground(AdaptiveGray)
	InfoMsgStyle            = lipgloss.NewStyle().Foreground(AdaptiveGray)
	FooterStyle             = lipgloss.NewStyle().Foreground(AdaptiveGray)
	FocusedPromtStyle       = lipgloss.NewStyle().Foreground(AdaptivePink)
	FocusedSearchPromtStyle = lipgloss.NewStyle().Foreground(AdaptiveCyan)
	BlurredPromtStyle       = lipgloss.NewStyle().Foreground(AdaptiveGray)
	HelpOverlayBorderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).
				BorderForeground(AdaptiveCyan).Padding(0, 1, 1)
	PercentRenderStyle     = lipgloss.NewStyle().Foreground(AdaptiveCyan)
	MsgLogStartRenderStyle = lipgloss.NewStyle().Foreground(AdaptiveGray)
	HelpKey                = lipgloss.NewStyle().Foreground(AdaptiveGray)
	HelpDesc               = lipgloss.NewStyle().Foreground(AdaptiveGrayTwo)
	HelpSep                = lipgloss.NewStyle().Foreground(AdaptiveGrayTwo)
	SearchHighlightStyle   = lipgloss.NewStyle().Foreground(AdaptiveCyan).Bold(true)
)

// minimalBorders replaces the full boxes around components by a single
// separator line above each component. Set once on startup.
var minimalBorders bool

// SetMinimalBorders switches between full boxes (false) and single separator
// lines (true) for all components drawn with AddBorder.
func SetMinimalBorders(minimal bool) {
	minimalBorders = minimal
}

// MinimalBorders reports whether the minimal border layout is active.
func MinimalBorders() bool {
	return minimalBorders
}

// SeparatorLine returns a plain horizontal border line of the given width.
func SeparatorLine(width int) string {
	border := BorderStyle.GetBorderStyle()
	return lipgloss.NewStyle().Foreground(AdaptiveBorderColor).
		Render(strings.Repeat(border.Top, max(0, width)))
}

// FrameSize returns the horizontal and vertical space used by the border
// drawn by AddBorder.
func FrameSize() (width, height int) {
	if minimalBorders {
		return 0, 1
	}
	return BorderStyle.GetFrameSize()
}

// Adds a border with title to viewport and returns viewport string.
func AddBorder(vp viewport.Model, title string, footer string, ownFooterStyle bool) string {
	if minimalBorders {
		return addSeparator(vp, title, footer, ownFooterStyle)
	}

	border := BorderStyle.GetBorderStyle()
	borderStyle := lipgloss.NewStyle().Foreground(AdaptiveBorderColor)

	var vpTitle string

	if title == "" {
		vpTitle = ""
	} else {
		vpTitle = renderTitle(title)
		// Remove title if width is too low
		if lipgloss.Width(vpTitle) > vp.Width {
			vpTitle = ""
		}
	}

	// Manually construct the top line of the border with the title inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	vpTitleBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		borderStyle.Render(border.TopLeft),
		vpTitle,
		borderStyle.
			Render(strings.Repeat(border.Top, max(0, vp.Width-lipgloss.
				Width(vpTitle)+BorderStyle.GetHorizontalPadding()))),
		borderStyle.Render(border.TopRight),
	)

	var vpFooter string

	if footer == "" {
		vpFooter = ""
	} else {
		if ownFooterStyle {
			vpFooter = borderStyle.Render(border.MiddleRight) + " " + footer + " " +
				borderStyle.Render(border.MiddleLeft) + borderStyle.Render(border.Bottom)
		} else {
			vpFooter = borderStyle.Render(border.MiddleRight + " " + footer + " " +
				border.MiddleLeft + border.Bottom)
		}
		// Remove footer if width is too low
		if lipgloss.Width(vpFooter) > vp.Width {
			vpFooter = ""
		}
	}

	// Manually construct the bottom line of the border with the scroll percentage inside.
	// We calculate the number of "─" characters needed to fill the rest of the line.
	vpFooterBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		borderStyle.Render(border.BottomLeft),
		borderStyle.
			Render(strings.Repeat(border.Top, max(0, vp.Width-lipgloss.
				Width(vpFooter)+BorderStyle.GetHorizontalPadding()))),
		vpFooter,
		borderStyle.Render(border.BottomRight),
	)

	// Render the viewport content inside a box that has NO top and bottom border.
	vpBody := BorderStyle.BorderTop(false).BorderBottom(false).Render(vp.View())

	// Join the title bar and the main content vertically.
	return lipgloss.JoinVertical(lipgloss.Left, vpTitleBar, vpBody, vpFooterBar)
}

// addSeparator renders the viewport with a single separator line above it.
// The line holds the title on the left and the footer on the right:
// ─┤ title ├──────┤ footer ├─
func addSeparator(vp viewport.Model, title string, footer string, ownFooterStyle bool) string {
	border := BorderStyle.GetBorderStyle()
	borderStyle := lipgloss.NewStyle().Foreground(AdaptiveBorderColor)

	var lineTitle string
	if title != "" {
		lineTitle = renderTitle(title)
	}

	var lineFooter string
	if footer != "" {
		if ownFooterStyle {
			lineFooter = borderStyle.Render(border.MiddleRight) + " " + footer + " " +
				borderStyle.Render(border.MiddleLeft) + borderStyle.Render(border.Top)
		} else {
			lineFooter = borderStyle.Render(border.MiddleRight + " " + footer + " " +
				border.MiddleLeft + border.Top)
		}
	}

	// Drop footer, then title, if the line is too narrow.
	if lipgloss.Width(lineTitle)+lipgloss.Width(lineFooter) > vp.Width {
		lineFooter = ""
	}
	if lipgloss.Width(lineTitle) > vp.Width {
		lineTitle = ""
	}

	fill := vp.Width - lipgloss.Width(lineTitle) - lipgloss.Width(lineFooter)
	line := lineTitle + SeparatorLine(fill) + lineFooter

	return lipgloss.JoinVertical(lipgloss.Left, line, vp.View())
}

// renderTitle renders a border title piece: ─┤ title ├
// A plain title is drawn in the border color, a pre-styled title (containing
// escape sequences) keeps its own colors.
func renderTitle(title string) string {
	border := BorderStyle.GetBorderStyle()
	borderStyle := lipgloss.NewStyle().Foreground(AdaptiveBorderColor)
	if !strings.Contains(title, "\x1b") {
		title = borderStyle.Render(title)
	}
	return borderStyle.Render(border.Top+border.MiddleRight+" ") + title +
		borderStyle.Render(" "+border.MiddleLeft)
}
