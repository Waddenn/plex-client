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
		breadcrumb = "📂 Plex CLI > Library"
	case ModeItems:
		title := "Movies"
		if m.targetType == "show" {
			title = "Series"
		}
		breadcrumb = fmt.Sprintf("📂 Plex CLI > %s", title)
	case ModeSeasons:
		if m.selectedShowTitle != "" {
			breadcrumb = fmt.Sprintf("📂 Plex CLI > Series > %s", m.selectedShowTitle)
		} else {
			breadcrumb = "📂 Plex CLI > Series > Seasons"
		}
	case ModeEpisodes:
		if m.selectedShowTitle != "" {
			breadcrumb = fmt.Sprintf("📂 Plex CLI > Series > %s > Episodes", m.selectedShowTitle)
		} else {
			breadcrumb = "📂 Plex CLI > Series > Episodes"
		}
	}

	headerViewSource := ""
	if m.showSearch {
		headerViewSource = fmt.Sprintf("🔍 %s", m.textInput.View())
	} else {
		headerViewSource = breadcrumb
		if m.SyncStatus != "" {
			headerViewSource += shared.StyleDim.Render("  " + m.SyncStatus)
		}
		if m.textInput.Value() != "" {
			headerViewSource += shared.StyleDim.Render(fmt.Sprintf(" [Filter: %s]", m.textInput.Value()))
		}
	}
	if shared.IsBlankVisible(headerViewSource) {
		headerViewSource = "Browse"
	}

	// Layout dims
	availableWidth := shared.ClampMin(m.width, 20)
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
	helpKeys := "[/] Search • [S] Sort • [Enter] Select • [Esc/Q] Back"
	renderedFooter, footerHeight := shared.RenderFooterLegacySafe(footerText, helpKeys, availableWidth)

	// Calculate heights
	// footerHeight fixed to 1 via RenderFooterLegacySafe
	listHeight := availableHeight - headerHeight - footerHeight
	minListHeight := 3
	if listHeight < minListHeight {
		listHeight = minListHeight
	}

	if m.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			Padding(1)
		leftPane = lipgloss.NewStyle().
			Width(listWidth).
			Height(listHeight).
			MaxHeight(listHeight).
			Render(errorStyle.Render("⚠ " + m.errorMsg + "\n\nPress Esc/Q to go back"))
	} else if m.loading && count == 0 {
		leftPane = lipgloss.NewStyle().
			Width(listWidth).
			Height(listHeight).
			MaxHeight(listHeight).
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
			sidebarIndicator := ""
			textModified := false
			if v, ok := item.(plex.Video); ok {
				indicators, sidebarIndicator = m.renderStatusIndicator(v, listWidth)

				// For text-style, we modify the line style instead of adding indicators
				if m.StatusIndicatorStyle == "text-style" {
					if v.ViewCount > 0 {
						line = shared.StyleDim.Render(line)
						textModified = true
					} else if v.ViewOffset > 0 && v.Duration > 0 {
						line = lipgloss.NewStyle().Foreground(shared.ColorPlexOrange).Render(line)
						textModified = true
					}
				}
			}

			rowStyle := shared.StyleItemNormal.Copy().Width(listWidth).MaxHeight(1)
			if selected {
				rowStyle = rowStyle.Copy().
					Foreground(shared.ColorPlexOrange).
					Bold(true).
					Width(listWidth)
				prefix = shared.SelectionIndicator()

				// Override text modification for selected items
				if textModified && m.StatusIndicatorStyle == "text-style" {
					// Keep the orange color for selected items
					line = strings.TrimSpace(lipgloss.NewStyle().Render(line))
				}
			}

			// For sidebar style, prefix with a colored bar
			if sidebarIndicator != "" {
				prefix = sidebarIndicator + prefix[len(sidebarIndicator):]
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
			MaxHeight(listHeight).
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
				MaxHeight(listHeight).
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
				metaBadges = append(metaBadges, shared.StyleBadge.Render(formatAudioChannels(m.AudioChannels)))
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
				genreRow += shared.StyleBadge.Render(g.Tag)
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

	styledSummary := lipgloss.NewStyle().
		Foreground(shared.ColorLightGrey).
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

func formatAudioChannels(channels int) string {
	switch channels {
	case 1:
		return "MONO"
	case 2:
		return "2.0"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	default:
		return fmt.Sprintf("%dch", channels)
	}
}

// renderStatusIndicator returns the status indicator and sidebar indicator based on the configured style
func (m *Model) renderStatusIndicator(v plex.Video, listWidth int) (string, string) {
	if v.ViewCount == 0 && v.ViewOffset == 0 {
		// Unwatched
		if m.StatusIndicatorStyle == "dots" {
			return " " + shared.StyleDim.Render("○"), ""
		}
		return "", ""
	}

	// Calculate progress percentage for in-progress items
	percent := 0
	if v.ViewOffset > 0 && v.Duration > 0 {
		percent = int(float64(v.ViewOffset) / float64(v.Duration) * 100)
	}

	switch m.StatusIndicatorStyle {
	case "badges":
		if v.ViewCount > 0 {
			badge := shared.StyleBadge.Copy().
				Foreground(lipgloss.Color("#00FF00")).
				Render("WATCHED")
			return " " + badge, ""
		} else if v.ViewOffset > 0 && v.Duration > 0 {
			badge := shared.StyleBadgeOrange.Render(fmt.Sprintf("%d%%", percent))
			return " " + badge, ""
		}

	case "sidebar":
		if v.ViewCount > 0 {
			bar := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("▐")
			return "", bar
		} else if v.ViewOffset > 0 && v.Duration > 0 {
			bar := lipgloss.NewStyle().Foreground(shared.ColorPlexOrange).Render("▐")
			return "", bar
		}

	case "text-style":
		// Text styling is handled in the main rendering logic
		return "", ""

	case "dots":
		if v.ViewCount > 0 {
			dot := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("●")
			return " " + dot, ""
		} else if v.ViewOffset > 0 && v.Duration > 0 {
			dot := lipgloss.NewStyle().Foreground(shared.ColorPlexOrange).Render("◐")
			return " " + dot, ""
		}
	}

	return "", ""
}
