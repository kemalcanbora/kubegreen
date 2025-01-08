package model

import (
	time "time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Init() tea.Cmd {
	if m.State == MetricsView {
		return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return t
		})
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.State == VolumeResizeMenu || m.State == VolumeSizeInput {
			return m.handleVolumeMenu(msg)
		}
		if m.State == MetricsView {
			switch msg.String() {
			case "q", "ctrl+c", "esc", "backspace":
				m.State = MainMenu
				m.Cursor = m.lastMainCursor
				m.Message = ""
				return m, nil
			}
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return t
			})
		}
		return m.handleKeyPress(msg)
	case time.Time:
		if m.State == MetricsView {
			m.Message = m.handleMetrics()
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return t
			})
		}
	}
	return m, nil
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "enter":
		m.handleEnter()
	case "backspace":
		if m.State == ListSubMenu {
			m.State = MainMenu
			m.Cursor = 0
		}

	case "y", "Y":
		if m.State == RenewalConfirm {
			m.renewalResponse = "y"
			m.handleEnter()
		}
	case "n", "N":
		if m.State == RenewalConfirm {
			m.renewalResponse = "n"
			m.Message = "Certificate renewal cancelled"
			m.State = MainMenu
		}
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	newCursor := m.Cursor + delta
	if m.State == MainMenu {
		m.Cursor = bound(newCursor, 0, len(m.Choices)-1)
	} else {
		m.Cursor = bound(newCursor, 0, len(m.SubChoices)-1)
	}
}

func bound(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
