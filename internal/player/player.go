package player

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Waddenn/plex-client/internal/config"
)

func Play(title, url string, cfg *config.Config, extraArgs ...string) error {
	fullURL := fmt.Sprintf("%s?X-Plex-Token=%s", url, cfg.Plex.Token)
	
	args := []string{
		"--force-window=yes",
		"--fullscreen",
		"--msg-level=all=warn,vo=error",
		fmt.Sprintf("--title=%s", title),
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
	// We do not set Stdout/Stderr to os.Stdout/Stderr to avoid polluting the FZF interface.
	// By default, if they are nil, they go to /dev/null.
	
	return cmd.Run()
}
