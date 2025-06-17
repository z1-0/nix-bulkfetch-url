{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          nix-bulkfetch-url = pkgs.buildGoModule {
            pname = "nix-bulkfetch-url";
            version = "0.1.0";
            src = ./.;
            vendorHash = null;
          };
          default = self.packages.${system}.nix-bulkfetch-url;
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
