#flake.nix
{
  description = "Minimal Plex client with mpv";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";

  outputs = {
    self,
    nixpkgs,
  }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs {inherit system;};
    pythonEnv = pkgs.python3.withPackages (ps: with ps; [plexapi requests]);
  in {
    packages.${system}.plex-minimal = pkgs.writeShellApplication {
      name = "plex-minimal";
      runtimeInputs = [pythonEnv pkgs.mpv pkgs.fzf];

      text = ''
        export BUILD_CACHE=${./build_cache.py}
        exec python3 ${./plex-player.py} "$@"
      '';
    };

    defaultPackage.${system} = self.packages.${system}.plex-minimal;
  };
}
