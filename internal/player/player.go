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

func Play(title, url string, ratingKey string, cfg *config.Config, pClient *plex.Client, extraArgs ...string) error {
	fullURL := fmt.Sprintf("%s?X-Plex-Token=%s", url, cfg.Plex.Token)
	
	// Create a temporary IPC socket path
	ipcSocket := filepath.Join(os.TempDir(), fmt.Sprintf("plex-mpv-%d.sock", time.Now().UnixNano()))

	args := []string{
		"--force-window=yes",
		"--fullscreen",
		"--msg-level=all=warn,vo=error",
		fmt.Sprintf("--title=%s", title),
		fmt.Sprintf("--input-ipc-server=%s", ipcSocket),
	}

	// Setup ModernX environment if available
	if modernXDir := os.Getenv("MPV_MODERNX_DIR"); modernXDir != "" {
		tmpDir, err := os.MkdirTemp("", "plex-client-mpv-*")
		if err == nil {
			defer os.RemoveAll(tmpDir)
			
			// Setup directories
			os.Mkdir(filepath.Join(tmpDir, "scripts"), 0755)
			os.Mkdir(filepath.Join(tmpDir, "fonts"), 0755)
			
			// Copy/Symlink ModernX files
			os.Symlink(filepath.Join(modernXDir, "scripts", "modernx.lua"), filepath.Join(tmpDir, "scripts", "modernx.lua"))
			os.Symlink(filepath.Join(modernXDir, "fonts", "Material-Design-Iconic-Font.ttf"), filepath.Join(tmpDir, "fonts", "Material-Design-Iconic-Font.ttf"))
			
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
			os.WriteFile(confPath, []byte(confContent), 0644)
			
			args = append(args, fmt.Sprintf("--config-dir=%s", tmpDir))
		}
	}
	
	// Add override args from env
	if override := os.Getenv("MPV_CONFIG_OVERRIDE"); override != "" {
		args = append(args, strings.Fields(override)...)
	}

	args = append(args, extraArgs...)
	args = append(args, fullURL)

	cmd := exec.Command("mpv", args...)
	
	// Start monitoring routine
	go monitorProgress(ipcSocket, ratingKey, pClient)

	return cmd.Run()
}

func monitorProgress(socketPath string, ratingKey string, p *plex.Client) {
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
	} else if duration > 0 {
		// Report point where we stopped
		p.ReportProgress(ratingKey, int64(currentTime*1000), int64(duration*1000), "stopped")
	}
}

func sendIPC(conn net.Conn, cmd []interface{}) {
	data, _ := json.Marshal(map[string]interface{}{"command": cmd})
	conn.Write(append(data, '\n'))
}
