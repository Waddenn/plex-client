package shared

import "github.com/charmbracelet/lipgloss"

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

	// Styles
	StyleBase = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorBlack)

	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorPlexOrange).
			Bold(true).
			Padding(0, 1)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPlexOrange).
			Padding(1, 2)

	StyleItemNormal = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(ColorWhite)

	StyleItemActive = lipgloss.NewStyle().
			PaddingLeft(0).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(ColorPlexOrange).
			Foreground(ColorPlexOrange).
			Bold(true)

	StyleMetadataKey = lipgloss.NewStyle().
				Foreground(ColorLightGrey)

	StyleMetadataValue = lipgloss.NewStyle().
				Foreground(ColorWhite)

	// Dashboard Specifics
	StyleHero = lipgloss.NewStyle().
			Padding(1, 2).
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
			Width(25).
			Border(lipgloss.RoundedBorder(), false, true, false, false). // Right border only
			BorderForeground(ColorPlexOrange).
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
)

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
)
