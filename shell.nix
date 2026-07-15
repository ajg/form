{ goPackage ? "go" }:

let
  # Current nixpkgs, mid-2026 - has go (1.26.x) and go_1_25
  pkgsGo2526 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/3d46470bb3030020f7e1361f33514854f5bfa86d.tar.gz") { };

  # nixpkgs snapshot with go_1_24 (1.24.13)
  pkgsGo24 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/80d901ec0377e19ac3f7bb8c035201e2e098cc97.tar.gz") { };

  # nixpkgs snapshot with go_1_23 (1.23.12)
  pkgsGo23 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/a3f3e3f2c983e957af6b07a1db98bafd1f87b7a1.tar.gz") { };

  # nixpkgs snapshot with go_1_21, go_1_22
  pkgsGo2122 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/8c5066250910.tar.gz") { };

  # nixpkgs 23.05 - has go_1_18, go_1_19, go_1_20
  pkgs2305 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/nixos-23.05.tar.gz") { };

  # nixpkgs 22.05 - has go_1_17
  pkgs2205 = import (fetchTarball
    "https://github.com/NixOS/nixpkgs/archive/nixos-22.05.tar.gz") { };

  # Map Go packages to their source
  goFromPkgs = {
    go = pkgsGo2526.go;
    go_1_17 = pkgs2205.go_1_17;
    go_1_18 = pkgs2305.go_1_18;
    go_1_19 = pkgs2305.go_1_19;
    go_1_20 = pkgs2305.go_1_20;
    go_1_21 = pkgsGo2122.go_1_21;
    go_1_22 = pkgsGo2122.go_1_22;
    go_1_23 = pkgsGo23.go_1_23;
    go_1_24 = pkgsGo24.go_1_24;
    go_1_25 = pkgsGo2526.go_1_25;
    go_1_26 = pkgsGo2526.go;
  };

in pkgsGo2526.mkShell {
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
