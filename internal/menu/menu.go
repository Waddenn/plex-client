package menu

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/db"
	"github.com/Waddenn/plex-client/internal/player"
	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/ui"
)

// ShowMain displays the main menu and handles the navigation loop
func ShowMain(d *sql.DB, cfg *config.Config, p *plex.Client) {
	for {
		res, err := ui.RunFZF([]string{"Continue Watching", "Movies", "Series"}, "🎯 Choose: ", "", nil)
		if err != nil || res == nil || res.Choice == "" {
			break
		}

		switch res.Choice {
		case "Continue Watching":
			ShowContinueWatching(p, cfg)
		case "Movies":
			ShowMovies(d, cfg, p)
		case "Series":
			ShowSeries(d, cfg, p)
		}
	}
}

// ShowContinueWatching displays continue watching items from all sections
func ShowContinueWatching(p *plex.Client, cfg *config.Config) {
	// Not caching OnDeck for now, fetching live
	onDeck, err := getOnDeckAll(p)
	if err != nil {
		fmt.Println("Error fetching Continue Watching:", err)
		return
	}
	if len(onDeck) == 0 {
		fmt.Println("No active items.")
		return
	}

	// Prepare items
	var items []string
	keyMap := make(map[string]plex.Video)
	for i, v := range onDeck {
		// Format similar to python: "Title (Year) [Show] - 45% watched"
		percent := 0
		if v.Duration > 0 {
			percent = int((float64(v.ViewOffset) / float64(v.Duration)) * 100)
		}
		label := fmt.Sprintf("%s - %s [%d%%] #%d", v.GrandparentTitle, v.Title, percent, i)
		if v.Type == "movie" {
			label = fmt.Sprintf("%s (%d) [%d%%] #%d", v.Title, v.Year, percent, i)
		}
		items = append(items, label)
		keyMap[label] = v
	}

	res, err := ui.RunFZF(items, "⏯️ Continue Watching: ", "", nil)
	if err != nil || res == nil || res.Choice == "" {
		return
	}

	v := keyMap[res.Choice]
	partKey := ""
	if len(v.Media) > 0 && len(v.Media[0].Part) > 0 {
		partKey = v.Media[0].Part[0].Key
	}
	if partKey == "" {
		return
	}

	// Calculate resume time in seconds for mpv --start
	start := v.ViewOffset / 1000
    // v.RatingKey is already string from API
	player.Play(v.Title, cfg.Plex.BaseURL+partKey, v.RatingKey, cfg, p, fmt.Sprintf("--start=%d", start))
}

// getOnDeckAll fetches continue watching items from all sections
func getOnDeckAll(p *plex.Client) ([]plex.Video, error) {
	sections, err := p.GetSections()
	if err != nil {
		return nil, err
	}
	var all []plex.Video
	for _, s := range sections {
		vids, err := p.GetOnDeck(s.Key)
		if err == nil {
			all = append(all, vids...)
		}
	}
	return all, nil
}

// ShowMovies displays the movie browser
func ShowMovies(d *sql.DB, cfg *config.Config, p *plex.Client) {
	rows, err := d.Query("SELECT id, title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
	if err != nil {
		return
	}
	defer rows.Close()

	var items []string
	type MovieInfo struct {
		PartKey string
		Title   string
        ID      int
	}
	infoMap := make(map[string]MovieInfo)

	for rows.Next() {
		var id int
		var title, partKey string
		var year int
		if err := rows.Scan(&id, &title, &year, &partKey); err != nil {
			continue
		}
		label := fmt.Sprintf("%s (%d) |%d|", title, year, id)
		items = append(items, label)
		infoMap[label] = MovieInfo{PartKey: partKey, Title: title, ID: id}
	}

	exe, _ := os.Executable()
	previewCmd := fmt.Sprintf("%s --preview $(echo {} | awk -F'|' '{print $2}') --preview-type movie", exe)

	res, err := ui.RunFZF(items, "🎬 Movie: ", previewCmd, nil)
	if err != nil || res == nil || res.Choice == "" {
		return
	}

	info := infoMap[res.Choice]
	if info.PartKey != "" {
		args := []string{}
		if meta, err := p.GetMetadata(strconv.Itoa(info.ID)); err == nil && meta.ViewOffset > 0 {
			start := meta.ViewOffset / 1000
			args = append(args, fmt.Sprintf("--start=%d", start))
		}
		player.Play(info.Title, cfg.Plex.BaseURL+info.PartKey, strconv.Itoa(info.ID), cfg, p, args...)
	}
}

// ShowSeries displays the series browser
func ShowSeries(d *sql.DB, cfg *config.Config, p *plex.Client) {
	rows, err := d.Query("SELECT id, title FROM series ORDER BY title COLLATE NOCASE")
	if err != nil {
		return
	}
	defer rows.Close()

	var items []string
	idMap := make(map[string]int)

	for rows.Next() {
		var id int
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}
		label := fmt.Sprintf("%s |%d|", title, id)
		items = append(items, label)
		idMap[label] = id
	}

	exe, _ := os.Executable()
	previewCmd := fmt.Sprintf("%s --preview $(echo {} | awk -F'|' '{print $2}') --preview-type series", exe)

	res, err := ui.RunFZF(items, "📺 Series: ", previewCmd, nil)
	if err != nil || res == nil || res.Choice == "" {
		return
	}

	showID := idMap[res.Choice]
	showSeasons(d, cfg, p, showID)
}

// showSeasons displays seasons for a given show
func showSeasons(d *sql.DB, cfg *config.Config, p *plex.Client, showID int) {
	rows, err := d.Query("SELECT id, saison_index FROM saisons WHERE serie_id = ? ORDER BY saison_index", showID)
	if err != nil {
		return
	}
	defer rows.Close()

	var items []string
	idMap := make(map[string]int)

	for rows.Next() {
		var id, idx int
		if err := rows.Scan(&id, &idx); err != nil {
			continue
		}
		label := fmt.Sprintf("Season %d |%d|", idx, id)
		items = append(items, label)
		idMap[label] = id
	}

	res, err := ui.RunFZF(items, "📂 Season: ", "", nil) // No preview for season for now
	if err != nil || res == nil || res.Choice == "" {
		return
	}

	seasonID := idMap[res.Choice]
	showEpisodes(d, cfg, p, seasonID)
}

// showEpisodes displays episodes for a given season
func showEpisodes(d *sql.DB, cfg *config.Config, p *plex.Client, seasonID int) {
	rows, err := d.Query("SELECT id, episode_index, title, part_key FROM episodes WHERE saison_id = ? ORDER BY episode_index", seasonID)
	if err != nil {
		return
	}
	defer rows.Close()

	var items []string
	infoMap := make(map[string]struct{ Title, PartKey string; ID int })

	for rows.Next() {
		var id, idx int
		var title, partKey string
		if err := rows.Scan(&id, &idx, &title, &partKey); err != nil {
			continue
		}
		label := fmt.Sprintf("%02d. %s |%d|", idx, title, id)
		items = append(items, label)
		infoMap[label] = struct{ Title, PartKey string; ID int }{title, partKey, id}
	}

	exe, _ := os.Executable()
	previewCmd := fmt.Sprintf("%s --preview $(echo {} | awk -F'|' '{print $2}') --preview-type episode", exe)

	res, err := ui.RunFZF(items, "🎞️ Episode: ", previewCmd, nil)
	if err != nil || res == nil || res.Choice == "" {
		return
	}

	info := infoMap[res.Choice]
	if info.PartKey != "" {
		args := []string{}
		if meta, err := p.GetMetadata(strconv.Itoa(info.ID)); err == nil && meta.ViewOffset > 0 {
			start := meta.ViewOffset / 1000
			args = append(args, fmt.Sprintf("--start=%d", start))
		}
		player.Play(info.Title, cfg.Plex.BaseURL+info.PartKey, strconv.Itoa(info.ID), cfg, p, args...)
	}
}

// RunPreview displays preview information for items (used by fzf preview)
func RunPreview(idStr, pType string) error {
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil {
		return err
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	if pType == "movie" {
		var title, summary, genres, date string
		var duration int
		var rating float64
		err := d.QueryRow("SELECT title, summary, genres, originallyAvailableAt, duration, rating FROM films WHERE id=?", id).Scan(&title, &summary, &genres, &date, &duration, &rating)
		if err != nil {
			return err
		}
		fmt.Printf("🎬 %s\n\n", title)
		fmt.Printf("🕒 Duration: %d min\n", duration/60000)
		fmt.Printf("⭐ Rating: %.1f\n", rating)
		fmt.Printf("📅 Date: %s\n", date)
		fmt.Printf("🎭 Genres: %s\n\n", genres)
		fmt.Println("🧾 Synopsis:")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println(summary)
	} else if pType == "series" {
		var title, summary, genres string
		var rating float64
		err := d.QueryRow("SELECT title, summary, genres, rating FROM series WHERE id=?", id).Scan(&title, &summary, &genres, &rating)
		if err != nil {
			return err
		}
		fmt.Printf("📺 %s\n\n", title)
		fmt.Printf("⭐ Rating: %.1f\n", rating)
		fmt.Printf("🎭 Genres: %s\n\n", genres)
		fmt.Println("🧾 Synopsis:")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println(summary)
	} else if pType == "episode" {
		var title, summary string
		var duration int
		var rating float64
		err := d.QueryRow("SELECT title, summary, duration, rating FROM episodes WHERE id=?", id).Scan(&title, &summary, &duration, &rating)
		if err != nil {
			return err
		}
		fmt.Printf("🎞️ %s\n\n", title)
		fmt.Printf("🕒 Duration: %d min\n", duration/60000)
		fmt.Printf("⭐ Rating: %.1f\n", rating)
		fmt.Println("🧾 Synopsis:")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println(summary)
	}
	return nil
}

