.PHONY: build run test dev generate clean

# Build single binary
build: generate
	go build -o bin/vvs ./cmd/server

# Build binary then run it
run: generate
	@mkdir -p bin
	go build -o bin/vvs ./cmd/server
	./bin/vvs 

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
