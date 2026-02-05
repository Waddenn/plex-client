package cache

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Waddenn/plex-client/internal/plex"
)

// PlexProvider interface allows mocking the Plex client
type PlexProvider interface {
	GetSections() ([]plex.Directory, error)
	GetSectionAll(key string) ([]plex.Directory, []plex.Video, error) // Updated signature
	GetSectionDirs(key string) ([]plex.Directory, error)
	GetChildren(key string) ([]plex.Directory, []plex.Video, error)
}

func Sync(p PlexProvider, d *sql.DB, force bool) error {
	sections, err := p.GetSections()
	if err != nil {
		return err
	}

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range sections {
		// Incremental sync check
		var lastUpdated int64
		row := d.QueryRow("SELECT value FROM metadata WHERE key = ?", "section_"+s.Key)
		var valStr string
		if err := row.Scan(&valStr); err == nil {
			lastUpdated, _ = strconv.ParseInt(valStr, 10, 64)
		}

		if !force && s.UpdatedAt > 0 && lastUpdated >= s.UpdatedAt {
			// Section up to date
			continue
		}

		if s.Type == "movie" {
			// log.Printf("Syncing movies section: %s", s.Title)
			_, videos, err := p.GetSectionAll(s.Key) // Ignore Dirs, use Videos
			if err != nil {
				log.Printf("Error fetching movies for section %s: %v", s.Title, err)
				continue
			}
			for _, v := range videos {
				partKey := ""
				if len(v.Media) > 0 && len(v.Media[0].Part) > 0 {
					partKey = v.Media[0].Part[0].Key
				}
				genres := joinTags(v.Genre)
				updatedAt := time.Now().Unix()

				_, err := tx.Exec(`INSERT OR REPLACE INTO films (id, title, year, part_key, duration, summary, rating, genres, originallyAvailableAt, content_rating, studio, added_at, updated_at) 
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					v.RatingKey, v.Title, v.Year, partKey, v.Duration, v.Summary, v.Rating, genres, v.OriginallyAvailableAt, v.ContentRating, v.Studio, v.AddedAt, updatedAt)
				if err != nil {
					log.Printf("Error inserting movie %s: %v", v.Title, err)
				}
			}
		} else if s.Type == "show" {
			// log.Printf("Syncing shows section: %s", s.Title)
			shows, err := p.GetSectionDirs(s.Key)
			if err != nil {
				log.Printf("Error fetching shows for section %s: %v", s.Title, err)
				continue
			}
			for _, show := range shows {
				genres := joinTags(show.Genre)
				updatedAt := time.Now().Unix()

				_, err := tx.Exec(`INSERT OR REPLACE INTO series (id, title, summary, rating, genres, content_rating, studio, added_at, updated_at) 
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					show.RatingKey, show.Title, show.Summary, show.Rating, genres, show.ContentRating, show.Studio, show.AddedAt, updatedAt)
				if err != nil {
					log.Printf("Error inserting show %s: %v", show.Title, err)
					continue
				}

				if err := syncSeasons(p, tx, show.RatingKey); err != nil {
					log.Printf("Error fetching seasons for %s: %v", show.Title, err)
				}
			}
		}

		// Update metadata
		if s.UpdatedAt > 0 {
			_, err := tx.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)`, "section_"+s.Key, strconv.FormatInt(s.UpdatedAt, 10))
			if err != nil {
				log.Printf("Error updating metadata: %v", err)
			}
		}
	}

	return tx.Commit()
}

func syncSeasons(p PlexProvider, tx *sql.Tx, showID string) error {
	seasons, _, err := p.GetChildren(showID)
	if err != nil {
		return err
	}

	for _, season := range seasons {
		if season.Type != "season" {
			continue
		}

		sIndex, _ := strconv.Atoi(season.Index)
		updatedAt := time.Now().Unix()

		_, err := tx.Exec(`INSERT OR REPLACE INTO seasons (id, series_id, season_index, summary, updated_at) 
			VALUES (?, ?, ?, ?, ?)`,
			season.RatingKey, showID, sIndex, season.Summary, updatedAt)
		if err != nil {
			log.Printf("Error inserting season %s: %v", season.Title, err)
			continue
		}

		_, episodes, err := p.GetChildren(season.RatingKey)
		if err != nil {
			log.Printf("Error fetching episodes for season %s: %v", season.Title, err)
			continue
		}

		for _, e := range episodes {
			partKey := ""
			if len(e.Media) > 0 && len(e.Media[0].Part) > 0 {
				partKey = e.Media[0].Part[0].Key
			}
			eIndex := e.Index

			_, err := tx.Exec(`INSERT OR REPLACE INTO episodes (id, season_id, episode_index, title, part_key, duration, summary, rating, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				e.RatingKey, season.RatingKey, eIndex, e.Title, partKey, e.Duration, e.Summary, e.Rating, updatedAt)
			if err != nil {
				log.Printf("Error inserting episode %s: %v", e.Title, err)
			}
		}
	}
	return nil
}

func joinTags(tags []plex.Tag) string {
	var s []string
	for _, t := range tags {
		s = append(s, t.Tag)
	}
	return strings.Join(s, ", ")
}
