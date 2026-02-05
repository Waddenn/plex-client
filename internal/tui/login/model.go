package login

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Waddenn/plex-client/internal/appinfo"
	"github.com/Waddenn/plex-client/internal/auth"
	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MsgLoginSuccess is sent when login is complete and configured
type MsgLoginSuccess struct {
	Config *config.Config
}

type Model struct {
	cfg        *config.Config
	authClient *auth.AuthClient
	pin        *auth.PlexPin
	authLink   string
	err        error

	width, height int
	state         state
	focus         int // 0: Open Browser, 1: Cancel
	servers       []auth.PlexResource
	selectedConn  string // BaseURL
}

type state int

const (
	stateInit state = iota
	stateWaitingForPin
	stateConfirmOpen
	statePolling
	stateSearchingServers
	stateSuccess
)

func NewModel(cfg *config.Config, info appinfo.Info) Model {
	clientID := cfg.Plex.ClientIdentifier
	if clientID == "" {
		clientID = "plex-client-tui-fallback"
	}

	return Model{
		cfg:        cfg,
		authClient: auth.NewAuthClient(clientID, info),
		state:      stateInit,
	}
}

func (m Model) Init() tea.Cmd {
	return m.getPinCmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

		if m.state == stateConfirmOpen {
			switch msg.String() {
			case "left", "h", "tab":
				m.focus = 0
			case "right", "l", "shift+tab":
				m.focus = 1
			case "enter", "o", "y":
				if m.focus == 0 {
					m.state = statePolling
					return m, openBrowserCmd(m.authLink)
				}
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case msgPinReady:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.pin = msg.pin
		m.authLink = msg.authLink
		m.state = stateConfirmOpen
		return m, pollCmd(m.pin.ID, m.authClient)

	case msgPollResult:
		if msg.err != nil {
			// Poll again? Or fatal?
			// For now, simple retry logic is inside pollCmd wrapper usually,
			// but here we just get a result.
			// If error is just "not ready", we assume pollCmd handles wait?
			// Let's make pollCmd wait and retry internally or return "not ready" signal?
			// Simpler: The poll command waits a bit and checks.
			return m, pollCmd(m.pin.ID, m.authClient)
		}
		if msg.pin.AuthToken != "" {
			m.cfg.Plex.Token = msg.pin.AuthToken // Save token temporarily
			m.state = stateSearchingServers
			return m, getResourcesCmd(m.authClient, msg.pin.AuthToken)
		}
		// Not ready yet, poll again
		return m, pollCmd(m.pin.ID, m.authClient)

	case msgResourcesReady:
		if msg.err != nil {
			m.err = msg.err
			return m, nil // Wait for user to quit?
		}

		// Auto-select best connection
		bestURL := selectBestConnection(msg.resources)
		if bestURL == "" {
			m.err = fmt.Errorf("no suitable Plex server found")
			return m, nil
		}

		m.cfg.Plex.BaseURL = bestURL
		// Save config
		if err := config.Save(m.cfg); err != nil {
			m.err = err
			return m, nil
		}

		return m, func() tea.Msg {
			return MsgLoginSuccess{Config: m.cfg}
		}
	}

	return m, nil
}

func (m Model) View() string {
	frame := shared.StyleBorder.Copy()
	innerWidth := m.width - frame.GetHorizontalFrameSize()
	innerHeight := m.height - frame.GetVerticalFrameSize()
	if innerWidth < 20 {
		innerWidth = 20
	}
	if innerHeight < 5 {
		innerHeight = 5
	}

	if m.err != nil {
		content := lipgloss.JoinVertical(lipgloss.Center,
			shared.StyleTitle.Render("❌ Login Error"),
			"",
			shared.StyleItemNormal.Render(m.err.Error()),
			"",
			shared.StyleDim.Render("Press q to quit"),
		)
		card := lipgloss.NewStyle().Width(min(innerWidth, 60)).Render(content)
		return frame.Render(lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, card))
	}

	var content string
	switch m.state {
	case stateInit, stateWaitingForPin:
		content = lipgloss.JoinVertical(lipgloss.Center,
			shared.StyleTitle.Render("🔄 Initializing Login..."),
			"",
			shared.StyleDim.Render("Connecting to Plex.tv"),
		)

	case stateConfirmOpen, statePolling:
		var status string
		var tip string

		if m.state == stateConfirmOpen {
			btnStyle := lipgloss.NewStyle().
				Padding(0, 3).
				Margin(1, 1).
				Background(shared.ColorDarkGrey).
				Foreground(shared.ColorWhite)

			activeBtnStyle := btnStyle.Copy().
				Background(shared.ColorPlexOrange).
				Foreground(shared.ColorBlack).
				Bold(true)

			openBtn := btnStyle.Render("Open Browser")
			cancelBtn := btnStyle.Render("Cancel")

			if m.focus == 0 {
				openBtn = activeBtnStyle.Render("Open Browser")
			} else {
				cancelBtn = activeBtnStyle.Copy().
					Background(lipgloss.Color("#444444")).
					Foreground(shared.ColorWhite).
					Render("Cancel")
			}

			status = lipgloss.JoinHorizontal(lipgloss.Center, openBtn, cancelBtn)
			tip = shared.StyleDim.Render("(Use Arrows/Tab to move, Enter to select)")
		} else {
			status = shared.StyleItemActive.Render("Please sign in via your browser.")
			tip = shared.StyleItemNormal.Render("⏳ Waiting for authorization...")
		}

		content = lipgloss.JoinVertical(lipgloss.Center,
			shared.StyleTitle.Render("🔐 Plex Authentication"),
			"",
			status,
			tip,
			"",
			shared.StyleSecondary.Render("Link: "+m.authLink),
		)

	case stateSearchingServers:
		content = lipgloss.JoinVertical(lipgloss.Center,
			shared.StyleTitle.Render("🔍 Discovering Servers"),
			"",
			shared.StyleItemNormal.Render("Locating your Plex Media Server..."),
		)

	case stateSuccess:
		content = lipgloss.JoinVertical(lipgloss.Center,
			shared.StyleTitle.Render("✅ Login Successful!"),
			"",
			shared.StyleItemNormal.Render("Preparing your dashboard..."),
		)
	}

	card := lipgloss.NewStyle().Width(min(innerWidth, 60)).Render(content)
	return frame.Render(lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, card))
}

// -- Commands --

type msgPinReady struct {
	pin      *auth.PlexPin
	authLink string
	err      error
}

func (m Model) getPinCmd() tea.Msg {
	pin, link, err := m.authClient.GetPin()
	return msgPinReady{pin, link, err}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("xdg-open", url).Start()
		return nil
	}
}

type msgPollResult struct {
	pin *auth.PlexPin
	err error
}

func pollCmd(pinID int, client *auth.AuthClient) tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		p, err := client.CheckPin(pinID)
		return msgPollResult{p, err}
	})
}

type msgResourcesReady struct {
	resources []auth.PlexResource
	err       error
}

func getResourcesCmd(client *auth.AuthClient, token string) tea.Cmd {
	return func() tea.Msg {
		res, err := client.GetResources(token)
		return msgResourcesReady{res, err}
	}
}

// Helper logic
func selectBestConnection(resources []auth.PlexResource) string {
	for _, res := range resources {
		if res.Provides == "server" || res.Product == "Plex Media Server" {
			if len(res.Connections) > 0 {
				sort.Slice(res.Connections, func(i, j int) bool {
					return getConnectionScore(res.Connections[i]) > getConnectionScore(res.Connections[j])
				})
				return res.Connections[0].Uri
			}
		}
	}
	return ""
}

// Duplicate of main.go logic logic for now, should move to auth/utils?
func getConnectionScore(conn auth.PlexConnection) int {
	if !conn.Local {
		return 3
	}
	if strings.HasPrefix(conn.Address, "172.") {
		return 1
	}
	return 2
}
