
fmt:
	go fmt cmd/cloudexec/*.go
	go fmt pkg/digitalocean/*.go
	go fmt pkg/ssh/*.go
	go fmt pkg/state/*.go

trunk:
	trunk fmt
	trunk check

pack opSecretReference="op://Private/DigitalOcean/ApiKey":
  cd packer && packer build -var do_api_token=$(op read {{opSecretReference}}) cloudexec.pkr.hcl

build:
  nix build

install: build
	echo nix profile remove $(nix profile list | grep cloudexec | cut -d " " -f 1)
	nix profile remove $(nix profile list | grep cloudexec | cut -d " " -f 1)
	echo nix profile install ./result
	nix profile install ./result

launch-example:
  cd example && cloudexec launch
