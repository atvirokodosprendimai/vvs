.PHONY: build run test dev generate clean

# Build single binary
build: generate
	go build -o bin/vvs ./cmd/server

# Run in development
run: generate
	go run ./cmd/server --db ./data/vvs.db --addr :8080

# Run all tests
test:
	go test ./... -v -race -count=1

# Unit tests only (fast)
test-unit:
	go test ./internal/modules/*/domain/... ./internal/shared/... -v

# Integration tests
test-integration:
	go test ./internal/modules/*/adapters/... -v

# Generate templ files
generate:
	templ generate ./internal/...

# Clean build artifacts
clean:
	rm -rf bin/ data/

# Run with live reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air -c .air.toml
