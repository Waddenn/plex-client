package dashboard

import (
	"fmt"
	"strings"

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
		width:         80,
		height:        24,
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
				if m.sidebarCursor < 2 { // Movies, Series, Settings
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
				case 0: // Movies
					return m, func() tea.Msg { return shared.MsgSwitchView{View: shared.ViewMovieBrowser} }
				case 1: // Series
					return m, func() tea.Msg { return shared.MsgSwitchView{View: shared.ViewSeriesBrowser} }
				case 2: // Settings
					return m, func() tea.Msg { return shared.MsgSwitchView{View: shared.ViewSettings} }
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

func (m *Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, "Loading...")
	}

	// --- 1. Layout dims ---
	availableWidth := m.width
	if availableWidth < 20 {
		availableWidth = 20
	}
	availableHeight := m.height
	if availableHeight < 10 {
		availableHeight = 10
	}

	// --- 2. Render Header ---
	header := shared.StyleHeader.Copy().Width(availableWidth).Render("🏠 Dashboard")

	// --- 3. Render Footer ---
	help := "[←/→] Focus • [↑/↓] Navigate • [Enter] Open • [Q] Quit"
	space := availableWidth - lipgloss.Width(help) - 2
	footerContent := help
	if space > 0 {
		footerContent = strings.Repeat(" ", space) + help
	}
	footer := shared.StyleFooter.Copy().Width(availableWidth).Render(footerContent)

	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	contentHeight := m.height - headerHeight - footerHeight
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Layout components
	sidebar := m.renderSidebar(contentHeight)
	contentWidth := m.width - lipgloss.Width(sidebar)
	if contentWidth < 20 {
		contentWidth = 20
	}
	content := m.renderContent(contentWidth, contentHeight)

	// Combine Horizontal
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *Model) renderSidebar(height int) string {
	items := []string{"🎬 Movies", "📺 TV Series", "⚙️ Settings"}

	var renderedItems []string

	for i, item := range items {
		style := shared.StyleItemNormal
		prefix := "  "

		// If sidebar is active, show highlight
		if m.activeColumn == 0 && m.sidebarCursor == i {
			style = shared.StyleItemActive.Copy().Width(shared.SidebarWidth - 2)
			prefix = "> "
		}

		renderedItems = append(renderedItems, style.Render(prefix+item))
	}

	// Add padding/spacing
	list := lipgloss.JoinVertical(lipgloss.Left, renderedItems...)

	sidebarStyle := shared.StyleSidebar.Copy()
	if height > 0 {
		sidebarStyle = sidebarStyle.Height(height)
	}
	return sidebarStyle.Render(list)
}

func (m *Model) renderContent(width int, height int) string {
	if len(m.onDeck) == 0 {
		return shared.StyleDim.Render("No content active.")
	}

	selected := m.onDeck[0]
	if m.contentCursor >= 0 && m.contentCursor < len(m.onDeck) {
		selected = m.onDeck[m.contentCursor]
	}

	// 1. Continue Watching (Index 0)
	hero := m.renderHeroLine(m.onDeck[0], m.activeColumn == 1 && m.contentCursor == 0, width)

	// 2. Up Next List (Index 1+)
	list := ""
	if len(m.onDeck) > 1 {
		list = m.renderList(m.onDeck[1:], width)
	}

	leftBody := lipgloss.JoinVertical(lipgloss.Left,
		shared.StyleTitle.Render("Continue Watching"),
		hero,
		"",
		shared.StyleTitle.Render("Up Next"),
		list,
	)

	if width > 80 {
		leftWidth := int(float64(width) * 0.45)
		if leftWidth < 30 {
			leftWidth = 30
		}
		rightWidth := width - leftWidth

		// Re-calculate hero and list with correct width
		hero = m.renderHeroLine(m.onDeck[0], m.activeColumn == 1 && m.contentCursor == 0, leftWidth)
		list = ""
		if len(m.onDeck) > 1 {
			list = m.renderList(m.onDeck[1:], leftWidth)
		}

		leftBody = lipgloss.JoinVertical(lipgloss.Left,
			shared.StyleTitle.Render("Continue Watching"),
			hero,
			"",
			shared.StyleTitle.Render("Up Next"),
			list,
		)

		left := lipgloss.NewStyle().Width(leftWidth).Render(leftBody)
		right := m.renderDetailsPanel(selected, rightWidth, height)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	container := lipgloss.NewStyle().Width(width)
	if height > 0 {
		container = container.Height(height)
	}
	return container.Render(leftBody)
}

func (m Model) renderHeroLine(item plex.Video, active bool, width int) string {
	prefix := "  "
	style := shared.StyleItemNormal
	if active {
		prefix = "> "
		style = shared.StyleItemActive
	}

	title := item.Title
	if item.Type == "episode" {
		title = fmt.Sprintf("%s - S%02dE%02d", item.GrandparentTitle, item.ParentIndex, item.Index)
	}

	prog := "Ready to play"
	if item.ViewOffset > 0 && item.Duration > 0 {
		percent := int((float64(item.ViewOffset) / float64(item.Duration)) * 100)
		prog = fmt.Sprintf("%d%% watched", percent)
	}

	line := fmt.Sprintf("%s%s • %s", prefix, title, prog)

	maxLen := width - 2
	if maxLen < 10 {
		maxLen = 10
	}

	rowStyle := style.Copy().MaxHeight(1)
	if width > 0 {
		rowStyle = rowStyle.Width(width)
	}

	return rowStyle.Render(shared.Truncate(line, maxLen))
}

func (m Model) renderList(items []plex.Video, width int) string {
	var rows []string

	for i, item := range items {
		// Real index in onDeck is i + 1
		// contentCursor matches onDeck index
		isActive := (m.activeColumn == 1 && m.contentCursor == i+1)

		prefix := "  "
		style := shared.StyleItemNormal
		if isActive {
			prefix = "> "
			style = shared.StyleItemActive
		}

		title := item.Title
		if item.Type == "episode" {
			title = fmt.Sprintf("%s - S%02dE%02d", item.GrandparentTitle, item.ParentIndex, item.Index)
		}

		maxLen := width - 4
		if maxLen < 10 {
			maxLen = 10
		}

		line := fmt.Sprintf("%s%s", prefix, title)

		rowStyle := style.Copy().MaxHeight(1)
		if width > 0 {
			rowStyle = rowStyle.Width(width)
		}
		row := rowStyle.Render(shared.Truncate(line, maxLen))
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderDetailsPanel(item plex.Video, width int, height int) string {
	if width < 20 {
		return ""
	}

	title := shared.StyleHighlight.Render(item.Title)
	subtitle := ""
	if item.Type == "episode" {
		subtitle = fmt.Sprintf("%s • S%02dE%02d", item.GrandparentTitle, item.ParentIndex, item.Index)
	} else if item.Year > 0 {
		subtitle = fmt.Sprintf("%d", item.Year)
	}

	prog := "Ready to play"
	if item.ViewOffset > 0 && item.Duration > 0 {
		percent := int((float64(item.ViewOffset) / float64(item.Duration)) * 100)
		prog = fmt.Sprintf("%d%% watched", percent)
	}

	summary := item.Summary
	if summary == "" {
		summary = "No summary available."
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		shared.StyleDim.Render(subtitle),
		"",
		shared.StyleSecondary.Render(prog),
		"",
		lipgloss.NewStyle().Width(width-4).Foreground(lipgloss.Color("#cccccc")).Render(summary),
	)

	panel := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#333333")).
		Padding(0, 2).
		Render(content)

	if height > 0 {
		return lipgloss.NewStyle().Height(height).Render(panel)
	}
	return panel
}
