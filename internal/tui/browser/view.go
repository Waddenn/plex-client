package browser

import (
	"fmt"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	// --- 1. Dynamic Breadcrumb Header ---
	var breadcrumb string
	switch m.mode {
	case ModeSections:
		breadcrumb = "📂 Library"
	case ModeItems:
		title := "Movies"
		if m.targetType == "show" {
			title = "Series"
		}
		breadcrumb = fmt.Sprintf("📂 Library > %s", title)
	case ModeSeasons:
		breadcrumb = "📂 Library > Series > Seasons"
	case ModeEpisodes:
		breadcrumb = "📂 Library > Series > Seasons > Episodes"
	}

	headerViewSource := ""
	if m.showSearch {
		headerViewSource = fmt.Sprintf("🔍 %s", m.textInput.View())
	} else {
		headerViewSource = breadcrumb
		if m.textInput.Value() != "" {
			headerViewSource += shared.StyleDim.Render(fmt.Sprintf(" [Filter: %s]", m.textInput.Value()))
		}
	}
	if shared.IsBlankVisible(headerViewSource) {
		headerViewSource = "Browse"
	}

	// Layout dims
	availableWidth := shared.ClampMin(m.width, 40)
	availableHeight := shared.ClampMin(m.height, 10)

	renderedHeader, headerHeight := shared.RenderHeaderLegacySafe(headerViewSource, availableWidth)

	// Responsive Logic
	showDetails := availableWidth > shared.SplitThreshold
	var listWidth int
	if showDetails {
		listWidth, _ = shared.SplitWidths(availableWidth, shared.SplitLeftRatio, shared.SplitMinLeft, shared.SplitMinRight)
	} else {
		listWidth = availableWidth
	}
	detailsWidth := availableWidth - listWidth

	// --- 4. Render Body ---
	var leftPane, rightPane string
	filteredList := m.getFilteredList()
	count := len(filteredList)
	start := 0
	end := 0

	// Footer
	totalElements := len(filteredList)
	footerText := fmt.Sprintf("%d elements • Sorted by %s", totalElements, m.sortMethod.String())
	helpKeys := "[/] Search • [S] Sort • [Enter] Select • [Q] Quit"
	renderedFooter, footerHeight := shared.RenderFooterLegacySafe(footerText, helpKeys, availableWidth)

	// Calculate heights
	// footerHeight fixed to 1 via RenderFooterLegacySafe
	listHeight := availableHeight - headerHeight - footerHeight
	if listHeight < 0 {
		listHeight = 0
	}

	if m.loading && count == 0 {
		leftPane = lipgloss.NewStyle().
			Width(listWidth).
			Height(listHeight).
			Render("\n\n  Loading...")
	} else {
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

		var listContent string
		for i := start; i < end; i++ {
			item := filteredList[i]
			prefix := "  "
			maxLen := listWidth - 6
			if maxLen < 5 {
				maxLen = 5
			}

			var line string
			selected := (i == m.cursor)

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

			indicators := ""
			if v, ok := item.(plex.Video); ok {
				if v.ViewCount > 0 {
					indicators = " " + shared.StyleMetadataValue.Render("✔")
				} else if v.ViewOffset > 0 && v.Duration > 0 {
					indicators = " " + shared.StyleMetadataValue.Render("⏱")
				}
			}

			rowStyle := shared.StyleItemNormal.Copy().Width(listWidth).MaxHeight(1)
			if selected {
				rowStyle = rowStyle.Copy().
					Foreground(shared.ColorPlexOrange).
					Bold(true).
					Width(listWidth)
				prefix = shared.StyleHighlight.Render("▍") + " "
			}

			listContent += rowStyle.Render(fmt.Sprintf("%s%s%s", prefix, line, indicators)) + "\n"
		}

		// Fill remaining height
		linesRendered := end - start
		for i := 0; i < listHeight-linesRendered; i++ {
			listContent += "\n"
		}

		leftPane = lipgloss.NewStyle().
			Width(listWidth).
			Height(listHeight).
			Render(listContent)

		if showDetails {
			var selectedItem interface{}
			if m.cursor < len(filteredList) {
				selectedItem = filteredList[m.cursor]
			}

			details := ""
			if selectedItem != nil {
				details = renderDetails(selectedItem, detailsWidth-4)
			}

			rightPaneContent := lipgloss.NewStyle().
				Width(detailsWidth-4).
				Padding(0, 2).
				Render(details)

			rightPane = shared.StyleRightPanel.Copy().
				Width(detailsWidth).
				Height(listHeight).
				Render(lipgloss.Place(detailsWidth, listHeight, lipgloss.Left, lipgloss.Top, rightPaneContent))
		}
	}

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
