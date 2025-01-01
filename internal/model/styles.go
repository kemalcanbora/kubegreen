package model

import "github.com/charmbracelet/lipgloss"

var (
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			Background(lipgloss.Color("239")).
			Width(20).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Width(20).
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Italic(true).
			MarginTop(1).
			MarginBottom(1)

	// New styles for pod list
	podSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("170")).
				Background(lipgloss.Color("239"))

	podNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	namespaceStyle = lipgloss.NewStyle().Width(30)
	nameStyle      = lipgloss.NewStyle().Width(50)
	readyStyle     = lipgloss.NewStyle().Width(10).Align(lipgloss.Right)
	statusStyle    = lipgloss.NewStyle().Width(15).Align(lipgloss.Center)
	restartsStyle  = lipgloss.NewStyle().Width(15).Align(lipgloss.Right)
	ageStyle       = lipgloss.NewStyle().Width(10).Align(lipgloss.Right)
)
