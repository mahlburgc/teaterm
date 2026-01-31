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
	AdaptiveCyan        = lipgloss.AdaptiveColor{Light: "#006362", Dark: "#96ffec"}
	AdaptiveGreen       = lipgloss.AdaptiveColor{Light: "#41ab00", Dark: "#6cff11"}
	AdaptiveRed         = lipgloss.AdaptiveColor{Light: "#8f0000", Dark: "#be0000"}
	AdaptiveBorderColor = AdaptiveGray

	CursorStyle = lipgloss.NewStyle().Foreground(AdaptivePink)

	ConnectSymbolStyle = lipgloss.NewStyle().Foreground(AdaptiveGreen)

	DisconnectedSymbolStyle = lipgloss.NewStyle().Foreground(AdaptiveRed)
	FocusedPlaceholderStyle = lipgloss.NewStyle().Foreground(AdaptiveGray)
	BorderStyle             = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(AdaptiveBorderColor)
	SelectedCmdStyle        = lipgloss.NewStyle().Foreground(AdaptivePink)
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

// Adds a border with title to viewport and returns viewport string.
func AddBorder(vp viewport.Model, title string, footer string, ownFooterStyle bool) string {
	border := BorderStyle.GetBorderStyle()
	borderStyle := lipgloss.NewStyle().Foreground(AdaptiveBorderColor)

	var vpTitle string

	if title == "" {
		vpTitle = ""
	} else {
		vpTitle = borderStyle.Render(border.Top + border.MiddleRight + " " + title + " " + border.MiddleLeft)
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
