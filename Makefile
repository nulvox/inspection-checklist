# Home Inspection Checklist — build & deploy
# Mirrors the workflow of fate4.tools: compile Go->WASM, assemble docs/ for
# GitHub Pages, serve locally.

WASM_OUT := web/inspect.wasm
DOCS     := docs

# Stamped into the service worker so each deploy busts the offline cache (adr/0004).
# Prefer the git short-sha; fall back to a timestamp outside a checkout.
BUILD_VERSION := $(shell git rev-parse --short HEAD 2>/dev/null || date -u +%Y%m%d%H%M%S)

# wasm_exec.js moved between Go versions: lib/wasm (>=1.24) vs misc/wasm (older).
WASM_EXEC := $(shell \
	if [ -f "$$(go env GOROOT)/lib/wasm/wasm_exec.js" ]; then \
		echo "$$(go env GOROOT)/lib/wasm/wasm_exec.js"; \
	else \
		echo "$$(go env GOROOT)/misc/wasm/wasm_exec.js"; \
	fi)

.PHONY: all build wasm assets docs serve test lint vet fmt clean

all: build

## build: compile WASM and assemble docs/ for deployment
build: wasm assets docs
	@echo "Build complete -> $(DOCS)/"

## wasm: compile the Go WebAssembly binary
wasm:
	GOOS=js GOARCH=wasm go build -o $(WASM_OUT) ./cmd/wasm

## assets: copy the runtime wasm_exec.js shim into web/
assets:
	cp "$(WASM_EXEC)" web/js/wasm_exec.js

## docs: assemble the deployable static site into docs/
docs:
	rm -rf $(DOCS)
	mkdir -p $(DOCS)
	cp -r web/. $(DOCS)/
	touch $(DOCS)/.nojekyll
	@# Stamp the SW cache version so a new deploy invalidates the offline cache.
	sed -i.bak 's/__BUILD_VERSION__/$(BUILD_VERSION)/' $(DOCS)/sw.js && rm -f $(DOCS)/sw.js.bak
	@echo "Stamped service worker version: $(BUILD_VERSION)"

## serve: build then serve locally at http://localhost:8080
serve: build
	@echo "Serving $(DOCS)/ at http://localhost:8080 (Ctrl-C to stop)"
	cd $(DOCS) && python3 -m http.server 8080

## test: run Go unit tests (host build of the pure packages)
test:
	go test ./internal/...

## vet / lint / fmt
vet:
	go vet ./internal/...
lint:
	golangci-lint run ./... || echo "(install golangci-lint to lint)"
fmt:
	gofmt -w .

## clean: remove build artifacts
clean:
	rm -f $(WASM_OUT) web/js/wasm_exec.js
	rm -rf $(DOCS)
