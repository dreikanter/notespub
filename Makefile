.PHONY: build dev test lint clean install update

BINARY := npub
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.Version=$(VERSION)

build:  ## Compile CSS then build binary
	npx tailwindcss -i stylesheets/main.css -o style.css --minify
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/npub

dev:    ## Watch mode: recompile on changes
	npx tailwindcss -i stylesheets/main.css -o style.css --watch &
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/npub

clean:
	rm -f $(BINARY)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/npub

update:
	git checkout main
	git pull --tags
	$(MAKE) install
	@echo "Installed: $$(npub --version)"

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run
