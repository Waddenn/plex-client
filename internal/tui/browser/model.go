package browser

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	db         *sql.DB
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

func NewModel(p *plex.Client, db *sql.DB) Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 156
	ti.Width = 30

	return Model{
		plexClient:   p,
		db:           db,
		loading:      false,
		mode:         ModeSections,
		textInput:    ti,
		needsRefresh: true,
	}
}

// Messages
type MsgSectionsLoaded struct {
	Sections []plex.Directory
	Err      error
}

type MsgItemsLoaded struct {
	Items []plex.Video
	Dirs  []plex.Directory
	Err   error
}

type MsgChildrenLoaded struct {
	Dirs   []plex.Directory
	Videos []plex.Video
	Err    error
}

func fetchLibraryItems(p *plex.Client, key string) tea.Cmd {
	return func() tea.Msg {
		dirs, videos, err := p.GetSectionAll(key)
		if err != nil {
			return MsgItemsLoaded{Err: err}
		}
		return MsgItemsLoaded{Items: videos, Dirs: dirs}
	}
}

func fetchChildren(p *plex.Client, key string) tea.Cmd {
	return func() tea.Msg {
		dirs, vids, err := p.GetChildren(key)
		if err != nil {
			return MsgChildrenLoaded{Err: err}
		}
		return MsgChildrenLoaded{Dirs: dirs, Videos: vids}
	}
}

func fetchSections(p *plex.Client, targetType string) tea.Cmd {
	return func() tea.Msg {
		all, err := p.GetSections()
		if err != nil {
			return MsgSectionsLoaded{Err: err}
		}
		var filtered []plex.Directory
		for _, s := range all {
			if s.Type == targetType {
				filtered = append(filtered, s)
			}
		}
		return MsgSectionsLoaded{Sections: filtered}
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

func fetchLibraryItemsFromDB(db *sql.DB, targetType string) ([]plex.Video, error) {
	var videos []plex.Video
	var query string
	if targetType == "movie" {
		query = `SELECT id, title, year, part_key, duration, summary, rating, genres, originallyAvailableAt, content_rating, studio, added_at FROM films`
	} else {
		query = `SELECT id, title, summary, rating, genres, content_rating, studio, added_at FROM series`
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var v plex.Video
		var genres string
		if targetType == "movie" {
			if err := rows.Scan(&v.RatingKey, &v.Title, &v.Year, &v.Key, &v.Duration, &v.Summary, &v.Rating, &genres, &v.OriginallyAvailableAt, &v.ContentRating, &v.Studio, &v.AddedAt); err != nil {
				return nil, err
			}
			v.Type = "movie"
		} else {
			if err := rows.Scan(&v.RatingKey, &v.Title, &v.Summary, &v.Rating, &genres, &v.ContentRating, &v.Studio, &v.AddedAt); err != nil {
				return nil, err
			}
			v.Type = "show"
		}

		// Parse genres back into Tags
		if genres != "" {
			for _, g := range strings.Split(genres, ", ") {
				v.Genre = append(v.Genre, plex.Tag{Tag: g})
			}
		}

		videos = append(videos, v)
	}
	return videos, nil
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.needsRefresh = true
		return nil

	case tea.KeyMsg:
		// If search is active, pass input to textinput
		if m.showSearch {
			switch msg.String() {
			case "enter":
				// Commit search (optional, or just keep filtering live)
				// For now, enter just stays in search mode but maybe we want to select the first item?
				// Let's say enter selects the currently cursor-highlighted item even if searching
				// But we need to make sure we don't consume 'enter' here if we want to select
				// Actually, usually enter confirms the filter or selects.
				// Let's make "esc" clear search provided it was active.
			case "esc":
				m.showSearch = false
				m.textInput.Reset()
				m.cursor = 0
				return nil
			}
			var tiCmd tea.Cmd
			m.textInput, tiCmd = m.textInput.Update(msg)

			// Reset cursor on search change
			if msg.String() != "enter" && msg.String() != "up" && msg.String() != "down" {
				m.cursor = 0
				m.needsRefresh = true
			}

			// If user presses Down/Up while searching, we should allow navigation in the filtered list
			// But textinput consumes arrows. We need to check if we should override.
			// Textinput doesn't usually consume Up/Down unless multiline.
			if msg.String() == "up" || msg.String() == "down" || msg.String() == "enter" {
				// Fallthrough to normal navigation
			} else {
				return tiCmd
			}
		}

		switch msg.String() {
		case "/":
			if !m.showSearch {
				m.showSearch = true
				m.textInput.Focus()
				return textinput.Blink
			}

		case "s": // Cycle sort
			if !m.showSearch {
				m.sortMethod = (m.sortMethod + 1)
				if m.sortMethod > SortDateAdded {
					m.sortMethod = SortTitle
				}
				m.needsRefresh = true
				return nil
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			count := m.getFilteredCount()
			if m.cursor < count-1 {
				m.cursor++
			}
		case "esc", "backspace":
			if m.showSearch {
				m.showSearch = false
				m.textInput.Reset()
				m.cursor = 0
				return nil
			}

			if m.mode == ModeItems {
				// If only 1 section, going back means exiting the browser completely
				if len(m.sections) == 1 {
					return func() tea.Msg { return shared.MsgBack{} }
				}
				m.mode = ModeSections
				m.cursor = 0
				m.showSearch = false
				m.textInput.Reset()
				m.needsRefresh = true
				return nil
			} else if m.mode == ModeSeasons {
				m.mode = ModeItems
				m.cursor = 0
				m.showSearch = false
				m.textInput.Reset()
				m.needsRefresh = true
				return nil
			} else if m.mode == ModeEpisodes {
				// For mini-series that skipped the season selection, go back to items
				if len(m.seasons) == 0 {
					m.mode = ModeItems
				} else {
					m.mode = ModeSeasons
				}
				m.cursor = 0
				m.showSearch = false
				m.textInput.Reset()
				m.needsRefresh = true
				return nil
			}
			// If at root, let MainModel handle it (MsgBack)
			return func() tea.Msg { return shared.MsgBack{} }

		case "enter":
			filteredList := m.getFilteredList()
			if m.cursor < len(filteredList) {
				selected := filteredList[m.cursor]

				switch item := selected.(type) {
				case plex.Directory: // Section or Season
					if m.mode == ModeSections {
						m.mode = ModeItems
						m.loading = true
						m.cursor = 0
						m.showSearch = false // Reset search when drilling down
						m.textInput.Reset()
						m.needsRefresh = true

						// Instant load from DB
						var cmds []tea.Cmd
						if dbItems, err := fetchLibraryItemsFromDB(m.db, m.targetType); err == nil && len(dbItems) > 0 {
							m.items = dbItems
							m.loading = false // Hide loader if we have data
						}
						cmds = append(cmds, fetchLibraryItems(m.plexClient, item.Key))
						return tea.Batch(cmds...)
					} else if m.mode == ModeSeasons {
						m.mode = ModeEpisodes
						m.loading = true
						m.cursor = 0
						m.showSearch = false
						m.textInput.Reset()
						m.needsRefresh = true
						return fetchChildren(m.plexClient, item.RatingKey)
					}
				case plex.Video: // Item or Episode
					if m.mode == ModeItems {
						if item.Type == "show" {
							m.mode = ModeSeasons
							m.loading = true
							m.cursor = 0
							m.showSearch = false
							m.textInput.Reset()
							m.needsRefresh = true
							return fetchChildren(m.plexClient, item.RatingKey)
						}
						return func() tea.Msg { return shared.MsgPlayVideo{Video: item} }
					} else if m.mode == ModeEpisodes {
						return func() tea.Msg { return shared.MsgPlayVideo{Video: item} }
					}
				}
			}
		}

	case MsgSectionsLoaded:
		m.loading = false
		if msg.Err == nil {
			m.sections = msg.Sections
			// UX Improvement: If only one section, auto-select it
			if len(m.sections) == 1 {
				section := m.sections[0]
				m.mode = ModeItems
				m.loading = true
				m.needsRefresh = true

				var cmds []tea.Cmd
				if dbItems, err := fetchLibraryItemsFromDB(m.db, m.targetType); err == nil && len(dbItems) > 0 {
					m.items = dbItems
					m.loading = false
				}
				cmds = append(cmds, fetchLibraryItems(m.plexClient, section.Key))
				return tea.Batch(cmds...)
			}
		}
	case MsgItemsLoaded:
		m.loading = false
		m.needsRefresh = true
		if msg.Err == nil {
			if len(msg.Dirs) > 0 && len(msg.Items) == 0 {
				// It's a list of Shows
				var converted []plex.Video
				for _, d := range msg.Dirs {
					converted = append(converted, plex.Video{
						Title:         d.Title,
						Key:           d.Key,
						RatingKey:     d.RatingKey,
						Summary:       d.Summary,
						Type:          "show",
						Year:          d.Year,
						Rating:        d.Rating,
						Genre:         d.Genre,
						Director:      d.Director,
						ContentRating: d.ContentRating,
						Studio:        d.Studio,
						Role:          d.Role,
						AddedAt:       d.AddedAt,
					})
				}
				m.items = converted
			} else {
				m.items = msg.Items
			}
		}
	case MsgChildrenLoaded:
		m.loading = false
		m.needsRefresh = true
		if msg.Err == nil {
			m.seasons = msg.Dirs
			m.episodes = msg.Videos

			// Auto-Switch Logic:
			if m.mode == ModeSeasons {
				if len(m.seasons) == 0 && len(m.episodes) > 0 {
					m.mode = ModeEpisodes
				}
			}
		}
	}

	return cmd
}

// Helper methods for filtering

func (m Model) getFilteredCount() int {
	return len(m.getFilteredList())
}

func (m *Model) getFilteredList() []interface{} {
	if !m.needsRefresh && m.filteredList != nil {
		return m.filteredList
	}

	filter := strings.ToLower(m.textInput.Value())
	var result []interface{}

	switch m.mode {
	case ModeSections:
		for _, s := range m.sections {
			if filter == "" || strings.Contains(strings.ToLower(s.Title), filter) {
				result = append(result, s)
			}
		}
	case ModeItems:
		result = filterAndSortVideos(m.items, filter, m.sortMethod)
	case ModeSeasons:
		for _, s := range m.seasons {
			if filter == "" || strings.Contains(strings.ToLower(s.Title), filter) {
				result = append(result, s)
			}
		}
	case ModeEpisodes:
		result = filterAndSortVideos(m.episodes, filter, m.sortMethod)
	}

	m.filteredList = result
	m.needsRefresh = false
	return result
}

func filterAndSortVideos(videos []plex.Video, filter string, sortMethod SortMethod) []interface{} {
	var filtered []plex.Video
	for _, v := range videos {
		match := false
		if filter == "" {
			match = true
		} else {
			lowTitle := strings.ToLower(v.Title)
			lowSummary := strings.ToLower(v.Summary)
			if strings.Contains(lowTitle, filter) || strings.Contains(lowSummary, filter) {
				match = true
			} else {
				// Also search in director names
				for _, d := range v.Director {
					if strings.Contains(strings.ToLower(d.Tag), filter) {
						match = true
						break
					}
				}
			}
		}

		if match {
			filtered = append(filtered, v)
		}
	}

	// Sort using sort.Slice (O(n log n))
	sort.Slice(filtered, func(i, j int) bool {
		switch sortMethod {
		case SortYear:
			if filtered[i].Year != filtered[j].Year {
				return filtered[i].Year > filtered[j].Year // Newest first
			}
		case SortRating:
			if filtered[i].Rating != filtered[j].Rating {
				return filtered[i].Rating > filtered[j].Rating // Best first
			}
		case SortDateAdded:
			if filtered[i].AddedAt != filtered[j].AddedAt {
				return filtered[i].AddedAt > filtered[j].AddedAt // Newest first
			}
		}
		// Default: Title
		return strings.ToLower(filtered[i].Title) < strings.ToLower(filtered[j].Title)
	})

	var final []interface{}
	for _, v := range filtered {
		final = append(final, v)
	}
	return final
}

func (m *Model) View() string {
	var content string

	// --- 1. Dynamic Breadcrumb Header ---
	var breadcrumb string
	switch m.mode {
	case ModeSections:
		breadcrumb = fmt.Sprintf("📂 %s", shared.StyleHighlight.Render("Library"))
	case ModeItems:
		title := "Movies"
		if m.targetType == "show" {
			title = "Series"
		}
		breadcrumb = fmt.Sprintf("📂 Library > %s", shared.StyleHighlight.Render(title))
	case ModeSeasons:
		breadcrumb = fmt.Sprintf("📂 Library > Series > %s", shared.StyleHighlight.Render("Seasons"))
	case ModeEpisodes:
		breadcrumb = fmt.Sprintf("📂 Library > Series > Seasons > %s", shared.StyleHighlight.Render("Episodes"))
	}

	headerView := ""
	if m.showSearch {
		headerView = fmt.Sprintf("🔍 %s", m.textInput.View())
	} else {
		headerView = breadcrumb
		if m.textInput.Value() != "" {
			headerView += shared.StyleDim.Render(fmt.Sprintf(" [Filter: %s]", m.textInput.Value()))
		}
	}

	headerStyle := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(shared.ColorPlexOrange)

	renderedHeader := headerStyle.Render(headerView)

	// Layout dims - Use full width/height
	availableWidth := m.width
	if availableWidth < 20 { // Minimum width safety
		availableWidth = 20
	}

	availableHeight := m.height
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Calculate heights
	headerHeight := 3 // Title line + blank line + border bottom
	footerHeight := 1
	listHeight := availableHeight - headerHeight - footerHeight
	if listHeight < 0 {
		listHeight = 0
	}

	// Responsive Logic: If too narrow, hide details
	showDetails := availableWidth > 80

	var listWidth int
	if showDetails {
		listWidth = int(float64(availableWidth) * 0.35)
		if listWidth < 30 {
			listWidth = 30
		}
	} else {
		listWidth = availableWidth
	}

	// Details taking remaining space
	detailsWidth := availableWidth - listWidth
	if detailsWidth < 0 {
		detailsWidth = 0
	}

	// 1. Render List (Left Pane)
	var listContent string

	filteredList := m.getFilteredList()

	if m.loading && len(filteredList) == 0 {
		content = "Loading..."
		return lipgloss.Place(availableWidth, availableHeight, lipgloss.Center, lipgloss.Center, content)
	}

	count := len(filteredList)

	start := 0
	end := 0

	// Scrolling logic
	if count > listHeight {
		if m.cursor < listHeight/2 {
			start = 0
			end = listHeight
		} else if m.cursor >= count-listHeight/2 {
			start = count - listHeight
			end = count
		} else {
			start = m.cursor - listHeight/2
			end = start + listHeight
		}
	} else {
		start = 0
		end = count
	}

	for i := start; i < end; i++ {
		item := filteredList[i]

		prefix := "  "
		// Truncate title to fit listWidth
		maxLen := listWidth - 6
		if maxLen < 5 {
			maxLen = 5
		}

		var line string
		var selected bool
		if i == m.cursor {
			prefix = "➤ "
			selected = true
		}

		switch v := item.(type) {
		case plex.Directory:
			line = v.Title
		case plex.Video:
			if m.mode == ModeEpisodes {
				line = fmt.Sprintf("%d. %s", v.Index, v.Title)
			} else {
				line = v.Title
			}
		}

		if len(line) > maxLen {
			line = line[:maxLen-1] + "…"
		}

		// Add indicators
		indicators := ""
		if v, ok := item.(plex.Video); ok {
			if v.ViewCount > 0 {
				indicators = " " + shared.StyleMetadataValue.Render("✔")
			} else if v.ViewOffset > 0 && v.Duration > 0 {
				indicators = " " + shared.StyleMetadataValue.Render("⏱")
			}
		}

		// Create a style for the full row width
		rowStyle := shared.StyleItemNormal
		if selected {
			// Highlight the entire row width
			rowStyle = lipgloss.NewStyle().
				Background(shared.ColorPlexOrange).
				Foreground(lipgloss.Color("#000000")).
				Bold(true).
				Width(listWidth)
		}

		listContent += rowStyle.Render(fmt.Sprintf("%s%s%s", prefix, line, indicators)) + "\n"
	}

	// Fill remaining height with empty lines
	linesRendered := end - start
	for i := 0; i < listHeight-linesRendered; i++ {
		listContent += "\n"
	}

	// 2. Render Details (Right Pane)
	var details string

	var selectedItem interface{}
	if m.cursor < len(filteredList) {
		selectedItem = filteredList[m.cursor]
	}

	if selectedItem != nil {
		details = renderDetails(selectedItem, detailsWidth-4)
	}

	// Combine
	// Left Pane
	leftPane := lipgloss.NewStyle().
		Width(listWidth).
		Height(listHeight).
		Render(listContent)

	// Right Pane
	var rightPane string
	if showDetails {
		rightPaneContent := lipgloss.NewStyle().
			Width(detailsWidth-4).
			Padding(0, 2).
			Render(details)

		rightPane = lipgloss.NewStyle().
			Width(detailsWidth).
			Height(listHeight).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#333333")).
			Render(lipgloss.Place(detailsWidth, listHeight, lipgloss.Left, lipgloss.Top, rightPaneContent))
	}

	// --- 2. Enriched Footer ---
	totalElements := len(m.getFilteredList())
	unwatchedCount := 0
	for _, item := range m.getFilteredList() {
		if v, ok := item.(plex.Video); ok && v.ViewCount == 0 {
			unwatchedCount++
		}
	}

	footerParts := []string{
		fmt.Sprintf("%d elements", totalElements),
		fmt.Sprintf("Sorted by %s", m.sortMethod.String()),
	}
	if unwatchedCount > 0 {
		footerParts = append(footerParts, fmt.Sprintf("%d unwatched", unwatchedCount))
	}

	footerText := strings.Join(footerParts, " • ")

	// Help keys (Right aligned feel)
	helpKeys := shared.StyleDim.Render(" [/] Search • [S] Sort • [Enter] Select • [Q] Quit")

	footerStyle := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Background(lipgloss.Color("#1a1a1a")).
		Foreground(shared.ColorLightGrey)

	// Calculate space for help keys
	footerContent := footerText
	spaceCount := m.width - lipgloss.Width(footerText) - lipgloss.Width(helpKeys) - 2
	if spaceCount > 0 {
		footerContent += strings.Repeat(" ", spaceCount) + helpKeys
	} else {
		footerContent += "  " + helpKeys
	}

	renderedFooter := footerStyle.Render(footerContent)

	// Final Assemble
	var mainBody string
	if showDetails {
		mainBody = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	} else {
		mainBody = leftPane
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		renderedHeader,
		mainBody,
		renderedFooter,
	)
}

func renderDetails(item interface{}, width int) string {
	var title, subtitle, summary, info string
	var metaBadges []string
	var cast []string
	var director string

	switch v := item.(type) {
	case plex.Directory:
		title = v.Title
		subtitle = v.Type
		summary = v.Summary
		director = formatTags(v.Director)

	case plex.Video:
		title = v.Title

		// Episode specific handling
		if v.Type == "episode" {
			if v.ParentIndex > 0 {
				subtitle = fmt.Sprintf("Season %d", v.ParentIndex)
			}
			if v.Index > 0 {
				subtitle += fmt.Sprintf(" • Episode %d", v.Index)
			}
		} else {
			if v.Year > 0 {
				subtitle = fmt.Sprintf("%d", v.Year)
			}
			if v.ContentRating != "" {
				subtitle += " • " + v.ContentRating
			}
		}

		if v.Duration > 0 {
			mins := v.Duration / 60000
			subtitle += fmt.Sprintf(" • %dm", mins)
		}
		if v.Rating > 0 {
			subtitle += fmt.Sprintf(" • ⭐ %.1f", v.Rating)
		}

		summary = v.Summary

		// Media Info Badges
		if len(v.Media) > 0 {
			m := v.Media[0]
			if m.VideoResolution != "" {
				metaBadges = append(metaBadges, shared.StyleBadgeOrange.Render(strings.ToUpper(m.VideoResolution)))
			}
			if m.VideoCodec != "" {
				metaBadges = append(metaBadges, shared.StyleBadge.Render(strings.ToUpper(m.VideoCodec)))
			}
			if m.AudioCodec != "" {
				metaBadges = append(metaBadges, shared.StyleBadge.Render(strings.ToUpper(m.AudioCodec)))
			}
			if m.AudioChannels > 0 {
				metaBadges = append(metaBadges, shared.StyleBadge.Render(fmt.Sprintf("%d.1", m.AudioChannels-1)))
			}
		}

		// Director
		director = formatTags(v.Director)

		// Cast
		for i, r := range v.Role {
			if i >= 5 {
				break
			}
			cast = append(cast, fmt.Sprintf("• %s %s", shared.StyleMetadataValue.Render(r.Tag), shared.StyleRole.Render("as "+r.Role)))
		}

		// Genres
		if len(v.Genre) > 0 {
			genreRow := ""
			for i, g := range v.Genre {
				if i > 0 {
					genreRow += "  "
				}
				genreRow += lipgloss.NewStyle().
					Foreground(lipgloss.Color("#000000")).
					Background(shared.ColorLightGrey).
					Padding(0, 1).
					Render(g.Tag)
			}
			info += genreRow + "\n"
		}
	}

	// Build the view
	styledTitle := lipgloss.NewStyle().
		Foreground(shared.ColorPlexOrange).
		Bold(true).
		Render(title)

	styledSubtitle := shared.StyleMetadataKey.Render(subtitle)

	badgesRow := strings.Join(metaBadges, " ")

	// Progress Bar
	progressBar := ""
	if v, ok := item.(plex.Video); ok && v.ViewOffset > 0 && v.Duration > 0 {
		percent := float64(v.ViewOffset) / float64(v.Duration)
		barWidth := width / 2
		if barWidth > 30 {
			barWidth = 30
		}
		if barWidth < 10 {
			barWidth = 10
		}

		filled := int(percent * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}

		bar := strings.Repeat("━", filled) + strings.Repeat("─", barWidth-filled)
		progressBar = lipgloss.NewStyle().Foreground(shared.ColorPlexOrange).Render(bar)

		// Add timestamp info
		currentM := (v.ViewOffset / 1000) / 60
		totalM := (v.Duration / 1000) / 60
		progressBar += fmt.Sprintf(" %s %d / %d min", shared.StyleMetadataKey.Render("⏱"), currentM, totalM)
	}

	styledSummary := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#cccccc")).
		Width(width).
		Render(summary)

	detailsGrid := ""
	if director != "" {
		detailsGrid += fmt.Sprintf("%s %s\n", shared.StyleMetadataKey.Render("Director:"), shared.StyleMetadataValue.Render(director))
	}

	castSection := ""
	if len(cast) > 0 {
		castSection = "\n" + shared.StyleMetadataKey.Render("Top Cast:") + "\n" + strings.Join(cast, "\n")
	}

	var layout []string
	layout = append(layout, styledTitle, styledSubtitle, "\n")

	if badgesRow != "" {
		layout = append(layout, badgesRow, "\n")
	}

	if progressBar != "" {
		layout = append(layout, progressBar, "\n")
	}

	layout = append(layout, styledSummary, "\n")

	if detailsGrid != "" {
		layout = append(layout, detailsGrid)
	}

	if castSection != "" {
		layout = append(layout, castSection)
	}

	if info != "" {
		layout = append(layout, "\n", info)
	}

	return lipgloss.JoinVertical(lipgloss.Left, layout...)
}

func formatTags(tags []plex.Tag) string {
	var names []string
	for _, t := range tags {
		names = append(names, t.Tag)
	}
	return strings.Join(names, ", ")
}
