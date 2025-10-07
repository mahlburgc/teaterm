package internal

import "github.com/charmbracelet/lipgloss"

var (
	CursorStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	ConnectSymbolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("77"))
	FocusedPlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	FocusedBorderStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("242"))
	BlurredBorderStyle      = FocusedBorderStyle
	SelectedCmdStyle        = CursorStyle
	SpinnerStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	VpTxMsgStyle            = CursorStyle
	FooterStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	FocusedPromtStyle       = CursorStyle
	BlurredPromtStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)
