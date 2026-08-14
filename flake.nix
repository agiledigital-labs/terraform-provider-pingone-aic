{
  description = "Development environment for terraform-provider-pingone-aic";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems =
        f:
        nixpkgs.lib.genAttrs systems (
          system:
          f (
            import nixpkgs {
              inherit system;
              # Terraform is BUSL-1.1 (unfree) from 1.6 on. Allow exactly that
              # one package rather than blanket-allowing unfree in this shell.
              # Swap terraform -> opentofu below to avoid this entirely.
              config.allowUnfreePredicate = p: builtins.elem (nixpkgs.lib.getName p) [ "terraform" ];
            }
          )
        );
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            # go.mod requires >= 1.24.
            pkgs.go
            pkgs.gopls
            pkgs.terraform
            pkgs.gnumake
          ];

          shellHook = ''
            echo "terraform-provider-pingone-aic  $(go version | cut -d' ' -f3)  $(terraform version | head -1)"
          '';
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
