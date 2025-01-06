package model

import (
	"fmt"
	"strings"
)

func (m *Model) View() string {
	var b strings.Builder

	switch m.State {
	case MainMenu:
		b.WriteString("What would you like to do?\n\n")

		for i, choice := range m.Choices {
			cursor := " "
			if m.Cursor == i {
				cursor = ">"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
		}

	case ListSubMenu:
		if m.Choices[m.lastMainCursor] == "pod" {
			b.WriteString(renderChoices(m.SubChoices, m.Cursor, true))
		} else {
			b.WriteString("List sub-options:\n\n")
			b.WriteString(renderChoices(m.SubChoices, m.Cursor, false))
		}

	case RenewalConfirm:
		b.WriteString("Renewal Confirm\n\n")

	case VolumeResizeMenu:
		b.WriteString("Select a volume to delete:\n\n")

		for i, choice := range m.SubChoices {
			cursor := " "
			if m.Cursor == i {
				cursor = ">"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
		}
	}

	if m.Message != "" {
		b.WriteString(messageStyle.Render(m.Message))
		b.WriteRune('\n')
	}

	b.WriteString("\n(↑/↓ or j/k to move, enter to select")
	if m.State == ListSubMenu {
		b.WriteString(", backspace to go back")
	}
	b.WriteString(", q to quit)\n")

	return b.String()
}

func renderChoices(choices []string, cursor int, isPodList bool) string {
	var b strings.Builder
	for i, choice := range choices {
		if isPodList {
			columns := strings.Split(choice, "\t")
			if len(columns) != 6 {
				continue // Skip invalid rows
			}

			rowStyle := podNormalStyle
			if i == cursor {
				rowStyle = podSelectedStyle
			}

			b.WriteString(rowStyle.Render(
				namespaceStyle.Render(columns[0]) +
					nameStyle.Render(columns[1]) +
					readyStyle.Render(columns[2]) +
					statusStyle.Render(columns[3]) +
					restartsStyle.Render(columns[4]) +
					ageStyle.Render(columns[5]),
			))
		} else {
			prefix := "  "
			if i == cursor {
				prefix = "> "
				b.WriteString(selectedStyle.Render(prefix + choice))
			} else {
				b.WriteString(normalStyle.Render(prefix + choice))
			}
		}
		b.WriteRune('\n')
	}
	return b.String()
}
