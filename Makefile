# Capture version, commit, and date
GIT_COMMIT=$(shell git rev-list -1 HEAD)
GIT_DATE=$(shell git log -1 --format=%cd --date=format:'%Y-%m-%d %H:%M:%S')
VERSION="$(shell cat VERSION | tr -d '\n\r')"

build: 
	# Build the Go app with ldflags
	@mkdir -p dist
	cd cmd/cloudexec && go build -ldflags "-X 'main.version=$(VERSION)' -X 'main.commit=$(GIT_COMMIT)' -X 'main.date=$(GIT_DATE)'" -o ../../dist/cloudexec

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
