SHELL=bash

# Capture version, commit, and date
GIT_COMMIT=$(shell git rev-list -1 HEAD)
GIT_DATE=$(shell git log -1 --format=%cd --date=format:'%Y-%m-%d-%H:%M:%S')
VERSION="$(shell cat VERSION | tr -d '\n\r')"

# Build the Go app with ldflags
build: 
	@mkdir -p dist
	cd cmd/cloudexec && go build -ldflags "-X 'main.Version=$(VERSION)' -X 'main.Commit=$(GIT_COMMIT)' -X 'main.Date=$(GIT_DATE)'" -o ../../dist/cloudexec

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

# smuggles git info in the VERSION file to work around nix flake hermicity
nix-install:
	@cp -f ./VERSION ./.VERSION.backup
	@trap 'mv -f ./.VERSION.backup ./VERSION' EXIT; \
	SUCCESS=0; \
	echo "commit=$(GIT_COMMIT)" >> ./VERSION; \
	echo "date=$(GIT_DATE)" >> ./VERSION; \
	echo "nix build"; \
	nix build && SUCCESS=1; \
	if [ $$SUCCESS -eq 1 ]; then \
		echo nix profile remove $(shell nix profile list | grep cloudexec | cut -d " " -f 1); \
		nix profile remove $(shell nix profile list | grep cloudexec | cut -d " " -f 1); \
		echo nix profile install ./result; \
		nix profile install ./result; \
	fi
