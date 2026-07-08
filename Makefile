COVER_PROFILE=cover.out
COVER_HTML=cover.html
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

# Code signing: sign with a stable local identity so macOS keeps the Accessibility
# permission across rebuilds. Create it once with `make codesign-setup`.
SIGN_IDENTITY ?= MM Local Codesign
BUNDLE_ID ?= com.obrams.mm
APP ?= ./bin/mm.app

.PHONY: $(COVER_PROFILE) $(COVER_HTML)

all: open

build: clean
	mkdir -p -v ./bin/mm.app/Contents/Resources
	mkdir -p -v ./bin/mm.app/Contents/MacOS
	cp ./appInfo/*.plist ./bin/mm.app/Contents/Info.plist
	cp ./appInfo/*.icns ./bin/mm.app/Contents/Resources/icon.icns
	go build -ldflags "-X main.version=$(VERSION)" -o ./bin/mm.app/Contents/MacOS/mm cmd/main.go
	@if security find-identity -p codesigning | grep -q "$(SIGN_IDENTITY)"; then \
		echo "==> signing $(APP) with '$(SIGN_IDENTITY)'"; \
		codesign --force --sign "$(SIGN_IDENTITY)" --identifier $(BUNDLE_ID) $(APP); \
	else \
		echo "==> '$(SIGN_IDENTITY)' not found; app left ad-hoc. Run 'make codesign-setup' for persistent Accessibility."; \
	fi

# Create the local self-signed code-signing identity (idempotent).
codesign-setup:
	./scripts/setup-codesign.sh

# Build, then install the (signed) bundle into /Applications for everyday use.
install: build
	rm -rf /Applications/mm.app
	cp -R ./bin/mm.app /Applications/
	@echo "==> installed /Applications/mm.app"

open: build
	open ./bin

clean:
	rm -rf ./bin

start:
	go run cmd/main.go

test:coverage

coverage: $(COVER_HTML)

$(COVER_HTML): $(COVER_PROFILE)
	go tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)

$(COVER_PROFILE):
	go test -v -failfast -race -coverprofile=$(COVER_PROFILE) ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

.PHONY: build 
.PHONY: clean