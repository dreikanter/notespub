.PHONY: build dev test lint

build:  ## Compile CSS then build binary
	npx tailwindcss -i stylesheets/main.css -o style.css --minify
	go build ./cmd/notespub

dev:    ## Watch mode: recompile on changes
	npx tailwindcss -i stylesheets/main.css -o style.css --watch &
	go build ./cmd/notespub

test:
	go test ./...

lint:
	go tool golangci-lint run
