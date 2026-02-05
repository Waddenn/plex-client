package shared

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPlexOrange = lipgloss.Color("#e5a00d")
	ColorDarkGrey   = lipgloss.Color("#282a2e")
	ColorBlack      = lipgloss.Color("#1a1a1a")
	ColorWhite      = lipgloss.Color("#ffffff")
	ColorLightGrey  = lipgloss.Color("#b2b2b2")
	ColorRed        = lipgloss.Color("#e52d27")
	ColorDeepRed    = lipgloss.Color("#922b21")
	ColorBackground = lipgloss.Color("#0f0f0f")
	ColorBorder     = lipgloss.Color("#333333")

	// Layout
	FramePaddingX = 2
	FramePaddingY = 1
	SidebarWidth  = 24

	// Styles
	StyleBase = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorBlack)

	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorPlexOrange).
			Bold(true).
			Padding(0, 1)

	StyleHeader = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorPlexOrange)

	StyleFooter = lipgloss.NewStyle().
			Padding(0, 1).
			Background(ColorBlack).
			Foreground(ColorLightGrey)

	StyleBorder = lipgloss.NewStyle().
			Padding(FramePaddingY, FramePaddingX)

	StyleItemNormal = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(ColorWhite)

	StyleItemActive = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(ColorBlack).
			Background(ColorPlexOrange).
			Bold(true)

	StyleMetadataKey = lipgloss.NewStyle().
				Foreground(ColorLightGrey)

	StyleMetadataValue = lipgloss.NewStyle().
				Foreground(ColorWhite)

	// Dashboard Specifics
	StyleHero = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPlexOrange).
			Align(lipgloss.Center)

	StyleSecondary = lipgloss.NewStyle().
			Foreground(ColorLightGrey)

	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorPlexOrange).
			Bold(true)

	StyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	StyleSidebar = lipgloss.NewStyle().
			Width(SidebarWidth).
			Border(lipgloss.NormalBorder(), false, true, false, false). // Right border only
			BorderForeground(ColorBorder).
			Padding(0, 1)

	StyleBadge = lipgloss.NewStyle().
			Foreground(ColorBlack).
			Background(ColorLightGrey).
			Padding(0, 1).
			MarginRight(1).
			Bold(true)

	StyleBadgeOrange = StyleBadge.Copy().
				Background(ColorPlexOrange)

	StyleRole = lipgloss.NewStyle().
			Foreground(ColorPlexOrange).
			Italic(true)

	StyleRightPanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(ColorBorder)
)

// Truncate safely truncates a string to a maximum length with an ellipsis.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max < 2 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

// View represents the current active view
type View int

const (
	ViewDashboard View = iota
	ViewMovieBrowser
	ViewSeriesBrowser
	ViewSeasonBrowser
	ViewEpisodeBrowser
	ViewPlayer
	ViewCountdown
	ViewSettings
	ViewLogin
)
