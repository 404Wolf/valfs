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
    in {
      devShells = {
        default = pkgs.mkShell {
          hardeningDisable = ["all"];
          packages = with pkgs; [
            go
            gopls
            delve
            cobra-cli
            openapi-generator-cli
          ];
        };
      };
      packages = rec {
        default = valfs;
        valfs = pkgs.callPackage ./package.nix {};
      };
    });
}
