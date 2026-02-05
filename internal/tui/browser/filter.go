package browser

import (
	"sort"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
)

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
