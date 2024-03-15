{
  description = "CloudExec VPS provisioning helper";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/4ecab3273592f27479a583fb6d975d4aba3486fe"; # v23.05
    utils.url = "github:numtide/flake-utils/04c1b180862888302ddfb2e3ad9eaa63afc60cf8"; # v1.0.0
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
              pkgs.git
              pkgs.go_1_20
            ];
            ldflags = [
              "-X main.Version=${version}"
              "-X main.Commit=${gitCommit}"
              "-X main.Date=${gitDate}"
            ];
          };

          vscode = pkgs.vscode-with-extensions.override {
            vscode = pkgs.vscodium;
            vscodeExtensions = with pkgs.vscode-extensions; [
              golang.go
              jnoortheen.nix-ide
              mads-hartmann.bash-ide-vscode
              mikestead.dotenv
              naumovs.color-highlight
              oderwat.indent-rainbow
              vscodevim.vim
              yzhang.markdown-all-in-one
            ];
          };

          solc-select = pkgs.python310Packages.buildPythonPackage (pyCommon // {
            pname = "solc-select";
            version = "1.0.4";
            src = builtins.fetchGit {
              url = "git+ssh://git@github.com/crytic/solc-select";
              rev = "8072a3394bdc960c0f652fb72e928a7eae3631da";
            };
            propagatedBuildInputs = with pkgs.python310Packages; [
              packaging
              setuptools
              pycryptodome
            ];
          });

          crytic-compile = pkgs.python310Packages.buildPythonPackage (pyCommon // rec {
            pname = "crytic-compile";
            version = "0.3.5";
            src = builtins.fetchGit {
              url = "git+ssh://git@github.com/crytic/crytic-compile";
              rev = "3a4b0de72ad418b60b9ef8c38d7de31ed39e3898";
            };
            propagatedBuildInputs = with pkgs.python310Packages; [
              cbor2
              packages.solc-select
              pycryptodome
              setuptools
              toml
            ];
          });

          medusa = pkgs.buildGoModule {
            pname = "medusa";
            version = "0.1.2"; # from cmd/root.go
            src = builtins.fetchGit {
              url = "git+ssh://git@github.com/trailofbits/medusa";
              rev = "ac99e78ee38df86a8afefb21f105be9e4eae46ee";
            };
            vendorSha256 = "sha256-k5DtmpNi1ynSWgJ6b9EIlqCM6OlCkQf3Cf/daP+I7mY=";
            nativeBuildInputs = [
              packages.crytic-compile
              pkgs.solc
              pkgs.nodejs
            ];
            doCheck = false; # tests require `npm install` which can't run in hermetic build env
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
              bashInteractive
              shellcheck
              packages.vscode
              just
              trunk-io
              # go development
              go_1_20
              gotools
              go-tools
              gopls
              go-outline
              gocode
              gopkgs
              gocode-gomod
              godef
              golint
              # deployment tools
              packer
              doctl
              curl
              # manual testing
              packages.medusa
              packages.crytic-compile
            ];
          };
        };

      }
   );
}
