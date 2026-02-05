package browser

import (
	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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
						if dbItems, err := fetchLibraryItemsFromStore(m.store, m.targetType); err == nil && len(dbItems) > 0 {
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
				if dbItems, err := fetchLibraryItemsFromStore(m.store, m.targetType); err == nil && len(dbItems) > 0 {
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
