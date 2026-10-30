{
  description = "Minimal Discord CLI for AI agents";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-darwin" "x86_64-darwin" "x86_64-linux" "aarch64-linux" ];
      goEnv.GOEXPERIMENT = "jsonv2"; # encoding/json/v2 is still an experiment in Go 1.26
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAll (pkgs: {
        default = pkgs.buildGoModule {
          pname = "discord";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "cmd/discord" ];
          env = goEnv;
          vendorHash = "sha256-HXY0wM0sUn3f1xCZRLYEZg9J5v+rKizPrXTfu7JQvEQ=";
        };
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools ];
          env = goEnv;
        };
      });
    };
}
