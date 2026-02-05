package settings

import (
	"fmt"

	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/tui/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	cfg    *config.Config
	cursor int
	width  int
	height int

	// Options for select lists
	voOptions    []string
	hwdecOptions []string
	tmOptions    []string
	subLangs     []string
	audioLangs   []string
}

func NewModel(cfg *config.Config) Model {
	return Model{
		cfg:          cfg,
		voOptions:    []string{"auto", "gpu", "gpu-next", "x11", "xv"},
		hwdecOptions: []string{"auto", "auto-safe", "vaapi", "nvdec", "vdpau", "no"},
		tmOptions:    []string{"auto", "st2094-10", "mobius", "spline", "bt.2446a", "clip"},
		subLangs:     []string{"auto", "eng", "fra", "spa", "deu", "ita", "jpn"},
		audioLangs:   []string{"auto", "eng", "fra", "spa", "deu", "ita", "jpn"},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

type MsgConfigChanged struct {
	Config *config.Config
}

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

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < settingCount-1 {
				m.cursor++
			}
		case "left", "h", "right", "l", "enter", " ":
			return m.handleInput(msg)
		case "esc", "backspace", "q":
			return m, func() tea.Msg { return shared.MsgBack{} }
		}
	}

	return m, nil
}

func (m Model) handleInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	save := false
	key := msg.String()

	switch m.cursor {
	case SettingUseCPU:
		if key == "enter" || key == " " || key == "left" || key == "right" || key == "h" || key == "l" {
			m.cfg.Player.UseCPU = !m.cfg.Player.UseCPU
			save = true
		}
	case SettingHWDec:
		m.cfg.Player.HWDec = rotateOption(m.cfg.Player.HWDec, m.hwdecOptions, key)
		save = true
	case SettingVO:
		m.cfg.Player.VO = rotateOption(m.cfg.Player.VO, m.voOptions, key)
		save = true
	case SettingToneMapping:
		m.cfg.Player.ToneMapping = rotateOption(m.cfg.Player.ToneMapping, m.tmOptions, key)
		save = true
	case SettingSubtitles:
		if key == "enter" || key == " " || key == "left" || key == "right" || key == "h" || key == "l" {
			m.cfg.Player.SubtitlesEnabled = !m.cfg.Player.SubtitlesEnabled
			save = true
		}
	case SettingSubLang:
		m.cfg.Player.SubtitlesLang = rotateOption(defaultAuto(m.cfg.Player.SubtitlesLang), m.subLangs, key)
		save = true
	case SettingAudioLang:
		m.cfg.Player.AudioLang = rotateOption(defaultAuto(m.cfg.Player.AudioLang), m.audioLangs, key)
		save = true
	case SettingIcons:
		if key == "enter" || key == " " || key == "left" || key == "right" || key == "h" || key == "l" {
			m.cfg.UI.UseIcons = !m.cfg.UI.UseIcons
			save = true
		}
	}

	if save {
		config.Save(m.cfg)
		return m, func() tea.Msg { return MsgConfigChanged{Config: m.cfg} }
	}

	return m, nil
}

func rotateOption(current string, options []string, key string) string {
	idx := -1
	for i, o := range options {
		if o == current {
			idx = i
			break
		}
	}

	if idx == -1 {
		idx = 0
	}

	if key == "right" || key == "l" || key == "enter" || key == " " {
		idx = (idx + 1) % len(options)
	} else if key == "left" || key == "h" {
		idx = (idx - 1 + len(options)) % len(options)
	}

	return options[idx]
}

func (m Model) View() string {
	title := shared.StyleTitle.Render("⚙️ Settings")

	settings := []string{
		m.renderToggle("Use CPU (Software Decoding)", m.cfg.Player.UseCPU, m.cursor == SettingUseCPU),
		m.renderChoice("Hardware Decoding", m.cfg.Player.HWDec, m.cursor == SettingHWDec),
		m.renderChoice("Video Output (VO)", m.cfg.Player.VO, m.cursor == SettingVO),
		m.renderChoice("HDR Tone Mapping", m.cfg.Player.ToneMapping, m.cursor == SettingToneMapping),
		m.renderToggle("Subtitles Enabled", m.cfg.Player.SubtitlesEnabled, m.cursor == SettingSubtitles),
		m.renderChoice("Subtitles Language", defaultAuto(m.cfg.Player.SubtitlesLang), m.cursor == SettingSubLang),
		m.renderChoice("Audio Language", defaultAuto(m.cfg.Player.AudioLang), m.cursor == SettingAudioLang),
		m.renderToggle("Use Icons in UI", m.cfg.UI.UseIcons, m.cursor == SettingIcons),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, settings...)

	tip := m.renderTip()

	footer := shared.StyleDim.Render("\n[esc/q] Back • [↑/↓] Navigate • [←/→/enter] Change")

	main := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		content,
		"",
		tip,
		footer,
	)

	return shared.StyleBorder.Render(main)
}

func (m Model) renderTip() string {
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

	return lipgloss.NewStyle().
		Foreground(shared.ColorLightGrey).
		Italic(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(shared.ColorPlexOrange).
		PaddingLeft(2).
		Width(60).
		Render("💡 Tip: " + tip)
}

func (m Model) renderToggle(label string, value bool, active bool) string {
	style := shared.StyleItemNormal
	prefix := "  "
	if active {
		style = shared.StyleItemActive
		prefix = "▶ "
	}

	valStr := "OFF"
	valStyle := shared.StyleDim
	if value {
		valStr = "ON"
		valStyle = shared.StyleHighlight
	}

	return style.Render(fmt.Sprintf("%s%-30s %s", prefix, label, valStyle.Render(valStr)))
}

func (m Model) renderChoice(label string, value string, active bool) string {
	style := shared.StyleItemNormal
	prefix := "  "
	if active {
		style = shared.StyleItemActive
		prefix = "▶ "
	}

	if value == "" {
		value = "auto"
	}

	return style.Render(fmt.Sprintf("%s%-30s %s", prefix, label, shared.StyleHighlight.Render(value)))
}

func defaultAuto(value string) string {
	if value == "" {
		return "auto"
	}
	return value
}
