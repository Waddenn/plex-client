package browser

import (
	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/store"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Mode int

const (
	ModeSections Mode = iota
	ModeItems
	ModeSeasons
	ModeEpisodes
)

type SortMethod int

const (
	SortTitle SortMethod = iota
	SortYear
	SortRating
	SortDateAdded
)

func (s SortMethod) String() string {
	switch s {
	case SortTitle:
		return "Title"
	case SortYear:
		return "Year"
	case SortRating:
		return "Rating"
	case SortDateAdded:
		return "Recently Added"
	default:
		return "Unknown"
	}
}

type Model struct {
	plexClient *plex.Client
	store      *store.Store
	width      int
	height     int

	mode Mode

	// Data
	sections []plex.Directory
	items    []plex.Video

	// For drill-down
	seasons  []plex.Directory
	episodes []plex.Video

	cursor  int
	loading bool

	// Filter
	targetType string // "movie" or "show"

	// Search
	textInput  textinput.Model
	showSearch bool

	// Sorting
	sortMethod SortMethod

	// Cache
	filteredList []interface{}
	needsRefresh bool
}

func NewModel(p *plex.Client, s *store.Store) Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 156
	ti.Width = 30

	return Model{
		plexClient:   p,
		store:        s,
		width:        80, // Sensible defaults
		height:       24,
		loading:      false,
		mode:         ModeSections,
		textInput:    ti,
		needsRefresh: true,
	}
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

// SetType allows the main model to configure this browser before switching to it
func (m *Model) SetType(t string) tea.Cmd {
	m.targetType = t
	m.mode = ModeSections
	m.loading = true
	m.cursor = 0
	m.showSearch = false
	m.textInput.Reset()
	m.needsRefresh = true

	// Two-stage loading for sections is usually fast, but let's stick to sections for now
	return fetchSections(m.plexClient, t)
}
