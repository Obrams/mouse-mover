COVER_PROFILE=cover.out
COVER_HTML=cover.html
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

.PHONY: $(COVER_PROFILE) $(COVER_HTML)

all: open

build: clean
	mkdir -p -v ./bin/mm.app/Contents/Resources
	mkdir -p -v ./bin/mm.app/Contents/MacOS
	cp ./appInfo/*.plist ./bin/mm.app/Contents/Info.plist
	cp ./appInfo/*.icns ./bin/mm.app/Contents/Resources/icon.icns
	go build -ldflags "-X main.version=$(VERSION)" -o ./bin/mm.app/Contents/MacOS/mm cmd/main.go

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