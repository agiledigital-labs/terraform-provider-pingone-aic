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
      devShells = forAllSystems (
        pkgs:
        let
          # Everything needed to build, gate and test the Go code.
          # go.mod requires >= 1.24.
          goToolchain = [
            pkgs.go
            pkgs.gnumake
          ];
        in
        {
          # Lean shell for CI (.github/workflows/test.yml).
          #
          # Deliberately excludes terraform: it is unfree (BUSL-1.1), so it is
          # absent from the public binary cache and would compile from source
          # on every cold runner — minutes of CI time for a dependency no test
          # currently invokes. Add it here when acceptance tests need it, and
          # add a nix store cache at the same time.
          ci = pkgs.mkShell { packages = goToolchain; };

          default = pkgs.mkShell {
            packages = goToolchain ++ [
              pkgs.gopls
              pkgs.terraform
            ];

            shellHook = ''
              # Cheap checks on commit, expensive ones on push. Repo-local
              # (.git/config only) and idempotent.
              if [ -d .git ] && [ "$(git config --get core.hooksPath)" != ".githooks" ]; then
                git config core.hooksPath .githooks
                echo "enabled git hooks -> .githooks (bypass with --no-verify)"
              fi
              echo "terraform-provider-pingone-aic  $(go version | cut -d' ' -f3)  $(terraform version | head -1)"
            '';
          };
        }
      );

      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
