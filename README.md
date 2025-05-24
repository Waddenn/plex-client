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
  --baseurl http://localhost:32400 \
  --token YOUR_PLEX_TOKEN

![2025-05-24_23:53:54](https://github.com/user-attachments/assets/29ea301f-5c24-49ee-b871-9a5fbebad97c)
