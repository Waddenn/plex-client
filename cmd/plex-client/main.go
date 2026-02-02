package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Waddenn/plex-client/internal/cache"
	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/db"
	"github.com/Waddenn/plex-client/internal/menu"
	"github.com/Waddenn/plex-client/internal/plex"
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
		if err := menu.RunPreview(*previewCmd, *previewType); err != nil {
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

	menu.ShowMain(d, cfg, p)
}
