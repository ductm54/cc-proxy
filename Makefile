.PHONY: all build web clean dev test

BIN := cc-proxy

all: build

web:
	cd web && bun install && bun run build

build: web
	go build -o $(BIN) ./cmd/cc-proxy

dev:
	cd web && bun run dev &
	go run ./cmd/cc-proxy serve --log-dev

test: web
	go test ./...

clean:
	rm -f $(BIN)
	rm -rf web/dist
