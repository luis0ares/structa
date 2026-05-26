# Structa — build helpers
#
# Tested with GNU Make on Windows (Git Bash / MSYS / mingw32-make).
# All commands assume the Wails CLI is on PATH. If not, run `make wails-cli`.

WAILS ?= wails
BIN   := build/bin/structa.exe

.PHONY: help dev build build-clean build-installer run tidy generate tsc lint deps wails-cli clean

help:
	@echo "Targets:"
	@echo "  dev             - wails dev (hot reload window)"
	@echo "  build           - production build -> $(BIN)"
	@echo "  build-clean     - production build with a clean bin/ first"
	@echo "  build-installer - build + NSIS installer"
	@echo "  run             - run the last build"
	@echo "  generate        - regenerate TS bindings (frontend/wailsjs/)"
	@echo "  tsc             - type-check the frontend"
	@echo "  tidy            - go mod tidy"
	@echo "  deps            - install Go modules + frontend npm packages"
	@echo "  wails-cli       - install the Wails CLI via go install"
	@echo "  clean           - remove build/bin and frontend/dist"

dev:
	$(WAILS) dev

build:
	$(WAILS) build

build-clean:
	$(WAILS) build -clean

build-installer:
	$(WAILS) build -nsis

run: $(BIN)
	"$(BIN)"

generate:
	$(WAILS) generate module

tsc:
	cd frontend && npx tsc --noEmit

tidy:
	go mod tidy

deps:
	go mod download
	cd frontend && npm install

wails-cli:
	go install github.com/wailsapp/wails/v2/cmd/wails@latest

clean:
	rm -rf build/bin
	rm -rf frontend/dist
