.PHONY: all deps build
export PATH:=deps:$(PATH)
export CGO_ENABLED:=0
export GOOS:=linux
export GOARCH:=amd64
GOVVV_PKG:=main

all: deps build

deps:
	go mod download
	go build -o deps/govvv github.com/ahmetb/govvv

build:
	@$(eval FLAGS := $$(shell PATH=$(PATH) govvv -flags -pkg $(GOVVV_PKG) ))
	go build \
		-o bin/xc \
		-ldflags="$(FLAGS)" \
		cmd/xc/main.go
	cp aws/xcAwsInventory.py bin/
