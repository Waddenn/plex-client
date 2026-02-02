{
  description = "Minimal Plex client with mpv and ModernX";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {inherit system;};

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
      packages.default = pkgs.buildGoModule {
        pname = "plex-client";
        version = "0.1.0";
        src = ./.;

        vendorHash = null;

        nativeBuildInputs = [pkgs.makeWrapper];
        
        postInstall = ''
          wrapProgram $out/bin/plex-client \
            --suffix PATH : ${pkgs.lib.makeBinPath [pkgs.mpv pkgs.fzf]} \
            --set MPV_MODERNX_DIR "${modernx}"
        '';
      };

      devShells.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
          gotools
          go-tools
          mpv
          fzf
        ];
      };
    });
}
