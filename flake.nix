{
  description = "Minimal Plex client with mpv and ModernX";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";

  outputs = {
    self,
    nixpkgs,
  }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs {inherit system;};

    pythonEnv = pkgs.python3.withPackages (ps: with ps; [plexapi requests]);

    modernx = pkgs.stdenv.mkDerivation {
      name = "modernx-osc";
      src = pkgs.fetchFromGitHub {
        owner = "cyl0";
        repo = "ModernX";
        rev = "3f2ed6b";
        sha256 = "sha256-q7DwyfmOIM7K1L7vvCpq1EM0RVpt9E/drhAa9rLYb1k=";
      };
      installPhase = ''
        mkdir -p $out/scripts
        mkdir -p $out/fonts
        cp modernx.lua $out/scripts/
        cp Material-Design-Iconic-Font.ttf $out/fonts/
      '';
    };
  in {
    packages.${system}.plex-minimal = pkgs.writeShellApplication {
      name = "plex-minimal";
      runtimeInputs = [pythonEnv pkgs.mpv pkgs.fzf];

      text = ''
        export BUILD_CACHE=${./build_cache.py}

        TMPDIR=$(mktemp -d)
        export MPV_MODERNX_DIR="$TMPDIR"

        mkdir -p "$MPV_MODERNX_DIR/scripts" "$MPV_MODERNX_DIR/fonts"
        cp ${modernx}/scripts/modernx.lua "$MPV_MODERNX_DIR/scripts/"
        cp ${modernx}/fonts/Material-Design-Iconic-Font.ttf "$MPV_MODERNX_DIR/fonts/"

        echo "osc=no" >> "$MPV_MODERNX_DIR/mpv.conf"
        echo "border=no" >> "$MPV_MODERNX_DIR/mpv.conf"

        export MPV_CONFIG_OVERRIDE="--config-dir=$MPV_MODERNX_DIR"
        exec python3 ${./plex-player.py} "$@"
      '';
    };

    defaultPackage.${system} = self.packages.${system}.plex-minimal;
  };
}
