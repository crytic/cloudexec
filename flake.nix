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
      in
      rec {

        # Provide some binary packages for selected system types.
        packages = rec {

          default = cloudexec;

          cloudexec =
            let
              gitInfo = pkgs.runCommand "get-git-info" {} ''
                echo "commit=$(git rev-parse HEAD)" > $out
                echo "date=$(git log -1 --format=%cd --date=format:'%Y-%m-%d %H:%M:%S')" >> $out
              '';
              
              gitCommit = builtins.head (builtins.match "commit=(.*)" (builtins.readFile gitInfo));
              gitDate = builtins.head (builtins.match "date=(.*)" (builtins.readFile gitInfo));
            in pkgs.buildGoModule {
            pname = "cloudexec";
            version = "0.0.1"; # TBD
            src = ./.;
            vendorSha256 = "sha256-xiiMcjo+hRllttjYXB3F2Ms2gX43r7/qgwxr4THNhsk=";
            nativeBuildInputs = [
              pkgs.go_1_20
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
            ];
          };
        };

      }
   );
}
