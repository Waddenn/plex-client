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

	// Navigation context
	selectedShowTitle string // Title of the selected show (for breadcrumbs)

	// Error handling
	errorMsg string

	// Cache
	filteredList []interface{}
	needsRefresh bool

	// Sync State
	SyncStatus string
	AutoSync   bool

	// UI Config
	StatusIndicatorStyle string
}

func NewModel(p *plex.Client, s *store.Store, autoSync bool, statusIndicatorStyle string) Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 156
	ti.Width = 30

	return Model{
		plexClient:           p,
		store:                s,
		width:                80, // Sensible defaults
		height:               24,
		loading:              false,
		mode:                 ModeSections,
		textInput:            ti,
		needsRefresh:         true,
		AutoSync:             autoSync,
		StatusIndicatorStyle: statusIndicatorStyle,
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
	m.filteredList = nil

	// Clear data slices to prevent "relics" from previous state
	m.sections = nil
	m.items = nil
	m.seasons = nil
	m.episodes = nil
	m.errorMsg = ""

	var cmds []tea.Cmd
	dbSections, err := fetchSectionsFromStore(m.store, t)
	if err != nil {
		m.errorMsg = "Database error: " + err.Error()
		m.loading = false
		return nil
	}

	if len(dbSections) > 0 {
		m.sections = dbSections
		m.loading = false

		// UX Improvement: If only one section, auto-select it
		if len(m.sections) == 1 {
			section := m.sections[0]
			m.mode = ModeItems
			m.loading = true
			dbItems, err := fetchLibraryItemsFromStore(m.store, m.targetType)
			if err != nil {
				m.errorMsg = "Database error: " + err.Error()
				m.loading = false
				return nil
			}

			if len(dbItems) > 0 {
				m.items = dbItems
				m.loading = false
			} else if !m.AutoSync {
				m.loading = false
				m.errorMsg = "Library has not been synced yet. Please enable Background Sync or run a manual sync (r)."
			}

			if m.AutoSync {
				cmds = append(cmds, fetchLibraryItems(m.plexClient, section.Key))
			} else {
				// SQL only, ensure we don't stall in loading state
				m.loading = false
			}
		}
	} else if !m.AutoSync {
		// Nothing in DB and AutoSync is off -> nowhere to get data
		m.loading = false
		m.errorMsg = "No cached libraries found. Please enable Background Sync or run a manual sync (r)."
	}

	if m.AutoSync {
		cmds = append(cmds, fetchSections(m.plexClient, t))
	}

	return tea.Batch(cmds...)
}
