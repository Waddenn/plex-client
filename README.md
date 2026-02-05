# Plex TUI Client

A terminal-based Plex client using MPV for playback.

![screenshot](https://github.com/user-attachments/assets/694828be-ce7a-46dd-bbfc-ad7f1af74b84)

## Features

- **TUI Navigation**: Interface for browsing libraries, seasons, and episodes.
- **PIN-based Authentication**: Login process handled within the terminal.
- **MPV Integration**: Video playback via MPV.
- **Local Cache**: SQLite database for library metadata to reduce network requests.
- **Cross-platform**: Buildable with standard Go tools or via Nix.

## Usage

### Using Nix
```bash
nix run github:Waddenn/plex-client
```

### Using Go
```bash
go run ./cmd/plex-client
```

First-time users will be prompted to authenticate with a PIN.

## Configuration

Configuration is stored in `~/.config/plex-client/config.toml`.

```toml
[player]
quality = "auto"
subtitles_enabled = true

[ui]
show_preview = true
sort_by = "title"
```

## Requirements

- **MPV**: Required for playback.
- **Go 1.22+**: Required if building from source.

---
[Contribution Guidelines](./CONTRIBUTING.md) | [License](./LICENSE)
