package cache

import (
	"database/sql"
	"strconv"
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
	// Use cache=shared and busy_timeout to better simulate real world concurrency
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=1000")
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
            directors TEXT,
            cast TEXT,
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
            video_resolution TEXT,
            video_codec TEXT,
            audio_codec TEXT,
            audio_channels INTEGER,
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
            directors TEXT,
            cast TEXT,
            originallyAvailableAt TEXT,
            content_rating TEXT,
            studio TEXT,
            added_at INTEGER,
            updated_at INTEGER,
            video_resolution TEXT,
            video_codec TEXT,
            audio_codec TEXT,
            audio_channels INTEGER
        );`,
		`CREATE TABLE IF NOT EXISTS sections (
			key TEXT PRIMARY KEY,
			title TEXT,
			type TEXT,
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

	if err := Sync(mock, db, true, func(s string, a int) {}); err != nil {
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
func TestSaveMovies(t *testing.T) {
	db := initTestDB(t)
	defer db.Close()

	movies := []plex.Video{
		{RatingKey: "1", Title: "Movie 1", Year: 2021, AddedAt: 100, UpdatedAt: 200, Summary: "Summary 1"},
		{RatingKey: "2", Title: "Movie 2", Year: 2022, AddedAt: 105, UpdatedAt: 205, Summary: "Summary 2"},
	}

	totalAdded := 0
	if err := SaveMovies(db, movies, &totalAdded, nil); err != nil {
		t.Fatalf("SaveMovies failed: %v", err)
	}

	if totalAdded != 2 {
		t.Errorf("Expected 2 movies added, got %d", totalAdded)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM films").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 rows in films table, got %d", count)
	}
}

func TestIncrementalSync(t *testing.T) {
	db := initTestDB(t)
	defer db.Close()

	movies := []plex.Video{
		{RatingKey: "1", Title: "Initial Title", Year: 2021, AddedAt: 100, UpdatedAt: 200, Summary: "Initial Summary"},
	}

	// First save
	added := 0
	SaveMovies(db, movies, &added, nil)

	// Second save with SAME UpdatedAt but DIFFERENT Title
	movies[0].Title = "Updated Title"
	added = 0
	if err := SaveMovies(db, movies, &added, nil); err != nil {
		t.Fatalf("Second SaveMovies failed: %v", err)
	}

	if added != 0 {
		t.Errorf("Expected 0 movies added on incremental sync with same UpdatedAt, got %d", added)
	}

	var title string
	db.QueryRow("SELECT title FROM films WHERE id=1").Scan(&title)
	if title != "Initial Title" {
		t.Errorf("Expected title 'Initial Title' (skipped update), got '%s'", title)
	}

	// Third save with NEWER UpdatedAt
	movies[0].UpdatedAt = 300
	added = 0
	if err := SaveMovies(db, movies, &added, nil); err != nil {
		t.Fatalf("Third SaveMovies failed: %v", err)
	}

	db.QueryRow("SELECT title FROM films WHERE id=1").Scan(&title)
	if title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title' after UpdatedAt changed, got '%s'", title)
	}
}
func TestSaveSeasonsAndEpisodes(t *testing.T) {
	db := initTestDB(t)
	defer db.Close()

	seriesID := "100"
	seasons := []plex.Directory{
		{RatingKey: "101", Title: "Season 1", Type: "season", Index: "1", Summary: "S1 Summary"},
	}
	episodes := []plex.Video{
		{RatingKey: "102", Title: "Episode 1", Index: 1, Summary: "E1 Summary"},
	}

	added := 0
	if err := SaveSeasons(db, seriesID, seasons, &added, nil); err != nil {
		t.Fatalf("SaveSeasons failed: %v", err)
	}
	if err := SaveEpisodes(db, "101", episodes, &added, nil); err != nil {
		t.Fatalf("SaveEpisodes failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM seasons WHERE series_id=100").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 season, got %d", count)
	}

	db.QueryRow("SELECT count(*) FROM episodes WHERE season_id=101").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 episode, got %d", count)
	}
}

func TestConcurrency(t *testing.T) {
	// Note: :memory: DBs in SQLite have some issues with shared cache/concurrency
	// but we can try to see if IMMEDIATE transactions prevent basic errors.
	db := initTestDB(t)
	defer db.Close()

	// In real setup we use WAL and busy_timeout, but :memory: is limited.
	// We'll just test if multiple goroutines can call SaveMovies without crashing.

	const workers = 10
	errChan := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			movies := []plex.Video{
				{RatingKey: strconv.Itoa(id), Title: "Concurrent Movie " + strconv.Itoa(id), UpdatedAt: 100},
			}
			errChan <- SaveMovies(db, movies, nil, nil)
		}(i)
	}

	for i := 0; i < workers; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent worker failed: %v", err)
		}
	}
}
