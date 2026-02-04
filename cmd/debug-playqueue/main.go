package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/plex"
)

// Helper struct for Root response
type RootContainer struct {
    MachineIdentifier string `xml:"machineIdentifier,attr"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	host, err := os.Hostname()
	if err != nil {
		host = "plex-client-debug"
	}
	p := plex.New(cfg.Plex.BaseURL, cfg.Plex.Token, host)

	// 1. Get Sections
	sections, err := p.GetSections()
	if err != nil {
		log.Fatalf("GetSections failed: %v", err)
	}
	if len(sections) == 0 {
		log.Fatal("No sections found")
	}

	fmt.Printf("Found %d sections. Using first one: %s\n", len(sections), sections[0].Title)

	// Find an episode
	var episode plex.Video
	var item plex.Video
	found := false
	for _, s := range sections {
		fmt.Printf("Checking section: %s (Type: %s, Key: %s)\n", s.Title, s.Type, s.Key)
		
		if s.Type == "show" {
			// Shows are directories
			shows, err := p.GetSectionDirs(s.Key)
			if err != nil {
				fmt.Printf("GetSectionDirs failed: %v\n", err)
				continue
			}
			fmt.Printf("Found %d shows in section %s\n", len(shows), s.Title)
			
			for _, show := range shows {
				// Get Seasons
				seasons, _, err := p.GetChildren(show.RatingKey)
				if err != nil {
					continue
				}
				if len(seasons) > 0 {
					// Get Episodes of first season
					_, episodes, err := p.GetChildren(seasons[0].RatingKey)
					if err != nil {
						continue
					}
					if len(episodes) > 0 {
						episode = episodes[0]
						found = true
						break
					}
				}
			}
		} else if s.Type == "movie" {
			// Fallback to movie
		}
		if found {
			break
		}
	}

	if !found {
		log.Printf("No episode found, falling back to movie selection...")
		// fallback to previous movie logic
		items, _ := p.GetSectionAll(sections[0].Key)
		if len(items) > 0 {
			item = items[0]
		} else {
			log.Fatal("No items found at all")
		}
	} else {
		item = episode
	}

	fmt.Printf("Testing Play Queue with Item: %s (Key: %s, Type: %s)\n", item.Title, item.RatingKey, item.Type)

	// Test refactored CreatePlayQueue
	pq, err := p.CreatePlayQueue(item)
	if err != nil {
		log.Fatalf("CreatePlayQueue failed: %v", err)
	}

	fmt.Printf("Play Queue Created! ID: %s, Item Count: %d\n", pq.PlayQueueID, len(pq.Items))
	for i, v := range pq.Items {
		fmt.Printf("Item %d: %s (Key: %s)\n", i, v.Title, v.RatingKey)
	}
}
