package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Plex   PlexConfig   `toml:"plex"`
	Player PlayerConfig `toml:"player"`
	UI     UIConfig     `toml:"ui"`
	Sync   SyncConfig   `toml:"sync"`
}

type PlexConfig struct {
	BaseURL          string `toml:"baseurl"`
	Token            string `toml:"token"`
	ClientIdentifier string `toml:"client_identifier"`
}

type PlayerConfig struct {
	Quality          string   `toml:"quality"`
	MPVArgs          []string `toml:"mpv_args"`
	UseCPU           bool     `toml:"use_cpu"`
	HWDec            string   `toml:"hwdec"` // e.g., vaapi, vulkan, auto
	VO               string   `toml:"vo"`    // e.g., gpu-next, wayland
	ToneMapping      string   `toml:"tone-mapping"`
	SubtitlesEnabled bool     `toml:"subtitles_enabled"`
	SubtitlesLang    string   `toml:"subtitles_lang"`
	AudioLang        string   `toml:"audio_lang"`
}

type UIConfig struct {
	ShowPreview          bool   `toml:"show_preview"`
	SortBy               string `toml:"sort_by"`
	UseIcons             bool   `toml:"use_icons"`
	StatusIndicatorStyle string `toml:"status_indicator_style"`
}

type SyncConfig struct {
	AutoSync                  bool `toml:"auto_sync"`
	ForceSyncOnStart          bool `toml:"force_sync_on_start"`
	BackgroundSyncIntervalMin int  `toml:"background_sync_interval_minutes"`
}

// Defaults returns a config with sensible defaults
func Defaults() *Config {
	return &Config{
		Plex: PlexConfig{
			BaseURL: "",
			Token:   "",
		},
		Player: PlayerConfig{
			Quality:          "auto",
			MPVArgs:          []string{},
			SubtitlesEnabled: true,
			SubtitlesLang:    "eng",
			AudioLang:        "eng",
		},
		UI: UIConfig{
			ShowPreview:          true,
			SortBy:               "title",
			UseIcons:             true,
			StatusIndicatorStyle: "badges",
		},
		Sync: SyncConfig{
			AutoSync:                  true,
			ForceSyncOnStart:          false,
			BackgroundSyncIntervalMin: 0,
		},
	}
}

func ConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "plex-client")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func CacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cacheDir, "plex-client")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// Load loads config from TOML, merging with defaults
func Load() (*Config, error) {
	cfg := Defaults() // Start with defaults

	dir, err := ConfigDir()
	if err != nil {
		return cfg, err
	}

	path := filepath.Join(dir, "config.toml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil // Return defaults if no file
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	if cfg.Plex.ClientIdentifier == "" {
		cfg.Plex.ClientIdentifier = generateClientID()
		_ = Save(cfg) // Best effort save
	}

	return cfg, nil
}

func generateClientID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback if random fails
		return "plex-client-go-cli-" + time.Now().Format("20060102150405")
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Save saves config to TOML
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "config.toml")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}
