package browser

import "github.com/Waddenn/plex-client/internal/plex"

// Messages
type MsgSectionsLoaded struct {
	Sections []plex.Directory
	Err      error
}

type MsgItemsLoaded struct {
	SectionKey string
	Items      []plex.Video
	Dirs       []plex.Directory
	Err        error
}

type MsgChildrenLoaded struct {
	ParentID string
	Dirs     []plex.Directory
	Videos   []plex.Video
	Err      error
}
