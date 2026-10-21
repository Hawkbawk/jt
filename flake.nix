{
  description = "jt - a JIRA TUI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        go = pkgs.go_latest or pkgs.go;
      in
      {
        devShells.default = pkgs.mkShell {
          packages = [
            go
            pkgs.gopls
            pkgs.delve
            pkgs.golangci-lint
            pkgs.gotools
            pkgs.gofumpt
            pkgs.go-tools
            pkgs.gotests
            pkgs.gomodifytags
            pkgs.impl
          ];

          GOFLAGS = "-timeout=10s";
        };
      });
}
