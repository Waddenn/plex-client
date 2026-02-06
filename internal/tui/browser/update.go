package browser

import (
	"fmt"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) tea.Cmd {

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

		case "q":
			if !m.showSearch {
				return func() tea.Msg { return shared.MsgBack{} }
			}

		case "r":
			if !m.showSearch {
				return func() tea.Msg { return shared.MsgManualSync{} }
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return nil
		case "down", "j":
			count := m.getFilteredCount()
			if m.cursor < count-1 {
				m.cursor++
			}
			return nil
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
				m.filteredList = nil
				return nil
			} else if m.mode == ModeSeasons {
				m.mode = ModeItems
				m.selectedShowTitle = "" // Clear show title when going back
				m.cursor = 0
				m.showSearch = false
				m.textInput.Reset()
				m.needsRefresh = true
				m.filteredList = nil
				return nil
			} else if m.mode == ModeEpisodes {
				// For mini-series that skipped the season selection, go back to items
				if len(m.seasons) == 0 {
					m.mode = ModeItems
					m.selectedShowTitle = "" // Clear show title when going back
				} else {
					m.mode = ModeSeasons
				}
				m.cursor = 0
				m.showSearch = false
				m.textInput.Reset()
				m.needsRefresh = true
				m.filteredList = nil
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
						m.filteredList = nil

						// Clear previous items to avoid relics if cache is empty
						m.items = nil

						// Instant load from DB
						if dbItems, err := fetchLibraryItemsFromStore(m.store, m.targetType); err == nil && len(dbItems) > 0 {
							m.items = dbItems
							m.loading = false // Hide loader if we have data
						}
						if m.AutoSync {
							return fetchLibraryItems(m.plexClient, item.Key)
						}
						m.loading = false
						return nil
					} else if m.mode == ModeSeasons {
						m.mode = ModeEpisodes
						m.loading = true
						m.cursor = 0
						m.showSearch = false
						m.textInput.Reset()
						m.needsRefresh = true
						m.filteredList = nil

						// Clear previous episodes
						m.episodes = nil

						// Instant load from DB
						if dbEpisodes, err := fetchEpisodesFromStore(m.store, item.RatingKey); err == nil && len(dbEpisodes) > 0 {
							m.episodes = dbEpisodes
							m.loading = false
						}
						if m.AutoSync {
							return fetchChildren(m.plexClient, item.RatingKey)
						}
						m.loading = false
						return nil
					}
				case plex.Video: // Item or Episode
					if m.mode == ModeItems {
						if item.Type == "show" {
							m.selectedShowTitle = item.Title // Store show title for breadcrumbs
							m.mode = ModeSeasons
							m.loading = true
							m.cursor = 0
							m.showSearch = false
							m.textInput.Reset()
							m.needsRefresh = true
							m.filteredList = nil

							// Clear previous seasons
							m.seasons = nil

							// Instant load from DB
							if dbSeasons, err := fetchSeasonsFromStore(m.store, item.RatingKey); err == nil && len(dbSeasons) > 0 {
								m.seasons = dbSeasons
								m.loading = false
							}
							if m.AutoSync {
								return fetchChildren(m.plexClient, item.RatingKey)
							}
							m.loading = false
							return nil
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
		m.filteredList = nil // Force refresh
		if msg.Err != nil {
			m.errorMsg = fmt.Sprintf("Failed to load sections: %v", msg.Err)
		} else {
			m.sections = msg.Sections
			m.errorMsg = "" // Clear any previous error

			// Background Update Store
			syncCmd := saveSectionsInBackground(m.store.DB, m.sections)

			// UX Improvement: If only one section, auto-select it
			if len(m.sections) == 1 {
				section := m.sections[0]
				m.mode = ModeItems
				m.loading = true
				m.needsRefresh = true
				m.filteredList = nil

				// Clear previous items
				m.items = nil

				if dbItems, err := fetchLibraryItemsFromStore(m.store, m.targetType); err == nil && len(dbItems) > 0 {
					m.items = dbItems
					m.loading = false
				}
				return tea.Batch(syncCmd, fetchLibraryItems(m.plexClient, section.Key))
			}
			return syncCmd
		}
	case MsgItemsLoaded:
		m.loading = false
		m.needsRefresh = true
		m.filteredList = nil // Force refresh to show updated metadata
		if msg.Err != nil {
			m.errorMsg = fmt.Sprintf("Failed to load items: %v", msg.Err)
		} else {
			m.errorMsg = "" // Clear any previous error
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
			// Background Update Store
			return saveItemsInBackground(m.store.DB, m.items, m.targetType)
		}
	case MsgChildrenLoaded:
		m.loading = false
		m.needsRefresh = true
		m.filteredList = nil // Force refresh to show updated metadata
		if msg.Err != nil {
			m.errorMsg = fmt.Sprintf("Failed to load content: %v", msg.Err)
		} else {
			m.errorMsg = "" // Clear any previous error
			m.seasons = msg.Dirs
			m.episodes = msg.Videos

			// Background Update Store
			syncCmd := saveChildrenInBackground(m.store.DB, msg.ParentID, m.seasons, m.episodes)

			// Auto-Switch Logic:
			if m.mode == ModeSeasons {
				if len(m.seasons) == 0 && len(m.episodes) > 0 {
					m.mode = ModeEpisodes
					m.filteredList = nil
				}
			}
			return syncCmd
		}
	case MsgBackgroundSyncFinished:
		// Silently ignore or maybe show a tiny indicator if Added > 0
		return nil
	}
	return nil
}
