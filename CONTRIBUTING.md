# Contributing to Plex Client

Thank you for your interest in contributing! We follow a **Trunk-Based Development** workflow to ensure stability and rapid iteration.

## core Principles

1.  **Main is Stable**: The `main` branch should always be buildable and passing tests.
2.  **Pull Requests**: All changes must be submitted via a Pull Request (PR). Direct pushes to `main` are discouraged.
3.  **CI Checks**: Your PR must pass all automated checks (Build & Test) before it can be merged.

## Getting Started

Required tools:
- [Nix](https://nixos.org/download.html) (with flakes enabled)

### Local Development

1.  **Clone the repository**:
    ```bash
    git clone https://github.com/yourusername/plex-client.git
    cd plex-client
    ```

2.  **Enter the development environment**:
    ```bash
    nix develop
    ```
    This provides `go`, `gopls`, `mpv`, etc.

3.  **Run the application**:
    ```bash
    go run .
    ```

### Running Tests

Run all unit tests:
```bash
go test -v ./...
```

To run the full Nix build (which also verifies dependencies):
```bash
nix build
```

## Submitting a Change

1.  Create a new branch for your feature or fix:
    ```bash
    git checkout -b feature/my-new-feature
    ```
2.  Make your changes.
3.  Verify locally with `go test ./...` and `nix build`.
4.  Push your branch and open a Pull Request against `main`.

## Code Style

- We use standard Go formatting (`go fmt`).
- Ensure your code is readable and idiomatic.
