{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
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
