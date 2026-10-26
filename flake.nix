{
  description = "jt - a JIRA TUI";

  nixConfig = {
    extra-substituters = [ "https://hawkbawk-jt.cachix.org" ];
    extra-trusted-public-keys = [ "hawkbawk-jt.cachix.org-1:aGp3dIggp5bzdE2yp/mS5wqKSGVme3O7ApV3RC2kchE=" ];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        go = pkgs.go_latest or pkgs.go;
        buildGoModule = pkgs.buildGoModule.override { inherit go; };
      in
      {
        packages.default = buildGoModule {
          pname = "jt";
          version = self.shortRev or self.dirtyShortRev or "dev";
          src = self;
          vendorHash = "sha256-twCE26OD77J8EucTCwO6iAlqebjQ8w15+P0WntAb+Ac=";
          ldflags = [ "-s" "-w" ];
          doCheck = false;
          meta.mainProgram = "jt";
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

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
