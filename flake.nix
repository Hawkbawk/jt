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
        version = "0.1.0";
      in
      {
        packages.default = buildGoModule {
          pname = "jt";
          inherit version;
          src = self;
          vendorHash = "sha256-9sKPgdsySgP+GZwUQEpS+NRNd2gqaUo/v6UeiXTJaZc=";
          ldflags = [ "-s" "-w" "-X main.version=${version}" ];
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
