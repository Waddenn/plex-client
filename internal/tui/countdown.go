package tui

import (
	"fmt"
	"time"

	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CountdownModel struct {
	SecondsRemaining int
	NextTitle        string
	NextAction       func() tea.Cmd
	CancelAction     func() tea.Cmd
}

type TickMsg time.Time

func (m *CountdownModel) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *CountdownModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		if m.SecondsRemaining > 0 {
			m.SecondsRemaining--
			return m, tick()
		}
		// Time's up!
		return m, m.NextAction()

	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter", " ":
			// Play immediately
			return m, m.NextAction()
		case "n", "esc", "q", "ctrl+c":
			// Cancel
			return m, m.CancelAction()
		}
	}
	return m, nil
}

func (m CountdownModel) View() string {
	title := shared.StyleTitle.Render("⏳ Play Queue")

	content := fmt.Sprintf("\nNext up: %s\n\nStarting in %d seconds...",
		lipgloss.NewStyle().Bold(true).Foreground(shared.ColorPlexOrange).Render(m.NextTitle),
		m.SecondsRemaining)

	help := lipgloss.NewStyle().Foreground(shared.ColorLightGrey).Render("\n(Enter=Play Now, Esc=Cancel)")

	return shared.StyleBorder.Render(lipgloss.JoinVertical(lipgloss.Center,
		title,
		content,
		help,
	))
}
