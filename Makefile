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
