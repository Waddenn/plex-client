package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Waddenn/plex-client/internal/cache"
	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/db"
	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		baseURLFlag = flag.String("baseurl", "", "Plex server BaseURL")
		tokenFlag   = flag.String("token", "", "Plex Token")
		forceSync   = flag.Bool("force-sync", false, "Force full cache sync")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Apply flags to config
	if *baseURLFlag != "" {
		cfg.Plex.BaseURL = *baseURLFlag
	}
	if *tokenFlag != "" {
		cfg.Plex.Token = *tokenFlag
	}

	// Save config if flags were provided
	if *baseURLFlag != "" || *tokenFlag != "" {
		if err := config.Save(cfg); err != nil {
			log.Printf("Warning: failed to save config: %v", err)
		} else {
			dir, _ := config.ConfigDir()
			fmt.Printf("✅ Configuration saved to %s/config.toml\n", dir)
		}
	}

	// Check for commands
	if len(os.Args) > 1 && os.Args[1] == "login" {
		fmt.Println("ℹ️  Login is now handled directly in the TUI.")
	}

	d, err := db.Open()
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer d.Close()

	p := plex.New(cfg.Plex.BaseURL, cfg.Plex.Token, "plex-client-go")

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

	// Use config sync settings
	forceSyncFlag := *forceSync || cfg.Sync.ForceSyncOnStart

	// Only attempt sync if we are authenticated
	if cfg.Plex.Token != "" {
		if !hasData || forceSyncFlag {
			fmt.Println("🚀 Syncing library for the first time... This might take a while.")
			if err := cache.Sync(p, d, forceSyncFlag); err != nil {
				log.Printf("Sync error: %v", err)
			}
		} else if cfg.Sync.AutoSync {
			// Background sync
			go func() {
				if err := cache.Sync(p, d, false); err != nil {
					log.Printf("Background sync error: %v", err)
				}
			}()
		}
	}

	m := tui.NewModel(d, cfg, p)
	if _, err := tea.NewProgram(&m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
