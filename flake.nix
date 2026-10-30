{
  description = "lazypass - lazygit-style password manager";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAll (pkgs: rec {
        lazypass = pkgs.callPackage ./nix/package.nix { };
        default = lazypass;
      });

      apps = forAll (pkgs: rec {
        lazypass = {
          type = "app";
          program = "${self.packages.${pkgs.system}.lazypass}/bin/lazypass";
        };
        default = lazypass;
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools go-tools delve wl-clipboard ];
        };
      });

      # NixOS: imports = [ lazypass.nixosModules.default ]; programs.lazypass.enable = true;
      nixosModules.default = { config, lib, pkgs, ... }: {
        options.programs.lazypass.enable = lib.mkEnableOption "lazypass password manager";
        config = lib.mkIf config.programs.lazypass.enable {
          environment.systemPackages = [ self.packages.${pkgs.system}.lazypass ];
        };
      };

      overlays.default = final: _prev: {
        lazypass = final.callPackage ./nix/package.nix { };
      };
    };
}
