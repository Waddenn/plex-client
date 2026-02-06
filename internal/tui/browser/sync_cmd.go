package browser

import (
	"database/sql"

	"github.com/Waddenn/plex-client/internal/cache"
	"github.com/Waddenn/plex-client/internal/plex"
	tea "github.com/charmbracelet/bubbletea"
)

// MsgBackgroundSyncFinished is sent when a background sync-on-browse completes
type MsgBackgroundSyncFinished struct {
	Added int
	Error error
}

func saveSectionsInBackground(db *sql.DB, sections []plex.Directory) tea.Cmd {
	return func() tea.Msg {
		added := 0
		for _, s := range sections {
			// Just update the sections table
			_, _ = db.Exec(`INSERT OR REPLACE INTO sections (key, title, type, updated_at) VALUES (?, ?, ?, ?)`, s.Key, s.Title, s.Type, s.UpdatedAt)
		}
		return MsgBackgroundSyncFinished{Added: added}
	}
}

func saveItemsInBackground(db *sql.DB, items []plex.Video, itemType string) tea.Cmd {
	return func() tea.Msg {
		added := 0
		var err error
		if itemType == "show" {
			err = cache.SaveSeries(db, convertToDirs(items), &added, nil)
		} else {
			err = cache.SaveMovies(db, items, &added, nil)
		}
		return MsgBackgroundSyncFinished{Added: added, Error: err}
	}
}

func saveChildrenInBackground(db *sql.DB, parentID string, dirs []plex.Directory, vids []plex.Video) tea.Cmd {
	return func() tea.Msg {
		added := 0
		var err error
		if len(dirs) > 0 {
			// Assuming these are seasons
			err = cache.SaveSeasons(db, parentID, dirs, &added, nil)
		}
		if len(vids) > 0 {
			// Assuming these are episodes
			err = cache.SaveEpisodes(db, parentID, vids, &added, nil)
		}
		return MsgBackgroundSyncFinished{Added: added, Error: err}
	}
}

// Map Video back to Directory for SaveSeries (internal use)
func convertToDirs(vids []plex.Video) []plex.Directory {
	var dirs []plex.Directory
	for _, v := range vids {
		dirs = append(dirs, plex.Directory{
			RatingKey:     v.RatingKey,
			Title:         v.Title,
			Summary:       v.Summary,
			Rating:        v.Rating,
			Genre:         v.Genre,
			Director:      v.Director,
			Role:          v.Role,
			ContentRating: v.ContentRating,
			Studio:        v.Studio,
			AddedAt:       v.AddedAt,
			Year:          v.Year,
		})
	}
	return dirs
}
