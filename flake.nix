{
  description = "Prometheus textfile collector for pending NixOS reboots";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
  };

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forAllSystems =
        f:
        nixpkgs.lib.genAttrs systems (
          system:
          f {
            pkgs = import nixpkgs { inherit system; };
          }
        );
    in
    {
      packages = forAllSystems (
        { pkgs }:
        {
          default = pkgs.buildGoModule {
            pname = "nixos-reboot-required-collector";
            version = "0.1.0";

            src = ./.;

            vendorHash = null;
            subPackages = [ "cmd/nixos-reboot-required-collector" ];

            meta = {
              description = "Prometheus textfile collector for pending NixOS reboots";
              license = pkgs.lib.licenses.mit;
              mainProgram = "nixos-reboot-required-collector";
            };
          };
        }
      );

      devShells = forAllSystems (
        { pkgs }:
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gofumpt
            ];
          };
        }
      );
    };
}
