package tui

import (
	"database/sql"
	"fmt"

	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/player"
	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/browser"
	"github.com/Waddenn/plex-client/internal/tui/dashboard"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MainModel struct {
	cfg        *config.Config
	db         *sql.DB
	plexClient *plex.Client

	width  int
	height int

	currentView shared.View

	// Sub-models
	dashboard dashboard.Model
	browser   *browser.Model
	countdown CountdownModel

	// Play Queue State
	playQueue []plex.Video
	queueIdx  int
}

func NewModel(db *sql.DB, cfg *config.Config, p *plex.Client) MainModel {
	bm := browser.NewModel(p, db)
	return MainModel{
		cfg:         cfg,
		db:          db,
		plexClient:  p,
		currentView: shared.ViewDashboard,
		dashboard:   dashboard.NewModel(p),
		browser:     &bm,
	}
}

func (m *MainModel) Init() tea.Cmd {
	return m.dashboard.Init()
}

// MsgQueueLoaded is returned when a Play Queue is fetched
type MsgQueueLoaded struct {
	Queue []plex.Video
	Index int // Index to start playing
}

// MsgPlaybackFinished indicates player exited
type MsgPlaybackFinished struct {
	Completed bool
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keys
		if m.currentView != shared.ViewCountdown { // Countdown handles its own keys
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// Propagate window size to ALL submodels
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		m.dashboard, _ = m.dashboard.Update(msg)
		cmd = m.browser.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case shared.MsgSwitchView:
		m.currentView = msg.View

		// Handle init logic for specific views
		if msg.View == shared.ViewMovieBrowser {
			return m, m.browser.SetType("movie")
		} else if msg.View == shared.ViewSeriesBrowser {
			return m, m.browser.SetType("show")
		}
		return m, nil

	case shared.MsgBack:
		if m.currentView != shared.ViewDashboard {
			// Clear queue state
			m.playQueue = nil
			m.queueIdx = 0

			m.currentView = shared.ViewDashboard
			// Maybe refresh dashboard?
			return m, m.dashboard.Init()
		}

	case shared.MsgPlayVideo:
		// Assert video type
		v, ok := msg.Video.(plex.Video)
		if !ok {
			m.currentView = shared.ViewDashboard
			return m, nil
		}

		// For episodes, fetch/create Play Queue
		if v.Type == "episode" {
			m.currentView = shared.ViewPlayer // Show placeholder
			return m, fetchPlayQueue(m.plexClient, v)
		}

		// For movies, play single video directly
		// Find media part
		if len(v.Media) == 0 || len(v.Media[0].Part) == 0 {
			// No media found
			m.currentView = shared.ViewDashboard
			return m, nil
		}
		partKey := v.Media[0].Part[0].Key
		playbackURL := m.plexClient.BaseURL + partKey

		m.currentView = shared.ViewPlayer
		// Run player in a command
		return m, func() tea.Msg {
			completed, err := player.Play(v.Title, playbackURL, v.RatingKey, int64(v.ViewOffset), m.cfg, m.plexClient)
			if err != nil {
				return shared.MsgError{Err: err}
			}
			return MsgPlaybackFinished{Completed: completed}
		}

	case MsgQueueLoaded:
		m.playQueue = msg.Queue
		m.queueIdx = msg.Index
		return m, m.playCurrentQueueItem()

	case MsgPlaybackFinished:
		// Logic to determine what to do next
		if len(m.playQueue) > 0 && m.queueIdx < len(m.playQueue)-1 && msg.Completed {
			// Proceed to Countdown
			nextItem := m.playQueue[m.queueIdx+1]

			title := nextItem.Title
			if nextItem.GrandparentTitle != "" {
				title = fmt.Sprintf("%s - %s", nextItem.GrandparentTitle, nextItem.Title)
			}

			m.currentView = shared.ViewCountdown
			m.countdown = CountdownModel{
				SecondsRemaining: 3, // 3 seconds countdown (matches old behavior)
				NextTitle:        title,
				NextAction: func() tea.Cmd {
					m.queueIdx++
					return func() tea.Msg { return MsgPlayNext{} }
				},
				CancelAction: func() tea.Cmd {
					return func() tea.Msg { return shared.MsgBack{} }
				},
			}
			return m, m.countdown.Init()
		}

		// If finished or no queue, go back
		return m, func() tea.Msg { return shared.MsgBack{} }

	case MsgPlayNext:
		// Triggered by countdown completion
		return m, m.playCurrentQueueItem()
	}

	// Update active submodel
	switch m.currentView {
	case shared.ViewDashboard:
		newModel, newCmd := m.dashboard.Update(msg)
		m.dashboard = newModel
		cmd = newCmd
	case shared.ViewMovieBrowser, shared.ViewSeriesBrowser:
		cmd = m.browser.Update(msg)
	case shared.ViewCountdown:
		newModel, newCmd := m.countdown.Update(msg)
		m.countdown = *newModel.(*CountdownModel)
		cmd = newCmd
	}

	return m, cmd
}

// MsgPlayNext is a signal to play the next item in queue
type MsgPlayNext struct{}

func (m MainModel) playCurrentQueueItem() tea.Cmd {
	if m.queueIdx < 0 || m.queueIdx >= len(m.playQueue) {
		return func() tea.Msg { return shared.MsgBack{} }
	}

	item := m.playQueue[m.queueIdx]
	if len(item.Media) == 0 || len(item.Media[0].Part) == 0 {
		// Skip or error?
		return func() tea.Msg { return MsgPlaybackFinished{Completed: true} } // Skip
	}

	partKey := item.Media[0].Part[0].Key
	title := item.Title
	if item.Type == "episode" && item.GrandparentTitle != "" {
		title = fmt.Sprintf("%s - S%02dE%02d - %s", item.GrandparentTitle, item.ParentIndex, item.Index, item.Title)
	}

	playbackURL := m.plexClient.BaseURL + partKey

	// Determine start time (resume) only if it's the item we started with?
	// The queue items from PMS usually have viewOffset if partially watched.
	startTime := int64(item.ViewOffset)

	return func() tea.Msg {
		completed, err := player.Play(title, playbackURL, item.RatingKey, startTime, m.cfg, m.plexClient)
		if err != nil {
			return shared.MsgError{Err: err}
		}
		return MsgPlaybackFinished{Completed: completed}
	}
}

func (m *MainModel) View() string {
	switch m.currentView {
	case shared.ViewDashboard:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.dashboard.View())
	case shared.ViewMovieBrowser, shared.ViewSeriesBrowser:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.browser.View())
	case shared.ViewPlayer:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, shared.StyleTitle.Render("▶ Playing Video..."))
	case shared.ViewCountdown:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.countdown.View())
	}

	return "Unknown View"
}
