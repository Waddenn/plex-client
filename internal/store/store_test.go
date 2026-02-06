package store

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func initTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	queries := []string{
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
            "cast" TEXT,
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
		`CREATE TABLE IF NOT EXISTS series (
            id INTEGER PRIMARY KEY,
            title TEXT,
            summary TEXT,
            rating REAL,
            genres TEXT,
            directors TEXT,
            "cast" TEXT,
            content_rating TEXT,
            studio TEXT,
            added_at INTEGER,
            updated_at INTEGER
        );`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("Failed to init schema: %v", err)
		}
	}
	return db
}

func TestStore_ListMovies(t *testing.T) {
	db := initTestDB(t)
	defer db.Close()

	_, err := db.Exec(`INSERT INTO films (id, title, year, part_key, duration, summary, rating, genres, directors, "cast", originallyAvailableAt, content_rating, studio, added_at, updated_at, video_resolution, video_codec, audio_codec, audio_channels)
		VALUES (1, 'Test Movie', 2021, '', 3600, 'Test Summary', 8.5, 'Action', 'John Doe', 'Jane Doe:Lead', '2021-01-01', 'PG-13', 'Studio X', 1600000000, 1600000000, '1080p', 'h264', 'aac', 6)`)
	if err != nil {
		t.Fatalf("Failed to insert movie: %v", err)
	}

	s := New(db)
	movies, err := s.ListMovies()
	if err != nil {
		t.Fatalf("ListMovies failed: %v", err)
	}

	if len(movies) != 1 {
		t.Errorf("Expected 1 movie, got %d", len(movies))
	}

	if movies[0].Title != "Test Movie" {
		t.Errorf("Expected title 'Test Movie', got '%s'", movies[0].Title)
	}
}

func TestStore_ListSeries(t *testing.T) {
	db := initTestDB(t)
	defer db.Close()

	_, err := db.Exec(`INSERT INTO series (id, title, summary, rating, genres, directors, "cast", content_rating, studio, added_at, updated_at) 
		VALUES (1, 'Test Series', 'Test Summary', 9.0, 'Drama', 'Alice Smith', 'Bob Brown:Hero', 'TV-MA', 'Network Y', 1600000000, 1600000000)`)
	if err != nil {
		t.Fatalf("Failed to insert series: %v", err)
	}

	s := New(db)
	series, err := s.ListSeries()
	if err != nil {
		t.Fatalf("ListSeries failed: %v", err)
	}

	if len(series) != 1 {
		t.Errorf("Expected 1 series, got %d", len(series))
	}

	if series[0].Title != "Test Series" {
		t.Errorf("Expected title 'Test Series', got '%s'", series[0].Title)
	}
}
