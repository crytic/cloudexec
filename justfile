
fmt:
	go fmt cmd/cloudexec/*.go
	go fmt pkg/digitalocean/*.go
	go fmt pkg/ssh/*.go
	go fmt pkg/state/*.go

trunk:
	trunk fmt
	trunk check

build:
  nix build

install:
	nix build
	echo nix profile remove $(shell nix profile list | grep cloudexec | cut -d " " -f 1)
	nix profile remove $(shell nix profile list | grep cloudexec | cut -d " " -f 1)
	echo nix profile install ./result
	nix profile install ./result
