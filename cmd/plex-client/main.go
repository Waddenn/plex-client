package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"strconv"
	
	"github.com/Waddenn/plex-client/internal/cache"
	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/db"
	"github.com/Waddenn/plex-client/internal/player"
	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/ui"
)

func main() {
	var (
		baseURLFlag = flag.String("baseurl", "", "Plex server BaseURL")
		tokenFlag   = flag.String("token", "", "Plex Token")
		forceSync   = flag.Bool("force-sync", false, "Force full cache sync")
		previewCmd  = flag.String("preview", "", "Internal: Execute preview for item ID")
		previewType = flag.String("preview-type", "movie", "Internal: Preview type (movie, series, episode)")
	)
	flag.Parse()

	if *previewCmd != "" {
		if err := runPreview(*previewCmd, *previewType); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if *baseURLFlag != "" {
		cfg.BaseURL = *baseURLFlag
	}
	if *tokenFlag != "" {
		cfg.Token = *tokenFlag
	}

	if *baseURLFlag != "" || *tokenFlag != "" {
		if err := config.Save(cfg); err != nil {
			log.Printf("Warning: failed to save config: %v", err)
		}
	}

	if cfg.BaseURL == "" || cfg.Token == "" {
		fmt.Println("❌ Missing configuration.")
		fmt.Println("Usage: plex-client --baseurl URL --token TOKEN")
		os.Exit(1)
	}

	d, err := db.Open()
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer d.Close()

	p := plex.New(cfg.BaseURL, cfg.Token)

	// Check if we have data
	hasData := false
	var count int
	if err := d.QueryRow("SELECT count(*) FROM films").Scan(&count); err == nil && count > 0 {
		hasData = true
	}
	if !hasData {
		if err := d.QueryRow("SELECT count(*) FROM series").Scan(&count); err == nil && count > 0 {
			hasData = true
		}
	}

	if !hasData || *forceSync {
		fmt.Println("🚀 Syncing library for the first time... This might take a while.")
		if err := cache.Sync(p, d, *forceSync); err != nil {
			log.Printf("Sync error: %v", err)
		}
	} else {
		// update in background
		go func() {
			if err := cache.Sync(p, d, false); err != nil {
				log.Printf("Background sync error: %v", err)
			}
		}()
	}

	mainMenu(d, cfg, p)
}

func mainMenu(d *sql.DB, cfg *config.Config, p *plex.Client) {
	for {
		res, err := ui.RunFZF([]string{"Continue Watching", "Movies", "Series"}, "🎯 Choose: ", "", nil)
		if err != nil || res == nil || res.Choice == "" {
			break
		}

		switch res.Choice {
		case "Continue Watching":
			menuContinueWatching(p, cfg)
		case "Movies":
			menuMovies(d, cfg)
		case "Series":
			menuSeries(d, cfg)
		}
	}
}

func menuContinueWatching(p *plex.Client, cfg *config.Config) {
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
	player.Play(v.Title, cfg.BaseURL+partKey, cfg, fmt.Sprintf("--start=%d", start))
}

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

func menuMovies(d *sql.DB, cfg *config.Config) {
	rows, err := d.Query("SELECT id, title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
	if err != nil {
		return
	}
	defer rows.Close()

	var items []string
	type MovieInfo struct {
		PartKey string
		Title   string
	}
	infoMap := make(map[string]MovieInfo)

	for rows.Next() {
		var id int
		var title, partKey string
		var year int
		if err := rows.Scan(&id, &title, &year, &partKey); err != nil {
			continue
		}
		// FZF trick: "Title (Year) \t ID" and use with-nth to hide ID?
		// But passing ID to preview requires knowing it.
		// We format as: "Title (Year) <HIDDEN_ID>" using multiple spaces?
		// My ui.RunFZF wrapper doesn't support complex fzf opts like --delimiter yet.
		// I will just append ID at end and handle it.
		label := fmt.Sprintf("%s (%d) |%d|", title, year, id)
		items = append(items, label)
		infoMap[label] = MovieInfo{PartKey: partKey, Title: title}
	}

	exe, _ := os.Executable()
	// Preview command: `plex-client --preview {ID} --preview-type movie`
	// We need to extract ID from the line. Using awk or cut in the preview command string?
	// FZF replaces {} with the line.
	// We can do: `plex-client --preview $(echo {} | awk -F'|' '{print $2}') --preview-type movie`
	
	previewCmd := fmt.Sprintf("%s --preview $(echo {} | awk -F'|' '{print $2}') --preview-type movie", exe)

	res, err := ui.RunFZF(items, "🎬 Movie: ", previewCmd, nil)
	if err != nil || res == nil || res.Choice == "" {
		return
	}

	info := infoMap[res.Choice]
	if info.PartKey != "" {
		player.Play(info.Title, cfg.BaseURL+info.PartKey, cfg)
	}
}

func menuSeries(d *sql.DB, cfg *config.Config) {
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
	menuSeasons(d, cfg, showID)
}

func menuSeasons(d *sql.DB, cfg *config.Config, showID int) {
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
	menuEpisodes(d, cfg, seasonID)
}

func menuEpisodes(d *sql.DB, cfg *config.Config, seasonID int) {
	rows, err := d.Query("SELECT id, episode_index, title, part_key FROM episodes WHERE saison_id = ? ORDER BY episode_index", seasonID)
	if err != nil {
		return
	}
	defer rows.Close()
	
	var items []string
	infoMap := make(map[string]struct{Title, PartKey string})
	
	for rows.Next() {
		var id, idx int
		var title, partKey string
		if err := rows.Scan(&id, &idx, &title, &partKey); err != nil {
			continue
		}
		label := fmt.Sprintf("%02d. %s |%d|", idx, title, id)
		items = append(items, label)
		infoMap[label] = struct{Title, PartKey string}{title, partKey}
	}
	
	exe, _ := os.Executable()
	previewCmd := fmt.Sprintf("%s --preview $(echo {} | awk -F'|' '{print $2}') --preview-type episode", exe)

	res, err := ui.RunFZF(items, "🎞️ Episode: ", previewCmd, nil)
	if err != nil || res == nil || res.Choice == "" {
		return
	}
	
	info := infoMap[res.Choice]
	if info.PartKey != "" {
		player.Play(info.Title, cfg.BaseURL+info.PartKey, cfg)
	}
}

func runPreview(idStr, pType string) error {
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
		fmt.Println(summary) // Wrapping not implemented for brevity but FZF wraps usually
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
