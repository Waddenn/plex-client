package cache

import (
	"database/sql"
	"testing"

	"github.com/Waddenn/plex-client/internal/plex"
	_ "github.com/mattn/go-sqlite3"
)

// MockPlexClient implements PlexProvider
type MockPlexClient struct {
	Sections []plex.Directory
	Shows    map[string][]plex.Directory // Changed to Directory
	Videos   map[string][]plex.Video     // For movies if needed
	Children map[string]struct {
		Dirs []plex.Directory
		Vids []plex.Video
	}
}

func (m *MockPlexClient) GetSections() ([]plex.Directory, error) {
	return m.Sections, nil
}

func (m *MockPlexClient) GetSectionAll(key string) ([]plex.Directory, []plex.Video, error) {
	return nil, m.Videos[key], nil
}

func (m *MockPlexClient) GetSectionDirs(key string) ([]plex.Directory, error) {
	return m.Shows[key], nil
}

func (m *MockPlexClient) GetChildren(key string) ([]plex.Directory, []plex.Video, error) {
	c, ok := m.Children[key]
	if !ok {
		return nil, nil, nil
	}
	return c.Dirs, c.Vids, nil
}

func initTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	queries := []string{

		`CREATE TABLE IF NOT EXISTS series (
            id INTEGER PRIMARY KEY,
            title TEXT,
            summary TEXT,
            rating REAL,
            genres TEXT,
            content_rating TEXT,
            studio TEXT,
            added_at INTEGER,
            updated_at INTEGER
        );`,
		`CREATE TABLE IF NOT EXISTS seasons (
            id INTEGER PRIMARY KEY,
            series_id INTEGER,
            season_index INTEGER,
            summary TEXT,
            updated_at INTEGER,
            FOREIGN KEY(series_id) REFERENCES series(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS episodes (
            id INTEGER PRIMARY KEY,
            season_id INTEGER,
            episode_index INTEGER,
            title TEXT,
            part_key TEXT,
            duration INTEGER,
            summary TEXT,
            rating REAL,
            updated_at INTEGER,
            FOREIGN KEY(season_id) REFERENCES seasons(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS films (
            id INTEGER PRIMARY KEY,
            title TEXT,
            year INTEGER,
            part_key TEXT,
            duration INTEGER,
            summary TEXT,
            rating REAL,
            genres TEXT,
            originallyAvailableAt TEXT,
            content_rating TEXT,
            studio TEXT,
            added_at INTEGER,
            updated_at INTEGER
        );`,
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("Failed to init schema: %v", err)
		}
	}
	return db
}

func TestSyncShows(t *testing.T) {
	db := initTestDB(t)
	defer db.Close()

	mock := &MockPlexClient{
		Sections: []plex.Directory{
			{Key: "1", Title: "TV Shows", Type: "show", UpdatedAt: 1234567890},
		},
		Shows: map[string][]plex.Directory{
			"1": {
				{RatingKey: "100", Title: "Test Show", Summary: "A test show", Rating: 9.0, Genre: []plex.Tag{{Tag: "Comedy"}}},
			},
		},
		Children: map[string]struct {
			Dirs []plex.Directory
			Vids []plex.Video
		}{
			"100": { // Seasons of show 100
				Dirs: []plex.Directory{
					{RatingKey: "101", Title: "Season 1", Type: "season", Index: "1", Summary: "First season"},
				},
			},
			"101": { // Episodes of season 101
				Vids: []plex.Video{
					{RatingKey: "102", Title: "Pilot", Index: 1, Duration: 120000, Summary: "First ep", Rating: 8.5},
				},
			},
		},
	}

	if err := Sync(mock, db, true); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify Show
	var title string
	err := db.QueryRow("SELECT title FROM series WHERE id=100").Scan(&title)
	if err != nil {
		t.Fatalf("Show insert failed: %v", err)
	}
	if title != "Test Show" {
		t.Errorf("Expected show 'Test Show', got '%s'", title)
	}

	// Verify Season
	var summary string
	err = db.QueryRow("SELECT summary FROM seasons WHERE id=101").Scan(&summary)
	if err != nil {
		t.Fatalf("Season insert failed: %v", err)
	}
	if summary != "First season" {
		t.Errorf("Expected season summary 'First season', got '%s'", summary)
	}

	// Verify Episode
	var epTitle string
	err = db.QueryRow("SELECT title FROM episodes WHERE id=102").Scan(&epTitle)
	if err != nil {
		t.Fatalf("Episode insert failed: %v", err)
	}
	if epTitle != "Pilot" {
		t.Errorf("Expected episode title 'Pilot', got '%s'", epTitle)
	}
}
