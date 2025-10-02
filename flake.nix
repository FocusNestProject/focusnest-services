{
  description = "Go dev ";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
  }: let
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages.${system};
  in {
    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        go
        delve
      ];

      shellHook = ''
        # Use parent directory as GOPATH
        export GOPATH="$(pwd)/.."
        export PATH="$GOPATH/bin:$PATH"

        echo "Go dev environment ready"
        go version
        dlv version
      '';
    };
  };
}
