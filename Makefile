.PHONY: build dev test lint clean install update

BINARY := notespub
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.Version=$(VERSION)

node_modules: package.json package-lock.json
	npm install
	@touch node_modules

build: node_modules  ## Compile CSS then build binary
	npx tailwindcss -i stylesheets/main.css -o style.css --minify
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/notespub

dev: node_modules  ## Watch mode: recompile on changes
	npx tailwindcss -i stylesheets/main.css -o style.css --watch &
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/notespub

clean:
	rm -f $(BINARY)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/notespub

update:
	git checkout main
	git pull --tags
	$(MAKE) install
	@echo "Installed: $$(notespub --version)"

test:
	go test ./...

lint:
	go tool golangci-lint run
