🎥 **Plex Minimal with MPV**  
A minimalist Plex client that uses MPV and FZF for command-line navigation.

### Features
- Play movies and TV shows directly using MPV
- Command-line selection interface with `fzf`
- Local caching with SQLite for fast access
- TOML configuration with sensible defaults
- Includes a `flake.nix` for seamless NixOS integration

<img src="https://github.com/user-attachments/assets/694828be-ce7a-46dd-bbfc-ad7f1af74b84" alt="screenshot" width="800"/>

---

## Configuration

### Option 1: TOML Config File (Recommended)

Create `~/.config/plex-client/config.toml`:

```toml
[plex]
baseurl = "http://192.168.1.100:32400"
token = "your-plex-token"

[player]
quality = "auto"
subtitles_enabled = true
subtitles_lang = "eng"
audio_lang = "eng"

[ui]
show_preview = true
sort_by = "title"

[sync]
auto_sync = true
```

See [config.toml.example](./config.toml.example) for all available options.

### Option 2: Command-Line Flags

```bash
nix run . -- --baseurl http://localhost:32400 --token YOUR_PLEX_TOKEN
```

---

## File Locations (XDG Standard)

- **Config**: `~/.config/plex-client/config.toml`
- **Cache**: `~/.cache/plex-client/cache.db`

---

## Usage

```bash
# Using config file
nix run .

# Using flags (will save to config)
nix run . -- --baseurl URL --token TOKEN

# Force full cache sync
nix run . -- --force-sync
```
