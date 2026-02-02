package db

import (
	"database/sql"
	"path/filepath"

	"github.com/Waddenn/plex-client/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

func Open() (*sql.DB, error) {
	cacheDir, err := config.CacheDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cacheDir, "cache.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func initSchema(db *sql.DB) error {
	// Enable Write-Ahead Logging for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT
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
            updated_at INTEGER
        );`,
		`CREATE INDEX IF NOT EXISTS idx_films_title ON films(title);`,
		`CREATE INDEX IF NOT EXISTS idx_films_year ON films(year);`,

		`CREATE TABLE IF NOT EXISTS series (
            id INTEGER PRIMARY KEY,
            title TEXT,
            summary TEXT,
            rating REAL,
            genres TEXT,
            updated_at INTEGER
        );`,
		`CREATE INDEX IF NOT EXISTS idx_series_title ON series(title);`,

		`CREATE TABLE IF NOT EXISTS saisons (
            id INTEGER PRIMARY KEY,
            serie_id INTEGER,
            saison_index INTEGER,
            summary TEXT,
            updated_at INTEGER,
            FOREIGN KEY(serie_id) REFERENCES series(id) ON DELETE CASCADE
        );`,
		`CREATE INDEX IF NOT EXISTS idx_saisons_serie_id ON saisons(serie_id);`,

		`CREATE TABLE IF NOT EXISTS episodes (
            id INTEGER PRIMARY KEY,
            saison_id INTEGER,
            episode_index INTEGER,
            title TEXT,
            part_key TEXT,
            duration INTEGER,
            summary TEXT,
            rating REAL,
            updated_at INTEGER,
            FOREIGN KEY(saison_id) REFERENCES saisons(id) ON DELETE CASCADE
        );`,
		`CREATE INDEX IF NOT EXISTS idx_episodes_saison_id ON episodes(saison_id);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
