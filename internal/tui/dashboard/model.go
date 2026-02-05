package dashboard

import (
	"fmt"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	plexClient *plex.Client
	width      int
	height     int
	onDeck     []plex.Video
	loading    bool

	// Navigation
	// activeColumn: 0 = Sidebar, 1 = Content
	activeColumn int

	// sidebarCursor: 0 = Home (refresh), 1 = Movies, 2 = Series
	sidebarCursor int

	// contentCursor: 0 = Hero, 1+ = List items
	contentCursor int
}

func NewModel(p *plex.Client) Model {
	return Model{
		plexClient:    p,
		loading:       true,
		activeColumn:  1, // Start on Content (Hero)
		sidebarCursor: 0,
		contentCursor: 0,
	}
}

type MsgOnDeckLoaded struct {
	Items []plex.Video
	Err   error
}

func fetchOnDeck(p *plex.Client) tea.Cmd {
	return func() tea.Msg {
		sections, err := p.GetSections()
		if err != nil {
			return MsgOnDeckLoaded{Err: err}
		}
		var all []plex.Video
		for _, s := range sections {
			vids, err := p.GetOnDeck(s.Key)
			if err == nil {
				all = append(all, vids...)
			}
		}
		// Separate movies and episodes, then interleave for variety
		var movies, episodes []plex.Video
		for _, v := range all {
			if v.Type == "movie" {
				movies = append(movies, v)
			} else if v.Type == "episode" {
				episodes = append(episodes, v)
			}
		}

		// Interleave: movie, episode, movie, episode...
		var mixed []plex.Video
		maxLen := len(movies)
		if len(episodes) > maxLen {
			maxLen = len(episodes)
		}
		for i := 0; i < maxLen && len(mixed) < 8; i++ {
			if i < len(movies) {
				mixed = append(mixed, movies[i])
			}
			if i < len(episodes) && len(mixed) < 8 {
				mixed = append(mixed, episodes[i])
			}
		}

		return MsgOnDeckLoaded{Items: mixed}
	}
}

func (m Model) Init() tea.Cmd {
	return fetchOnDeck(m.plexClient)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.activeColumn == 0 {
				if m.sidebarCursor > 0 {
					m.sidebarCursor--
				}
			} else {
				if m.contentCursor > 0 {
					m.contentCursor--
				}
			}

		case "down", "j":
			if m.activeColumn == 0 {
				if m.sidebarCursor < 2 { // Home, Movies, Series
					m.sidebarCursor++
				}
			} else {
				// Hero (0) + OnDeck List
				maxCursor := len(m.onDeck) - 1
				if m.contentCursor < maxCursor {
					m.contentCursor++
				}
			}

		case "left", "h":
			if m.activeColumn == 1 {
				m.activeColumn = 0 // Switch to Sidebar
			}

		case "right", "l":
			if m.activeColumn == 0 {
				m.activeColumn = 1 // Switch to Content
			}

		case "enter":
			if m.loading {
				return m, nil
			}

			if m.activeColumn == 0 {
				// Sidebar Actions
				switch m.sidebarCursor {
				case 0: // Home - Refresh
					m.loading = true
					return m, fetchOnDeck(m.plexClient)
				case 1: // Movies
					return m, func() tea.Msg { return shared.MsgSwitchView{View: shared.ViewMovieBrowser} }
				case 2: // Series
					return m, func() tea.Msg { return shared.MsgSwitchView{View: shared.ViewSeriesBrowser} }
				}
			} else {
				// Content Actions
				if len(m.onDeck) > 0 {
					// contentCursor maps directly to onDeck index
					// 0 = Hero (first item), 1+ = List items (subsequent items)
					// Actually, let's keep it simple: 0 is Hero (onDeck[0]), 1 is onDeck[1], etc.
					if m.contentCursor < len(m.onDeck) {
						return m, func() tea.Msg { return shared.MsgPlayVideo{Video: m.onDeck[m.contentCursor]} }
					}
				}
			}
		}

	case MsgOnDeckLoaded:
		m.loading = false
		if msg.Err != nil {
			// handle error
		} else {
			m.onDeck = msg.Items
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, "Loading...")
	}

	// Layout components
	sidebar := m.renderSidebar()
	content := m.renderContent()

	// Combine Horizontal
	// Sidebar | Content

	return shared.StyleBorder.Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			sidebar,
			content,
		),
	)
}

func (m Model) renderSidebar() string {
	items := []string{"🏠 Dashboard", "🎬 Movies", "📺 TV Series"}

	var renderedItems []string

	for i, item := range items {
		style := shared.StyleItemNormal
		prefix := "  "

		// If sidebar is active, show highlight
		if m.activeColumn == 0 && m.sidebarCursor == i {
			style = shared.StyleItemActive
			prefix = "│ "
		} else if m.sidebarCursor == i {
			// Show selection but dim if not active column?
			// Or just show plain if not active
		}

		renderedItems = append(renderedItems, style.Render(prefix+item))
	}

	// Add padding/spacing
	list := lipgloss.JoinVertical(lipgloss.Left, renderedItems...)

	return shared.StyleSidebar.Render(list)
}

func (m Model) renderContent() string {
	if len(m.onDeck) == 0 {
		return "No content active."
	}

	// 1. Hero (Index 0)
	hero := m.renderHero(m.onDeck[0], m.activeColumn == 1 && m.contentCursor == 0)

	// 2. Up Next List (Index 1+)
	list := ""
	if len(m.onDeck) > 1 {
		list = m.renderList(m.onDeck[1:])
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		hero,
		" ",
		shared.StyleTitle.Render("Up Next"),
		list,
	)
}

func (m Model) renderHero(item plex.Video, active bool) string {
	// Dynamically size hero?
	// For now, let's make it fixed width or fill available
	// The sidebar takes ~25 chars + padding.

	style := shared.StyleHero.Copy().Width(60) // Fixed width for now to look good
	if active {
		style = style.BorderForeground(shared.ColorPlexOrange)
	} else {
		style = style.BorderForeground(shared.ColorDarkGrey)
	}

	title := shared.StyleHighlight.Render(item.Title)
	if item.Type == "episode" {
		title = fmt.Sprintf("%s\n%s",
			shared.StyleHighlight.Render(item.GrandparentTitle),
			shared.StyleDim.Render(fmt.Sprintf("S%02dE%02d - %s", item.ParentIndex, item.Index, item.Title)))
	}

	// Progress
	prog := ""
	if item.ViewOffset > 0 && item.Duration > 0 {
		percent := int((float64(item.ViewOffset) / float64(item.Duration)) * 100)
		prog = shared.StyleSecondary.Render(fmt.Sprintf("%d%% watched", percent))
	} else {
		prog = shared.StyleSecondary.Render("Ready to play")
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		" ▶ Continue Watching ",
		" ",
		title,
		" ",
		prog,
	)

	return style.Render(content)
}

func (m Model) renderList(items []plex.Video) string {
	var rows []string

	for i, item := range items {
		// Real index in onDeck is i + 1
		// contentCursor matches onDeck index
		isActive := (m.activeColumn == 1 && m.contentCursor == i+1)

		prefix := "  "
		style := shared.StyleItemNormal
		if isActive {
			prefix = "│ "
			style = shared.StyleItemActive
		}

		title := item.Title
		if item.Type == "episode" {
			title = fmt.Sprintf("%s - S%02dE%02d", item.GrandparentTitle, item.ParentIndex, item.Index)
		}

		row := style.Render(fmt.Sprintf("%s%s", prefix, title))
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
