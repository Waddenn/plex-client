package settings

import (
	"fmt"
	"strings"

	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SettingUseCPU = iota
	SettingHWDec
	SettingVO
	SettingToneMapping
	SettingSubtitles
	SettingSubLang
	SettingAudioLang
	SettingIcons
	settingCount
)

type MsgConfigChanged struct {
	Config *config.Config
}

type Model struct {
	cfg    *config.Config
	cursor int
	width  int
	height int
}

func NewModel(cfg *config.Config) Model {
	return Model{
		cfg: cfg,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = settingCount - 1
			}
		case "down", "j":
			if m.cursor < settingCount-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		case "left", "h":
			return m, m.changeSetting(-1)
		case "right", "l", "enter":
			return m, m.changeSetting(1)

		case "esc", "q", "backspace":
			return m, func() tea.Msg { return shared.MsgBack{} }
		}
	}

	return m, nil
}

func (m Model) changeSetting(delta int) tea.Cmd {
	switch m.cursor {
	case SettingUseCPU:
		m.cfg.Player.UseCPU = !m.cfg.Player.UseCPU
	case SettingHWDec:
		options := []string{"auto", "vaapi", "nvdec", "vdpau", "auto-safe", "no"}
		m.cfg.Player.HWDec = rotate(m.cfg.Player.HWDec, options, delta)
	case SettingVO:
		options := []string{"auto", "gpu-next", "gpu", "x11", "xv"}
		m.cfg.Player.VO = rotate(m.cfg.Player.VO, options, delta)
	case SettingToneMapping:
		options := []string{"auto", "st2094-10", "mobius", "spline", "bt.2446a", "clip"}
		m.cfg.Player.ToneMapping = rotate(m.cfg.Player.ToneMapping, options, delta)
	case SettingSubtitles:
		m.cfg.Player.SubtitlesEnabled = !m.cfg.Player.SubtitlesEnabled
	case SettingSubLang:
		langs := []string{"auto", "eng", "fra", "ger", "spa", "ita"}
		m.cfg.Player.SubtitlesLang = rotate(m.cfg.Player.SubtitlesLang, langs, delta)
	case SettingAudioLang:
		langs := []string{"auto", "eng", "fra", "ger", "spa", "ita"}
		m.cfg.Player.AudioLang = rotate(m.cfg.Player.AudioLang, langs, delta)
	case SettingIcons:
		m.cfg.UI.UseIcons = !m.cfg.UI.UseIcons
	}

	config.Save(m.cfg)
	return func() tea.Msg { return MsgConfigChanged{Config: m.cfg} }
}

func rotate(current string, options []string, delta int) string {
	idx := -1
	for i, opt := range options {
		if opt == current {
			idx = i
			break
		}
	}

	if idx == -1 {
		return options[0]
	}

	newIdx := (idx + delta) % len(options)
	if newIdx < 0 {
		newIdx += len(options)
	}
	return options[newIdx]
}

func (m Model) View() string {
	width := m.width
	if width < 20 {
		width = 20
	}
	height := m.height
	if height < 10 {
		height = 10
	}

	// 1. Render Header
	header := shared.StyleHeader.Copy().Width(width).Render("⚙️ Settings")
	headerHeight := lipgloss.Height(header)

	// 2. Render Footer
	footerHelp := "[esc/q/backspace] Back • [↑/↓] Navigate • [←/→/enter] Change"
	space := width - lipgloss.Width(footerHelp) - 2
	footerContent := footerHelp
	if space > 0 {
		footerContent = strings.Repeat(" ", space) + footerHelp
	}
	footer := shared.StyleFooter.Copy().Width(width).Render(footerContent)
	footerHeight := lipgloss.Height(footer)
	bodyHeight := height - headerHeight - footerHeight
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	settings := []string{
		m.renderToggle("Use CPU", "Software Decoding", m.cfg.Player.UseCPU, m.cursor == SettingUseCPU, width),
		m.renderChoice("Hardware Decoding", m.cfg.Player.HWDec, m.cursor == SettingHWDec),
		m.renderChoice("Video Output", m.cfg.Player.VO, m.cursor == SettingVO),
		m.renderChoice("HDR Tone Mapping", m.cfg.Player.ToneMapping, m.cursor == SettingToneMapping),
		m.renderToggle("Subtitles", "Enabled", m.cfg.Player.SubtitlesEnabled, m.cursor == SettingSubtitles, width),
		m.renderChoice("Subtitles Language", defaultAuto(m.cfg.Player.SubtitlesLang), m.cursor == SettingSubLang),
		m.renderChoice("Audio Language", defaultAuto(m.cfg.Player.AudioLang), m.cursor == SettingAudioLang),
		m.renderToggle("UI Icons", "Use icons in menus", m.cfg.UI.UseIcons, m.cursor == SettingIcons, width),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, settings...)

	// Ensure main body has a fixed height by using a container
	bodyContainer := lipgloss.NewStyle().Height(bodyHeight)

	if width > 70 {
		leftWidth := int(float64(width) * 0.55)
		if leftWidth < 38 {
			leftWidth = 38
		}
		rightWidth := width - leftWidth

		left := lipgloss.NewStyle().Width(leftWidth).Render(content)
		right := m.renderTipPanel(rightWidth, bodyHeight)

		main := bodyContainer.Render(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
		return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
	}

	main := bodyContainer.Render(lipgloss.JoinVertical(lipgloss.Left, content, "", m.renderTip(width-2)))
	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

func (m Model) renderTipPanel(width int, height int) string {
	tip := m.renderTip(width - 3)
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#333333")).
		PaddingLeft(2).
		Render(tip)
}

func (m Model) renderTip(width int) string {
	var tip string
	switch m.cursor {
	case SettingUseCPU:
		tip = "Forces software decoding. Use this if your GPU is unstable or causing crashes."
	case SettingHWDec:
		switch m.cfg.Player.HWDec {
		case "vaapi":
			tip = "Recommended for AMD and Intel GPUs on Linux (VA-API)."
		case "nvdec":
			tip = "Recommended for NVIDIA GPUs (NVCUVARR)."
		case "vdpau":
			tip = "Older acceleration for NVIDIA and AMD."
		case "auto-safe":
			tip = "Uses hardware decoding only where it is known to be stable."
		case "no":
			tip = "Disables hardware decoding completely."
		default:
			tip = "Tries to automatically pick the best hardware decoder."
		}
	case SettingVO:
		switch m.cfg.Player.VO {
		case "gpu-next":
			tip = "Modern, high-performance video renderer (Recommended for Wayland/AMD)."
		case "gpu":
			tip = "Stable and performant renderer for most systems."
		case "x11":
			tip = "Standard X11 output. Safe fallback if GPU renders fail."
		case "xv":
			tip = "Old and fast renderer. Legacy fallback for very old hardware."
		default:
			tip = "Automatically selects the best video output available."
		}
	case SettingToneMapping:
		switch m.cfg.Player.ToneMapping {
		case "st2094-10":
			tip = "High-quality dynamic tonemapping. Recommended for standard displays."
		case "mobius":
			tip = "Standard algorithm used by many software players (MPV default)."
		case "spline":
			tip = "S-curve mapping, often provides more contrast."
		case "bt.2446a":
			tip = "Broadcast standard algorithm for HDR to SDR conversion."
		case "clip":
			tip = "Hard clipping. Forces colors into SDR range, may lose detail in highlights."
		default:
			tip = "Use 'auto' if your screen supports HDR. Otherwise, converts HDR to SDR."
		}
	case SettingSubtitles:
		tip = "Automatically enable and display subtitles if available in the file."
	case SettingSubLang:
		tip = "Preferred subtitle language. Use auto to let MPV decide."
	case SettingAudioLang:
		tip = "Preferred audio language. Use auto to let MPV decide."
	case SettingIcons:
		tip = "Show icons (🎬, 📺) next to library names in the sidebar."
	}

	// Ensure the tip text is truncated to fit the width (minus padding for multi-line)
	maxTipLen := width - 2
	if maxTipLen < 10 {
		maxTipLen = 10
	}

	style := lipgloss.NewStyle().
		Foreground(shared.ColorLightGrey).
		Italic(true)

	// We handle multi-line manually to avoid lipgloss wrapping issues in small spaces
	return "💡 Tip:\n" + style.Render(shared.Truncate(tip, maxTipLen*2)) // Allow 2 lines worth of text
}

func (m Model) renderToggle(label string, hint string, value bool, active bool, width int) string {
	prefix := "  "
	style := shared.StyleItemNormal
	valStyle := shared.StyleHighlight

	if active {
		style = shared.StyleItemActive
		prefix = "> "
		// High contrast for active selection: black text on orange background
		valStyle = lipgloss.NewStyle().Foreground(shared.ColorBlack).Bold(true)
	}

	valStr := "OFF"
	if value {
		valStr = "ON"
	}

	labelWidth := 28
	labelText := label
	if hint != "" {
		labelText = fmt.Sprintf("%s (%s)", label, hint)
	}

	line := fmt.Sprintf("%s%-*s %s", prefix, labelWidth, shared.Truncate(labelText, labelWidth), valStyle.Render(valStr))
	return style.Copy().Width(width).MaxHeight(1).Render(line)
}

func (m Model) renderChoice(label string, value string, active bool) string {
	prefix := "  "
	style := shared.StyleItemNormal
	valStyle := shared.StyleHighlight

	if active {
		style = shared.StyleItemActive
		prefix = "> "
		// High contrast for active selection
		valStyle = lipgloss.NewStyle().Foreground(shared.ColorBlack).Bold(true)
	}

	if value == "" {
		value = "auto"
	}

	line := fmt.Sprintf("%s%-28s %s", prefix, shared.Truncate(label, 28), valStyle.Render(value))
	return style.Copy().MaxHeight(1).Render(line)
}

func defaultAuto(value string) string {
	if value == "" {
		return "auto"
	}
	return value
}
