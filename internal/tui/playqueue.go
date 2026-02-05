package tui

import (
	"fmt"

	"github.com/Waddenn/plex-client/internal/plex"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
)

// fetchPlayQueue creates a Play Queue from an episode
func fetchPlayQueue(p *plex.Client, initialVideo plex.Video) tea.Cmd {
	return func() tea.Msg {
		// Fetch full metadata to ensure we have parent keys (if not already present)
		// We might need to refresh because Dashboard OnDeck might be partial
		video, err := p.GetMetadata(initialVideo.RatingKey)
		if err != nil {
			return shared.MsgError{Err: fmt.Errorf("failed to get metadata: %w", err)}
		}

		// Create Play Queue
		pq, err := p.CreatePlayQueue(*video)
		if err != nil {
			return shared.MsgError{Err: fmt.Errorf("failed to create play queue: %w", err)}
		}

		// Find index of our starting item
		startIndex := 0
		for i, item := range pq.Items {
			if item.RatingKey == initialVideo.RatingKey {
				startIndex = i
				break
			}
		}

		return MsgQueueLoaded{Queue: pq.Items, Index: startIndex}
	}
}
