package cache

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Waddenn/plex-client/internal/plex"
)

// mediaInfo extracts media-related fields from plex.Video for storage.
// To add a new field: update this struct, Columns(), Placeholders(), and Values().
type mediaInfo struct {
	VideoResolution string
	VideoCodec      string
	AudioCodec      string
	AudioChannels   int
}

// extractMediaInfo extracts media info from the first Media entry.
func extractMediaInfo(v plex.Video) mediaInfo {
	if len(v.Media) == 0 {
		return mediaInfo{}
	}
	m := v.Media[0]
	return mediaInfo{
		VideoResolution: m.VideoResolution,
		VideoCodec:      m.VideoCodec,
		AudioCodec:      m.AudioCodec,
		AudioChannels:   m.AudioChannels,
	}
}

// Columns returns SQL column names for INSERT statements.
func (mediaInfo) Columns() string {
	return "video_resolution, video_codec, audio_codec, audio_channels"
}

// Placeholders returns SQL placeholders for INSERT statements.
func (mediaInfo) Placeholders() string {
	return "?, ?, ?, ?"
}

// Values returns the values for SQL INSERT in the same order as Columns().
func (m mediaInfo) Values() []interface{} {
	return []interface{}{m.VideoResolution, m.VideoCodec, m.AudioCodec, m.AudioChannels}
}

// PlexProvider interface allows mocking the Plex client
type PlexProvider interface {
	GetSections() ([]plex.Directory, error)
	GetSectionAll(key string) ([]plex.Directory, []plex.Video, error) // Updated signature
	GetSectionDirs(key string) ([]plex.Directory, error)
	GetChildren(key string) ([]plex.Directory, []plex.Video, error)
}

func Sync(p PlexProvider, d *sql.DB, force bool, onProgress func(status string, added int)) error {
	sections, err := p.GetSections()
	if err != nil {
		return err
	}

	totalAdded := 0
	for _, s := range sections {
		if err := SyncSection(p, d, s, force, &totalAdded, onProgress); err != nil {
			log.Printf("Error syncing section %s: %v", s.Title, err)
		}
	}

	return nil
}

func SyncSection(p PlexProvider, d *sql.DB, s plex.Directory, force bool, totalAdded *int, onProgress func(status string, added int)) error {
	// Incremental sync check
	var lastUpdated int64
	_ = d.QueryRow("SELECT value FROM metadata WHERE key = ?", "section_"+s.Key).Scan(&lastUpdated)

	// Update sections table
	_, _ = d.Exec(`INSERT OR REPLACE INTO sections (key, title, type, updated_at) VALUES (?, ?, ?, ?)`, s.Key, s.Title, s.Type, s.UpdatedAt)

	if !force && s.UpdatedAt > 0 && lastUpdated >= s.UpdatedAt {
		return nil
	}

	if s.Type == "movie" {
		onProgress("Updating "+s.Title, *totalAdded)
		_, videos, err := p.GetSectionAll(s.Key)
		if err != nil {
			return err
		}
		if err := SaveMovies(d, videos, totalAdded, func(count int) { onProgress("Updating "+s.Title, count) }); err != nil {
			return err
		}
	} else if s.Type == "show" {
		onProgress("Updating "+s.Title, *totalAdded)
		shows, err := p.GetSectionDirs(s.Key)
		if err != nil {
			return err
		}
		if err := SaveSeries(d, shows, totalAdded, func(count int) { onProgress("Updating "+s.Title, count) }); err != nil {
			return err
		}

		// Also sync seasons/episodes for these shows
		for _, show := range shows {
			if err := SyncShow(p, d, show.RatingKey, totalAdded, func(count int) { onProgress("Updating "+s.Title, count) }); err != nil {
				log.Printf("Error syncing show %s: %v", show.Title, err)
			}
		}
	}

	// Update metadata
	if s.UpdatedAt > 0 {
		_, _ = d.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)`, "section_"+s.Key, strconv.FormatInt(s.UpdatedAt, 10))
	}

	return nil
}

func SaveMovies(d *sql.DB, videos []plex.Video, totalAdded *int, onProgress func(int)) error {
	// Use BEGIN IMMEDIATE to avoid deadlocks during concurrent writes
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var m mediaInfo
	query := `INSERT OR REPLACE INTO films (id, title, year, part_key, duration, summary, rating, genres, directors, "cast", originallyAvailableAt, content_rating, studio, added_at, updated_at, ` + m.Columns() + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ` + m.Placeholders() + `)`

	for _, v := range videos {
		var existingUpdatedAt int64
		err := tx.QueryRow("SELECT updated_at FROM films WHERE id = ?", v.RatingKey).Scan(&existingUpdatedAt)

		// If it exists and hasn't changed, skip
		if err == nil && v.UpdatedAt > 0 && existingUpdatedAt >= v.UpdatedAt {
			continue
		}

		if err == sql.ErrNoRows {
			if totalAdded != nil {
				*totalAdded++
				if onProgress != nil {
					onProgress(*totalAdded)
				}
			}
		}

		partKey := ""
		if len(v.Media) > 0 && len(v.Media[0].Part) > 0 {
			partKey = v.Media[0].Part[0].Key
		}
		genres := joinTags(v.Genre)
		directors := joinTags(v.Director)
		cast := joinRoles(v.Role)

		// Use Plex's UpdatedAt if available, otherwise fallback to now
		updatedAt := v.UpdatedAt
		if updatedAt == 0 {
			updatedAt = time.Now().Unix()
		}

		media := extractMediaInfo(v)
		args := append([]interface{}{
			v.RatingKey, v.Title, v.Year, partKey, v.Duration, v.Summary, v.Rating,
			genres, directors, cast, v.OriginallyAvailableAt, v.ContentRating, v.Studio, v.AddedAt, updatedAt,
		}, media.Values()...)

		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("Error inserting movie %s: %v", v.Title, err)
		}
	}
	return tx.Commit()
}

func SaveSeries(d *sql.DB, shows []plex.Directory, totalAdded *int, onProgress func(int)) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, show := range shows {
		var existingUpdatedAt int64
		err := tx.QueryRow("SELECT updated_at FROM series WHERE id = ?", show.RatingKey).Scan(&existingUpdatedAt)

		if err == nil && show.UpdatedAt > 0 && existingUpdatedAt >= show.UpdatedAt {
			continue
		}

		if err == sql.ErrNoRows {
			if totalAdded != nil {
				*totalAdded++
				if onProgress != nil {
					onProgress(*totalAdded)
				}
			}
		}

		genres := joinTags(show.Genre)
		directors := joinTags(show.Director)
		cast := joinRoles(show.Role)

		updatedAt := show.UpdatedAt
		if updatedAt == 0 {
			updatedAt = time.Now().Unix()
		}

		_, err = tx.Exec(`INSERT OR REPLACE INTO series (id, title, summary, rating, genres, directors, "cast", content_rating, studio, added_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			show.RatingKey, show.Title, show.Summary, show.Rating, genres, directors, cast, show.ContentRating, show.Studio, show.AddedAt, updatedAt)
		if err != nil {
			log.Printf("Error inserting show %s: %v", show.Title, err)
		}
	}
	return tx.Commit()
}

func SaveSeasons(d *sql.DB, seriesID string, seasons []plex.Directory, added *int, onProgress func(int)) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, season := range seasons {
		if season.Type != "season" {
			continue
		}

		var existingUpdatedAt int64
		err := tx.QueryRow("SELECT updated_at FROM seasons WHERE id = ?", season.RatingKey).Scan(&existingUpdatedAt)

		if err == nil && season.UpdatedAt > 0 && existingUpdatedAt >= season.UpdatedAt {
			continue
		}

		if err == sql.ErrNoRows {
			if added != nil {
				*added++
				if onProgress != nil {
					onProgress(*added)
				}
			}
		}

		sIndex, _ := strconv.Atoi(season.Index)
		updatedAt := season.UpdatedAt
		if updatedAt == 0 {
			updatedAt = time.Now().Unix()
		}

		_, err = tx.Exec(`INSERT OR REPLACE INTO seasons (id, series_id, season_index, summary, updated_at) 
			VALUES (?, ?, ?, ?, ?)`,
			season.RatingKey, seriesID, sIndex, season.Summary, updatedAt)
		if err != nil {
			log.Printf("Error inserting season %s: %v", season.Title, err)
		}
	}
	return tx.Commit()
}

func SyncShow(p PlexProvider, d *sql.DB, showID string, added *int, onProgress func(int)) error {
	seasons, _, err := p.GetChildren(showID)
	if err != nil {
		return err
	}

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, season := range seasons {
		if season.Type != "season" {
			continue
		}

		var existingUpdatedAt int64
		err := tx.QueryRow("SELECT updated_at FROM seasons WHERE id = ?", season.RatingKey).Scan(&existingUpdatedAt)

		if err == nil && season.UpdatedAt > 0 && existingUpdatedAt >= season.UpdatedAt {
			// Even if season hasn't changed, we might want to check its episodes if force sync?
			// But for SyncShow (called during bulk sync or background), let's be conservative.
			// Actually, if we're here, we usually want to ensure episodes are synced too.
			// Let's at least process the episodes.
		}

		if err == sql.ErrNoRows {
			if added != nil {
				*added++
				if onProgress != nil {
					onProgress(*added)
				}
			}
		}

		sIndex, _ := strconv.Atoi(season.Index)
		updatedAt := season.UpdatedAt
		if updatedAt == 0 {
			updatedAt = time.Now().Unix()
		}

		_, err = tx.Exec(`INSERT OR REPLACE INTO seasons (id, series_id, season_index, summary, updated_at) 
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

		if err := saveEpisodesInTx(tx, season.RatingKey, episodes, added, onProgress); err != nil {
			log.Printf("Error saving episodes for season %s: %v", season.Title, err)
		}
	}
	return tx.Commit()
}

func SaveEpisodes(d *sql.DB, seasonID string, episodes []plex.Video, added *int, onProgress func(int)) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := saveEpisodesInTx(tx, seasonID, episodes, added, onProgress); err != nil {
		return err
	}

	return tx.Commit()
}

func saveEpisodesInTx(tx *sql.Tx, seasonID string, episodes []plex.Video, added *int, onProgress func(int)) error {
	var m mediaInfo
	query := `INSERT OR REPLACE INTO episodes (id, season_id, episode_index, title, part_key, duration, summary, rating, updated_at, ` + m.Columns() + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ` + m.Placeholders() + `)`

	for _, e := range episodes {
		var existingUpdatedAt int64
		err := tx.QueryRow("SELECT updated_at FROM episodes WHERE id = ?", e.RatingKey).Scan(&existingUpdatedAt)

		if err == nil && e.UpdatedAt > 0 && existingUpdatedAt >= e.UpdatedAt {
			continue
		}

		if err == sql.ErrNoRows {
			if added != nil {
				*added++
				if onProgress != nil {
					onProgress(*added)
				}
			}
		}

		partKey := ""
		if len(e.Media) > 0 && len(e.Media[0].Part) > 0 {
			partKey = e.Media[0].Part[0].Key
		}

		updatedAt := e.UpdatedAt
		if updatedAt == 0 {
			updatedAt = time.Now().Unix()
		}

		media := extractMediaInfo(e)
		args := append([]interface{}{
			e.RatingKey, seasonID, e.Index, e.Title, partKey, e.Duration, e.Summary, e.Rating, updatedAt,
		}, media.Values()...)

		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("Error inserting episode %s: %v", e.Title, err)
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

func joinRoles(roles []plex.Role) string {
	var s []string
	for _, r := range roles {
		// Format: "ActorName:CharacterRole"
		s = append(s, r.Tag+":"+r.Role)
	}
	return strings.Join(s, ", ")
}
