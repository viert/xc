.PHONY: all deps build
export PATH:=deps:$(PATH)
GOVVV_PKG:=main

all: deps build

deps:
	go mod download
	go build -o deps/govvv github.com/ahmetb/govvv

build:
	@$(eval FLAGS := $$(shell PATH=$(PATH) govvv -flags -pkg $(GOVVV_PKG) ))
	go build \
		-o xc \
		-ldflags="$(FLAGS)" \
		cmd/xc/main.go
