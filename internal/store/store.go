package store

import (
	"database/sql"
	"strings"

	"github.com/Waddenn/plex-client/internal/plex"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListMovies() ([]plex.Video, error) {
	const query = `SELECT id, title, year, part_key, duration, summary, rating, genres, originallyAvailableAt, content_rating, studio, added_at FROM films`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []plex.Video
	for rows.Next() {
		var v plex.Video
		var genres string
		if err := rows.Scan(&v.RatingKey, &v.Title, &v.Year, &v.Key, &v.Duration, &v.Summary, &v.Rating, &genres, &v.OriginallyAvailableAt, &v.ContentRating, &v.Studio, &v.AddedAt); err != nil {
			return nil, err
		}
		v.Type = "movie"
		applyGenres(&v, genres)
		videos = append(videos, v)
	}
	return videos, nil
}

func (s *Store) ListSeries() ([]plex.Video, error) {
	const query = `SELECT id, title, summary, rating, genres, content_rating, studio, added_at FROM series`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []plex.Video
	for rows.Next() {
		var v plex.Video
		var genres string
		if err := rows.Scan(&v.RatingKey, &v.Title, &v.Summary, &v.Rating, &genres, &v.ContentRating, &v.Studio, &v.AddedAt); err != nil {
			return nil, err
		}
		v.Type = "show"
		applyGenres(&v, genres)
		videos = append(videos, v)
	}
	return videos, nil
}

func applyGenres(v *plex.Video, genres string) {
	if genres == "" {
		return
	}
	for _, g := range strings.Split(genres, ", ") {
		v.Genre = append(v.Genre, plex.Tag{Tag: g})
	}
}
