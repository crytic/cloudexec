build: 
	@mkdir -p dist
	@cd cmd/cloudexec && go build -o ../../dist/cloudexec

format:
	trunk fmt

check:
	trunk check

trunk: format check

lint:
	go fmt cmd/cloudexec/*.go
	go fmt pkg/digitalocean/*.go
	go fmt pkg/ssh/*.go
	go fmt pkg/state/*.go
	shellcheck cmd/cloudexec/user_data.sh.tmpl

existing_installation=$(shell nix profile list | grep cloudexec | cut -d " " -f 1)
nix-install:
	nix build
	nix profile remove $(existing_installation)
	nix profile install ./result
