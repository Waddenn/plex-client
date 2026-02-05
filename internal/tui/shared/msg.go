package shared

// MsgBack requests navigation back to the previous screen
type MsgBack struct{}

// MsgSwitchView requests switching to a specific view
type MsgSwitchView struct {
    View View
    // Optional data to pass to the new view (e.g., Section Key)
    Data interface{}
}

// MsgError reports a global error
type MsgError struct {
    Err error
}

// MsgPlayVideo requests playback of a specific video
type MsgPlayVideo struct {
    Video interface{} // plex.Video
}
