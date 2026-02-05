package browser

import (
	"fmt"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	"github.com/charmbracelet/lipgloss"
)

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
