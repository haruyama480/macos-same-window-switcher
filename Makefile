GO      ?= go
BIN     := bin/same-window-switcher
PREFIX  ?= $(HOME)/.local
APPDIR  ?= $(HOME)/Applications
APP     := $(APPDIR)/SameWindowSwitcher.app
IDENT   := com.github.haruyama480.same-window-switcher
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test test-darwin install install-app doctor clean

build:
	CGO_ENABLED=1 GOOS=darwin $(GO) build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		-o $(BIN) ./cmd/same-window-switcher
	codesign -s - --identifier $(IDENT) --force $(BIN)

test:
	CGO_ENABLED=0 $(GO) test ./internal/types ./internal/policy ./internal/cycle ./internal/config ./internal/cli ./internal/filter ./internal/ax ./internal/app

test-darwin:
	CGO_ENABLED=1 $(GO) test ./internal/ax ./internal/filter

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BIN) $(PREFIX)/bin/same-window-switcher

install-app: build
	install -d $(APP)/Contents/MacOS
	install -m 755 $(BIN) $(APP)/Contents/MacOS/same-window-switcher
	install -m 644 packaging/Info.plist $(APP)/Contents/Info.plist
	codesign -s - --identifier $(IDENT) --force $(APP)

doctor: install
	$(PREFIX)/bin/same-window-switcher doctor

clean:
	rm -rf bin
