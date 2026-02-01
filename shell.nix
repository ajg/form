{ goPackage ? "go" }:

let
  # Current nixpkgs (has go_1_21, go_1_22)
  pkgsCurrent = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/8c5066250910.tar.gz") { };

  # nixpkgs 23.05 - has go_1_18, go_1_19, go_1_20
  pkgs2305 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/nixos-23.05.tar.gz") { };

  # nixpkgs 22.05 - has go_1_17
  pkgs2205 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/nixos-22.05.tar.gz") { };

  # Map Go packages to their source
  goFromPkgs = {
    go = pkgsCurrent.go;
    go_1_17 = pkgs2205.go_1_17;
    go_1_18 = pkgs2305.go_1_18;
    go_1_19 = pkgs2305.go_1_19;
    go_1_20 = pkgs2305.go_1_20;
    go_1_21 = pkgsCurrent.go_1_21;
    go_1_22 = pkgsCurrent.go_1_22;
  };

in pkgsCurrent.mkShell {
  buildInputs = [
    goFromPkgs.${goPackage}
  ];

  shellHook = ''
    echo "Go development environment ready"
    echo "Go version: $(go version)"
    echo ""
    echo "Available commands:"
    echo "  go build ./...   - Build the package"
    echo "  go test ./...    - Run tests"
    echo "  go test -v ./... - Run tests with verbose output"
    echo "  go test -cover ./... - Run tests with coverage"
  '';
}
