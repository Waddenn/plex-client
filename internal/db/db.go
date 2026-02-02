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
            originallyAvailableAt TEXT
        );`,
		`CREATE TABLE IF NOT EXISTS series (
            id INTEGER PRIMARY KEY,
            title TEXT,
            summary TEXT,
            rating REAL,
            genres TEXT
        );`,
		`CREATE TABLE IF NOT EXISTS saisons (
            id INTEGER PRIMARY KEY,
            serie_id INTEGER,
            saison_index INTEGER,
            summary TEXT
        );`,
		`CREATE TABLE IF NOT EXISTS episodes (
            id INTEGER PRIMARY KEY,
            saison_id INTEGER,
            episode_index INTEGER,
            title TEXT,
            part_key TEXT,
            duration INTEGER,
            summary TEXT,
            rating REAL
        );`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
