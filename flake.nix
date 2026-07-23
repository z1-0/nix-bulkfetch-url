{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        src = pkgs.lib.cleanSourceWith {
          src = ./.;
          filter =
            path: type:
            let
              base = baseNameOf path;
            in
            (type == "directory")
            || pkgs.lib.hasSuffix ".go" base
            || base == "go.mod"
            || base == "go.sum"
            || base == "LICENSE";
        };

        nix-bulkfetch-url = pkgs.buildGoModule {
          pname = "nix-bulkfetch-url";
          version = "0.1.0";
          inherit src;
          vendorHash = null;
          doCheck = false;
        };
      in
      {
        packages = {
          default = nix-bulkfetch-url;
          nix-bulkfetch-url = nix-bulkfetch-url;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
          ];
        };
      }
    );
}
