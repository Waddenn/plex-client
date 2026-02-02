package cache

import (
	"database/sql"
	"log"
	"strconv"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
)

func Sync(p *plex.Client, d *sql.DB, force bool) error {
	// Simple check: if DB has items, maybe skip unless force?
	// For now, we always sync but optimistically (insert only if not exists)
	// Or we use INSERT OR REPLACE.
	// The original python used a check on file mtime.

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
		if s.Type == "movie" {
			videos, err := p.GetSectionAll(s.Key)
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
				_, err := tx.Exec(`INSERT OR REPLACE INTO films (id, title, year, part_key, duration, summary, rating, genres, originallyAvailableAt) 
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					v.RatingKey, v.Title, v.Year, partKey, v.Duration, v.Summary, v.Rating, genres, v.OriginallyAvailableAt)
				if err != nil {
					log.Printf("Error inserting movie %s: %v", v.Title, err)
				}
			}
		} else if s.Type == "show" {
			shows, err := p.GetSectionAll(s.Key)
			if err != nil {
				log.Printf("Error fetching shows for section %s: %v", s.Title, err)
				continue
			}
			for _, show := range shows { // This gets all items, check if it returns episodes? No, section/all returns Shows.
				// We need to fetch episodes. Ideally we iterate sections.
				// Wait, Plex /library/sections/X/all for type=show returns Shows.
				// We then need to recurse? Or can we get all episodes?
				// Using /library/sections/X/all?type=4 (Episode) might work to get all episodes flatly?
				// But we need the hierarchy for the UI.
				// Let's stick to inserting Shows, then maybe we can lazily fetch seasons? 
				// The original script fetched seasons() and episodes().
				
				// Sticking to original logic:
				genres := joinTags(show.Genre)
				_, err := tx.Exec(`INSERT OR REPLACE INTO series (id, title, summary, rating, genres) 
                    VALUES (?, ?, ?, ?, ?)`,
					show.RatingKey, show.Title, show.Summary, show.Rating, genres)
				if err != nil {
					log.Printf("Error inserting show %s: %v", show.Title, err)
				}

				// Fetch children (Seasons)?
				// Plex API has /library/metadata/<ratingKey>/children for seasons
				// Then /library/metadata/<seasonRatingKey>/children for episodes
				
				// For simplicity/performance in this Go rewrite, let's implement a recursive fetcher helper in Plex package
				// Or use the ?includeChildren=1 or similar? No.
				
				// Let's recurse.
				if err := syncSeasons(p, tx, show.RatingKey); err != nil {
					log.Printf("Error fetching seasons for %s: %v", show.Title, err)
				}
			}
		}
	}

	return tx.Commit()
}

func syncSeasons(p *plex.Client, tx *sql.Tx, showID string) error {
	seasons, _, err := p.GetChildren(showID)
	if err != nil {
		return err
	}

	for _, season := range seasons {
		if season.Type != "season" {
			continue
		}
		// Season metadata (summary, etc) is sparse in children view usually, but we have title/index.
		// Key is usually /library/metadata/ID/children
		// RatingKey is the ID.
		
		// Wait, Directory struct in plex package needs RatingKey.
		// I missed adding RatingKey to Directory struct in plex/plex.go! 
		// I should check plex/plex.go again.
		// Assuming I fix plex/plex.go, let's write usage here.
		
		sIndex, _ := strconv.Atoi(season.Index) // Need to add Index/RatingKey to Directory struct
		_, err := tx.Exec(`INSERT OR REPLACE INTO saisons (id, serie_id, saison_index, summary) 
			VALUES (?, ?, ?, ?)`,
			season.RatingKey, showID, sIndex, season.Summary)
		if err != nil {
			log.Printf("Error inserting season %s: %v", season.Title, err)
		}

		// Fetch episodes for this season
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
			_, err := tx.Exec(`INSERT OR REPLACE INTO episodes (id, saison_id, episode_index, title, part_key, duration, summary, rating) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				e.RatingKey, season.RatingKey, e.Index, e.Title, partKey, e.Duration, e.Summary, e.Rating)
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
