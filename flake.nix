{
  description = "Val Town, as a fuse file system";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    treefmt-nix,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {inherit system;};
        python = pkgs.python3.withPackages (
          ps:
            with ps; [
              requests
            ]
        );

        # Configure treefmt
        treefmtEval = treefmt-nix.lib.evalModule pkgs {
          projectRootFile = "flake.nix";
          programs = {
            gofmt.enable = true;
            goimports.enable = true;
            alejandra.enable = true;
            deno.enable = true;
          };
          settings.formatter = {
            deno = {includes = ["*.ts" "*.tsx" "*.js" "*.jsx" "*.json"];};
          };
        };
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
                cobra-cli
                openapi-generator-cli
                inotify-tools
              ])
              ++ [
                # Add treefmt to the development shell
                treefmtEval.config.build.wrapper
              ];
          };
        };

        # Add formatter
        formatter = treefmtEval.config.build.wrapper;

        # Add formatting check
        checks = {
          formatting = treefmtEval.config.build.check self;
        };

        packages = rec {
          default = valfs;
          valfs = pkgs.callPackage ./package.nix {};
        };
      }
    );
}
