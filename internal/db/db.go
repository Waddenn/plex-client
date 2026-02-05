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

	if err := migrateSchema(db); err != nil {
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
            directors TEXT,
            cast TEXT,
            originallyAvailableAt TEXT,
            content_rating TEXT,
            studio TEXT,
            added_at INTEGER,
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
            directors TEXT,
            cast TEXT,
            content_rating TEXT,
            studio TEXT,
            added_at INTEGER,
            updated_at INTEGER
        );`,
		`CREATE INDEX IF NOT EXISTS idx_series_title ON series(title);`,

		`CREATE TABLE IF NOT EXISTS seasons (
            id INTEGER PRIMARY KEY,
            series_id INTEGER,
            season_index INTEGER,
            summary TEXT,
            updated_at INTEGER,
            FOREIGN KEY(series_id) REFERENCES series(id) ON DELETE CASCADE
        );`,
		`CREATE INDEX IF NOT EXISTS idx_seasons_series_id ON seasons(series_id);`,

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
		`CREATE INDEX IF NOT EXISTS idx_episodes_season_id ON episodes(season_id);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func migrateSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	hasSaisons, err := tableExists(tx, "saisons")
	if err != nil {
		return err
	}
	hasSeasons, err := tableExists(tx, "seasons")
	if err != nil {
		return err
	}

	// Migrate saisons -> seasons
	if hasSaisons && !hasSeasons {
		if _, err := tx.Exec(`CREATE TABLE seasons (
			id INTEGER PRIMARY KEY,
			series_id INTEGER,
			season_index INTEGER,
			summary TEXT,
			updated_at INTEGER,
			FOREIGN KEY(series_id) REFERENCES series(id) ON DELETE CASCADE
		);`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO seasons (id, series_id, season_index, summary, updated_at)
			SELECT id, serie_id, saison_index, summary, updated_at FROM saisons;`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DROP TABLE saisons;`); err != nil {
			return err
		}
	}

	// Migrate episodes.saison_id -> episodes.season_id
	hasEpisodes, err := tableExists(tx, "episodes")
	if err != nil {
		return err
	}
	if hasEpisodes {
		hasSeasonID, err := columnExists(tx, "episodes", "season_id")
		if err != nil {
			return err
		}
		hasSaisonID, err := columnExists(tx, "episodes", "saison_id")
		if err != nil {
			return err
		}
		if !hasSeasonID && hasSaisonID {
			if _, err := tx.Exec(`CREATE TABLE episodes_new (
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
			);`); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO episodes_new (id, season_id, episode_index, title, part_key, duration, summary, rating, updated_at)
				SELECT id, saison_id, episode_index, title, part_key, duration, summary, rating, updated_at FROM episodes;`); err != nil {
				return err
			}
			if _, err := tx.Exec(`DROP TABLE episodes;`); err != nil {
				return err
			}
			if _, err := tx.Exec(`ALTER TABLE episodes_new RENAME TO episodes;`); err != nil {
				return err
			}
		}
	}

	// Add directors and cast columns to films table
	hasFilms, err := tableExists(tx, "films")
	if err != nil {
		return err
	}
	if hasFilms {
		hasDirectors, err := columnExists(tx, "films", "directors")
		if err != nil {
			return err
		}
		if !hasDirectors {
			if _, err := tx.Exec(`ALTER TABLE films ADD COLUMN directors TEXT;`); err != nil {
				return err
			}
		}

		hasCast, err := columnExists(tx, "films", "cast")
		if err != nil {
			return err
		}
		if !hasCast {
			if _, err := tx.Exec(`ALTER TABLE films ADD COLUMN cast TEXT;`); err != nil {
				return err
			}
		}
	}

	// Add directors and cast columns to series table
	hasSeries, err := tableExists(tx, "series")
	if err != nil {
		return err
	}
	if hasSeries {
		hasDirectors, err := columnExists(tx, "series", "directors")
		if err != nil {
			return err
		}
		if !hasDirectors {
			if _, err := tx.Exec(`ALTER TABLE series ADD COLUMN directors TEXT;`); err != nil {
				return err
			}
		}

		hasCast, err := columnExists(tx, "series", "cast")
		if err != nil {
			return err
		}
		if !hasCast {
			if _, err := tx.Exec(`ALTER TABLE series ADD COLUMN cast TEXT;`); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func tableExists(tx *sql.Tx, name string) (bool, error) {
	var count int
	err := tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?;`, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func columnExists(tx *sql.Tx, tableName, columnName string) (bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + tableName + `);`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var (
		cid       int
		name      string
		colType   string
		notnull   int
		dfltValue *string
		pk        int
	)
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, nil
}
