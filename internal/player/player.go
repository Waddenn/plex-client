package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Waddenn/plex-client/internal/config"
	"github.com/Waddenn/plex-client/internal/plex"
)

func Play(title, url string, ratingKey string, startTimeMs int64, cfg *config.Config, pClient *plex.Client, extraArgs ...string) (bool, error) {
	fullURL := fmt.Sprintf("%s?X-Plex-Token=%s", url, cfg.Plex.Token)

	// Create a temporary IPC socket path
	ipcSocket := filepath.Join(os.TempDir(), fmt.Sprintf("plex-mpv-%d.sock", time.Now().UnixNano()))

	// Detect Wayland for better VO defaults
	isWayland := os.Getenv("WAYLAND_DISPLAY") != ""

	args := baseArgs(title, ipcSocket)
	args = append(args, buildSubtitleArgs(cfg)...)
	args = append(args, buildLanguageArgs(cfg)...)

	// Stability & CPU vs GPU logic
	if cfg.Player.UseCPU {
		args = append(args, buildCPUArgs()...)
	} else {
		args = append(args, buildGPUArgs(cfg, isWayland)...)
	}

	// Add start time if > 0
	if startTimeMs > 0 {
		// Convert ms to seconds (float)
		seconds := float64(startTimeMs) / 1000.0
		args = append(args, fmt.Sprintf("--start=%.2f", seconds))
	}

	// Setup ModernX environment if available
	if cfgPath, ok := setupModernX(); ok {
		defer os.RemoveAll(cfgPath)
		args = append(args, fmt.Sprintf("--config-dir=%s", cfgPath))
	}

	// Add args from config
	args = append(args, cfg.Player.MPVArgs...)

	// Add override args from env
	if override := os.Getenv("MPV_CONFIG_OVERRIDE"); override != "" {
		args = append(args, strings.Fields(override)...)
	}

	args = append(args, extraArgs...)
	args = append(args, fullURL)

	cmd := exec.Command("mpv", args...)

	// Start monitoring routine
	doneCh := make(chan bool)
	go monitorProgress(ipcSocket, ratingKey, pClient, doneCh)

	err := cmd.Run()

	// Wait for monitor to decide if we finished
	completed := <-doneCh

	if err != nil {
		return false, fmt.Errorf("mpv failed: %w", err)
	}
	return completed, nil
}

func baseArgs(title, ipcSocket string) []string {
	return []string{
		"--force-window=yes",
		"--fullscreen",
		"--target-colorspace-hint", // Essential for fixing faded colors
		"--panscan=1.0",            // Scaling: Fill screen by cropping black bars
		fmt.Sprintf("--title=%s", title),
		fmt.Sprintf("--input-ipc-server=%s", ipcSocket),
	}
}

func buildCPUArgs() []string {
	return []string{
		"--profile=fast",               // Global performance profile
		"--vo=xv,x11",                  // Stable legacy VOs
		"--hwdec=no",                   // Force software
		"--vd-lavc-threads=0",          // Maximize CPU usage
		"--vd-lavc-fast=yes",           // Favor speed over quality
		"--vd-lavc-skiploopfilter=all", // Big CPU saving
		"--sws-scaler=fast-bilinear",   // Lightweight scaling
		"--video-sync=audio",           // Prevent CPU spikes from sync
	}
}

func buildGPUArgs(cfg *config.Config, isWayland bool) []string {
	args := []string{}

	vo := "gpu-next" // Modern default
	if cfg.Player.VO != "" {
		vo = cfg.Player.VO
	} else if isWayland {
		vo = "gpu-next,wayland"
	}
	args = append(args, fmt.Sprintf("--vo=%s", vo))

	hwdec := "auto-safe"
	if cfg.Player.HWDec != "" {
		hwdec = cfg.Player.HWDec
	} else if isWayland {
		hwdec = "vaapi" // Best for AMD on Wayland
	}
	args = append(args, fmt.Sprintf("--hwdec=%s", hwdec))

	if isWayland {
		args = append(args, "--gpu-context=wayland")
	}

	tm := "st2094-10" // High quality default for GPU-Next
	if cfg.Player.ToneMapping != "" {
		tm = cfg.Player.ToneMapping
	}
	args = append(args,
		fmt.Sprintf("--tone-mapping=%s", tm),
		"--hdr-compute-peak=yes",
		"--gamut-mapping-mode=clip",
	)
	if tm != "auto" {
		args = append(args, "--target-trc=gamma2.2") // Fixes colors for SDR monitors
	}

	return args
}

func buildSubtitleArgs(cfg *config.Config) []string {
	if cfg.Player.SubtitlesEnabled {
		return []string{"--sid=auto"}
	}
	return []string{"--sid=no"}
}

func buildLanguageArgs(cfg *config.Config) []string {
	args := []string{}
	if cfg.Player.SubtitlesLang != "" {
		args = append(args, fmt.Sprintf("--slang=%s", cfg.Player.SubtitlesLang))
	}
	if cfg.Player.AudioLang != "" {
		args = append(args, fmt.Sprintf("--alang=%s", cfg.Player.AudioLang))
	}
	return args
}

func setupModernX() (string, bool) {
	modernXDir := os.Getenv("MPV_MODERNX_DIR")
	if modernXDir == "" {
		return "", false
	}

	tmpDir, err := os.MkdirTemp("", "plex-client-mpv-*")
	if err != nil {
		return "", false
	}

	// Setup directories
	if err := os.Mkdir(filepath.Join(tmpDir, "scripts"), 0755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", false
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "fonts"), 0755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", false
	}

	// Copy/Symlink ModernX files
	if err := os.Symlink(filepath.Join(modernXDir, "scripts", "modernx.lua"), filepath.Join(tmpDir, "scripts", "modernx.lua")); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", false
	}
	if err := os.Symlink(filepath.Join(modernXDir, "fonts", "Material-Design-Iconic-Font.ttf"), filepath.Join(tmpDir, "fonts", "Material-Design-Iconic-Font.ttf")); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", false
	}

	// Write mpv.conf
	// We try to include the user's original mpv.conf to respect their settings
	confContent := ""
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		userMpvConf := filepath.Join(userConfigDir, "mpv", "mpv.conf")
		if _, err := os.Stat(userMpvConf); err == nil {
			confContent += fmt.Sprintf("include=\"%s\"\n", userMpvConf)
		}
	}
	// Enforce settings required for ModernX
	confContent += "osc=no\nborder=no\n"

	confPath := filepath.Join(tmpDir, "mpv.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", false
	}

	return tmpDir, true
}

func monitorProgress(socketPath string, ratingKey string, p *plex.Client, doneCh chan<- bool) {
	// Default to false
	finalStatus := false
	defer func() { doneCh <- finalStatus }()

	// Wait for socket to be created
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return
	}
	defer conn.Close()

	// Observe properties
	sendIPC(conn, []interface{}{"observe_property", 1, "time-pos"})
	sendIPC(conn, []interface{}{"observe_property", 2, "duration"})
	sendIPC(conn, []interface{}{"observe_property", 3, "pause"})

	scanner := bufio.NewScanner(conn)
	var duration float64
	var currentTime float64
	var paused bool
	lastReport := time.Now()

	for scanner.Scan() {
		line := scanner.Bytes()
		var event struct {
			Event string      `json:"event"`
			Name  string      `json:"name"`
			Data  interface{} `json:"data"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if event.Event == "property-change" {
			switch event.Name {
			case "duration":
				if v, ok := event.Data.(float64); ok {
					duration = v
				}
			case "time-pos":
				if v, ok := event.Data.(float64); ok {
					currentTime = v
				}
			case "pause":
				if v, ok := event.Data.(bool); ok {
					paused = v
					state := "playing"
					if paused {
						state = "paused"
					}
					// Report immediate state change
					go p.ReportProgress(ratingKey, int64(currentTime*1000), int64(duration*1000), state)
					lastReport = time.Now()
				}
			}

			// Report every 10 seconds if playing
			if !paused && duration > 0 && currentTime > 0 && time.Since(lastReport) > 10*time.Second {
				go p.ReportProgress(ratingKey, int64(currentTime*1000), int64(duration*1000), "playing")
				lastReport = time.Now()
			}
		}
	}

	// When loop ends (mpv closed), check if we watched enough to scrobble
	if duration > 0 && currentTime > 0 && (currentTime/duration) > 0.90 {
		p.Scrobble(ratingKey)
		finalStatus = true
	} else if duration > 0 {
		// Report point where we stopped
		p.ReportProgress(ratingKey, int64(currentTime*1000), int64(duration*1000), "stopped")
	}
}

func sendIPC(conn net.Conn, cmd []interface{}) {
	data, _ := json.Marshal(map[string]interface{}{"command": cmd})
	conn.Write(append(data, '\n'))
}
