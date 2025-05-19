🎥 **Plex Minimal with MPV**  
A minimalist Plex client that uses MPV and FZF for command-line navigation.

### Features
- Play movies and TV shows directly using MPV
- Command-line selection interface with `fzf`
- Local caching with SQLite for fast access
- Includes a `flake.nix` for seamless NixOS integration

### Usage
```bash
nix run github:Waddenn/plex-client#plex-minimal -- \
  --baseurl http://192.168.1.2:32400 \
  --token YOUR_PLEX_TOKEN
