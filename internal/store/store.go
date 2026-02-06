package store

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
)

type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{DB: db}
}

// MediaInfo centralizes media-related fields for easier maintenance.
// To add a new field: add it here, update Columns(), Pointers(), and ToPlexMedia().
type MediaInfo struct {
	VideoResolution string
	VideoCodec      string
	AudioCodec      string
	AudioChannels   sql.NullInt64
}

// Columns returns the SQL column names for media fields.
func (MediaInfo) Columns() string {
	return "video_resolution, video_codec, audio_codec, audio_channels"
}

// Pointers returns pointers for sql.Scan.
func (m *MediaInfo) Pointers() []interface{} {
	return []interface{}{&m.VideoResolution, &m.VideoCodec, &m.AudioCodec, &m.AudioChannels}
}

// ToPlexMedia converts MediaInfo to plex.Media.
func (m *MediaInfo) ToPlexMedia() plex.Media {
	return plex.Media{
		VideoResolution: m.VideoResolution,
		VideoCodec:      m.VideoCodec,
		AudioCodec:      m.AudioCodec,
		AudioChannels:   int(m.AudioChannels.Int64),
	}
}

// ApplyTo sets the Media field on a plex.Video.
func (m *MediaInfo) ApplyTo(v *plex.Video) {
	v.Media = []plex.Media{m.ToPlexMedia()}
}

func (s *Store) ListMovies() ([]plex.Video, error) {
	var m MediaInfo
	query := `SELECT id, title, year, part_key, duration, rating, added_at, summary, genres, directors, "cast", originallyAvailableAt, content_rating, studio, ` + m.Columns() + ` FROM films`
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []plex.Video
	for rows.Next() {
		var v plex.Video
		var genres, directors, cast string
		var media MediaInfo
		scanArgs := append([]interface{}{
			&v.RatingKey, &v.Title, &v.Year, &v.Key, &v.Duration, &v.Rating, &v.AddedAt,
			&v.Summary, &genres, &directors, &cast, &v.OriginallyAvailableAt, &v.ContentRating, &v.Studio,
		}, media.Pointers()...)
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		v.Type = "movie"
		applyCommonFields(&v, genres, directors, cast)
		media.ApplyTo(&v)
		videos = append(videos, v)
	}
	return videos, nil
}

func (s *Store) GetVideoMetadata(id string, isEpisode bool) (*plex.Video, error) {
	var v plex.Video
	var media MediaInfo

	if isEpisode {
		query := `SELECT summary, rating, ` + media.Columns() + ` FROM episodes WHERE id = ?`
		scanArgs := append([]interface{}{&v.Summary, &v.Rating}, media.Pointers()...)
		if err := s.DB.QueryRow(query, id).Scan(scanArgs...); err != nil {
			return nil, err
		}
		media.ApplyTo(&v)
		return &v, nil
	}

	var genres, directors, cast string
	query := `SELECT summary, genres, directors, "cast", originallyAvailableAt, content_rating, studio, ` + media.Columns() + ` FROM films WHERE id = ?`
	scanArgs := append([]interface{}{&v.Summary, &genres, &directors, &cast, &v.OriginallyAvailableAt, &v.ContentRating, &v.Studio}, media.Pointers()...)
	if err := s.DB.QueryRow(query, id).Scan(scanArgs...); err != nil {
		return nil, err
	}
	applyCommonFields(&v, genres, directors, cast)
	media.ApplyTo(&v)
	return &v, nil
}

func (s *Store) ListSeries() ([]plex.Video, error) {
	const query = `SELECT id, title, rating, added_at, summary, genres, directors, "cast", content_rating, studio FROM series`
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []plex.Video
	for rows.Next() {
		var v plex.Video
		var genres, directors, cast string
		if err := rows.Scan(
			&v.RatingKey, &v.Title, &v.Rating, &v.AddedAt,
			&v.Summary, &genres, &directors, &cast, &v.ContentRating, &v.Studio,
		); err != nil {
			return nil, err
		}
		v.Type = "show"
		applyCommonFields(&v, genres, directors, cast)
		videos = append(videos, v)
	}
	return videos, nil
}

func (s *Store) GetSeriesMetadata(id string) (*plex.Video, error) {
	var v plex.Video
	var genres, directors, cast string
	err := s.DB.QueryRow(`SELECT summary, genres, directors, "cast", content_rating, studio FROM series WHERE id = ?`, id).Scan(
		&v.Summary, &genres, &directors, &cast, &v.ContentRating, &v.Studio)
	if err != nil {
		return nil, err
	}
	applyCommonFields(&v, genres, directors, cast)
	return &v, nil
}

func (s *Store) GetBatchMetadata(ids []string, itemType string) (map[string]*plex.Video, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var m MediaInfo
	table := "films"
	var fields string
	if itemType == "show" {
		table = "series"
		fields = "id, summary, genres, directors, \"cast\", content_rating, studio"
	} else if itemType == "episode" {
		table = "episodes"
		fields = "id, summary, rating, " + m.Columns()
	} else {
		fields = "id, summary, genres, directors, \"cast\", originallyAvailableAt, content_rating, studio, " + m.Columns()
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT " + fields + " FROM " + table + " WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]*plex.Video)
	for rows.Next() {
		var v plex.Video
		var id string
		var genres, directors, cast string
		var media MediaInfo

		if itemType == "show" {
			if err := rows.Scan(&id, &v.Summary, &genres, &directors, &cast, &v.ContentRating, &v.Studio); err != nil {
				return nil, err
			}
			applyCommonFields(&v, genres, directors, cast)
		} else if itemType == "episode" {
			scanArgs := append([]interface{}{&id, &v.Summary, &v.Rating}, media.Pointers()...)
			if err := rows.Scan(scanArgs...); err != nil {
				return nil, err
			}
			media.ApplyTo(&v)
		} else {
			scanArgs := append([]interface{}{&id, &v.Summary, &genres, &directors, &cast, &v.OriginallyAvailableAt, &v.ContentRating, &v.Studio}, media.Pointers()...)
			if err := rows.Scan(scanArgs...); err != nil {
				return nil, err
			}
			applyCommonFields(&v, genres, directors, cast)
			media.ApplyTo(&v)
		}
		v.RatingKey = id
		v.Type = itemType
		results[id] = &v
	}
	return results, nil
}

func (s *Store) ListSections(targetType string) ([]plex.Directory, error) {
	const query = `SELECT key, title, type, updated_at FROM sections WHERE type = ?`
	rows, err := s.DB.Query(query, targetType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []plex.Directory
	for rows.Next() {
		var d plex.Directory
		if err := rows.Scan(&d.Key, &d.Title, &d.Type, &d.UpdatedAt); err != nil {
			return nil, err
		}
		sections = append(sections, d)
	}
	return sections, nil
}

func (s *Store) ListSeasons(seriesID string) ([]plex.Directory, error) {
	const query = `SELECT id, season_index, summary FROM seasons WHERE series_id = ? ORDER BY season_index`
	rows, err := s.DB.Query(query, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []plex.Directory
	for rows.Next() {
		var d plex.Directory
		var idx int
		if err := rows.Scan(&d.RatingKey, &idx, &d.Summary); err != nil {
			return nil, err
		}
		d.Index = strconv.Itoa(idx)
		d.Type = "season"
		seasons = append(seasons, d)
	}
	return seasons, nil
}

func (s *Store) ListEpisodes(seasonID string) ([]plex.Video, error) {
	var m MediaInfo
	query := `SELECT id, episode_index, title, part_key, duration, rating, summary, ` + m.Columns() + ` FROM episodes WHERE season_id = ? ORDER BY episode_index`
	rows, err := s.DB.Query(query, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []plex.Video
	for rows.Next() {
		var v plex.Video
		var media MediaInfo
		scanArgs := append([]interface{}{&v.RatingKey, &v.Index, &v.Title, &v.Key, &v.Duration, &v.Rating, &v.Summary}, media.Pointers()...)
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		v.Type = "episode"
		v.ParentRatingKey = seasonID
		media.ApplyTo(&v)
		episodes = append(episodes, v)
	}
	return episodes, nil
}

func applyCommonFields(v *plex.Video, genres, directors, cast string) {
	applyGenres(v, genres)
	applyDirectors(v, directors)
	applyCast(v, cast)
}

func applyGenres(v *plex.Video, genres string) {
	if genres == "" {
		return
	}
	for _, g := range strings.Split(genres, ", ") {
		v.Genre = append(v.Genre, plex.Tag{Tag: g})
	}
}

func applyDirectors(v *plex.Video, directors string) {
	if directors == "" {
		return
	}
	for _, d := range strings.Split(directors, ", ") {
		v.Director = append(v.Director, plex.Tag{Tag: d})
	}
}

func applyCast(v *plex.Video, cast string) {
	if cast == "" {
		return
	}
	// Format: "ActorName:CharacterRole, ActorName2:CharacterRole2"
	for _, c := range strings.Split(cast, ", ") {
		parts := strings.SplitN(c, ":", 2)
		if len(parts) == 2 {
			v.Role = append(v.Role, plex.Role{
				Tag:  parts[0],
				Role: parts[1],
			})
		}
	}
}
