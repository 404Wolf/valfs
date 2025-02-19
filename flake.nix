{
  description = "Cookiecutter project template collection";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {inherit system;};
      python = pkgs.python3.withPackages (ps:
        with ps; [
          requests
        ]);
    in {
      devShells = {
        default = pkgs.mkShell {
          hardeningDisable = ["all"];
          packages =
            [python]
            ++ (with pkgs; [
              go
              gopls
              delve
              deno
              cobra-cli
              openapi-generator-cli
              inotify-tools
            ]);
        };
      };
      packages = rec {
        default = valfs;
        valfs = pkgs.callPackage ./package.nix {};
      };
    });
}
