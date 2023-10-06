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
            vendorSha256 = "sha256-xiiMcjo+hRllttjYXB3F2Ms2gX43r7/qgwxr4THNhsk=";
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
