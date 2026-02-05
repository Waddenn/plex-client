package browser

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/store"
)

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

func fetchLibraryItemsFromStore(s *store.Store, targetType string) ([]plex.Video, error) {
	if targetType == "movie" {
		return s.ListMovies()
	}
	return s.ListSeries()
}
