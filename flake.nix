{
  description = "CloudExec VPS provisioning helper";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.11";
    utils.url = "github:numtide/flake-utils";
    crytic.url = "github:crytic/crytic.nix";
  };

  outputs = inputs: with inputs;
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; config.allowUnfree = true; };
        pyCommon = {
          format = "pyproject";
          nativeBuildInputs = with pkgs.python310Packages; [ pythonRelaxDepsHook ];
          pythonRelaxDeps = true;
          doCheck = false;
        };
      in
      rec {

        # Provide some binary packages for selected system types.
        packages = rec {
          default = cloudexec;
          cloudexec = let
            version = let
              result = builtins.match "([^\n]*).*" (builtins.readFile ./VERSION);
            in if result != null then builtins.head result else "unknown";
            gitCommit = let
              result = builtins.match ".*commit=([^\n]*).*" (builtins.readFile ./VERSION);
            in if result != null then builtins.head result else "unknown";
            gitDate = let
              result = builtins.match ".*date=([^\n]*).*" (builtins.readFile ./VERSION);
            in if result != null then builtins.head result else "unknown";
          in pkgs.buildGoModule {
            pname = "cloudexec";
            version = "${version}";
            src = ./.;
            vendorHash = "sha256-xiiMcjo+hRllttjYXB3F2Ms2gX43r7/qgwxr4THNhsk=";
            nativeBuildInputs = [
              pkgs.go
            ];
            ldflags = [
              "-X main.Version=${version}"
              "-X main.Commit=${gitCommit}"
              "-X main.Date=${gitDate}"
            ];
          };
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.cloudexec}/bin/cloudexec";
          };
        };

        devShells = {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              # misc tools
              shellcheck
              just
              trunk-io
              # go development
              go
              gotools
              go-tools
              gopls
              go-outline
              gopkgs
              gocode-gomod
              godef
              golint
              # deployment tools
              packer
              doctl
              curl
              # manual testing
              crytic.packages.${system}.crytic-compile
              crytic.packages.${system}.medusa
              crytic.packages.${system}.echidna
              (crytic.lib.${system}.mkVscode {
                extensions = with pkgs.vscode-extensions; [
                  vscodevim.vim
                  golang.go
                ];
              })
            ];
          };
        };

      }
   );
}
