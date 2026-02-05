# 🎥 Plex Minimal TUI

A high-performance, minimalist Plex client for your terminal. Designed to be fast, elegant, and native.

![screenshot](https://github.com/user-attachments/assets/694828be-ce7a-46dd-bbfc-ad7f1af74b84)

## ✨ Features

- **Modern TUI**: Interactive navigation with beautiful layouts and predictive focus.
- **Native Auth**: PIN-based login directly in the TUI—no more manual token hunting.
- **MPV Integration**: High-quality playback with full HDR/SDR support.
- **Blazing Fast**: Local SQLite caching ensures instant library browsing.
- **Portable**: Works anywhere with Go, with native NixOS support.

## 🚀 Quick Start

### Nix users (Recommended)
```bash
nix run github:Waddenn/plex-client
```

### Go users
```bash
go run ./cmd/plex-client
```

_On first run, the TUI will guide you through the PIN authentication process._

## ⚙️ Configuration

Settings are stored at `~/.config/plex-client/config.toml`.

```toml
[player]
quality = "auto"
subtitles_enabled = true

[ui]
show_preview = true
sort_by = "title"
```

## 🛠️ Requirements

- **MPV**: Required for video playback.
- **Go 1.22+**: Required if building from source.

---
[Contribution Guidelines](./CONTRIBUTING.md) | [License](./LICENSE)
