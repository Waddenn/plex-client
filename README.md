🎥 **Plex Minimal with MPV**  
A minimalist Plex client that uses MPV and FZF for command-line navigation.

### Features
- Play movies and TV shows directly using MPV
- Command-line selection interface with `fzf`
- Local caching with SQLite for fast access
- Includes a `flake.nix` for seamless NixOS integration

<img src="https://github.com/user-attachments/assets/62992ce1-83fe-4c90-a969-afe74e219df8" alt="screenshot" width="700"/>

### Usage
```bash
nix run github:Waddenn/plex-client -- \
  --baseurl http://localhost:32400 \
  --token YOUR_PLEX_TOKEN
