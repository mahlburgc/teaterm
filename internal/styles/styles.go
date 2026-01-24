package styles

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	CursorStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	ConnectSymbolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("77"))
	DisconnectedSymbolStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("124"))
	FocusedPlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	FocusedBorderStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("242"))
	BlurredBorderStyle      = FocusedBorderStyle
	SelectedCmdStyle        = CursorStyle
	SpinnerStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	VpTxMsgStyle            = CursorStyle
	ErrMsgStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	InfoMsgStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	FooterStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	FocusedPromtStyle       = CursorStyle
	BlurredPromtStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	HelpOverlayBorderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).
				BorderForeground(lipgloss.Color("6")).Padding(0, 1, 1)
)

// Adds a border with title to viewport and returns viewport string.
func AddBorder(vp viewport.Model, title string, footer string, ownFooterStyle bool) string {
	border := FocusedBorderStyle.GetBorderStyle()
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

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
				Width(vpTitle)+FocusedBorderStyle.GetHorizontalPadding()))),
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
				Width(vpFooter)+FocusedBorderStyle.GetHorizontalPadding()))),
		vpFooter,
		borderStyle.Render(border.BottomRight),
	)

	// Render the viewport content inside a box that has NO top and bottom border.
	vpBody := FocusedBorderStyle.BorderTop(false).BorderBottom(false).Render(vp.View())

	// Join the title bar and the main content vertically.
	return lipgloss.JoinVertical(lipgloss.Left, vpTitleBar, vpBody, vpFooterBar)
}
