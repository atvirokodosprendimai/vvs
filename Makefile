.PHONY: build run test dev generate clean \
        build-all build-core build-portal build-stb \
        run-all run-core run-portal run-stb

# ── local dev env ─────────────────────────────────────────────────────────────
# Override any of these via environment before calling make.
DEV_DB       ?= ./data/dev.db
DEV_ADDR     ?= :8080
DEV_PORTAL   ?= :8081
DEV_STB      ?= :8082
DEV_NATS     ?= 127.0.0.1:4222

# ── single binary (legacy targets) ────────────────────────────────────────────

# Build single binary
build: generate
	go build -o bin/vvs ./cmd/server

# Build binary then run it
run: generate
	@mkdir -p bin
	go build -o bin/vvs ./cmd/server
	./bin/vvs serve

# ── multi-binary build ────────────────────────────────────────────────────────

# Build all binaries: vvs-core, vvs-portal, vvs-stb
build-all: generate
	@mkdir -p bin
	go build -o bin/vvs-core   ./cmd/server
	go build -o bin/vvs-portal ./cmd/portal
	go build -o bin/vvs-stb    ./cmd/stb

build-core: generate
	@mkdir -p bin
	go build -o bin/vvs-core ./cmd/server

build-portal: generate
	@mkdir -p bin
	go build -o bin/vvs-portal ./cmd/portal

build-stb: generate
	@mkdir -p bin
	go build -o bin/vvs-stb ./cmd/stb

# ── run individual ────────────────────────────────────────────────────────────

run-core: build-core
	@mkdir -p data
	VVS_DB_PATH=$(DEV_DB) \
	VVS_ADDR=$(DEV_ADDR) \
	NATS_LISTEN_ADDR=$(DEV_NATS) \
	./bin/vvs-core serve

run-portal: build-portal
	NATS_URL=nats://$(DEV_NATS) \
	PORTAL_ADDR=$(DEV_PORTAL) \
	PORTAL_INSECURE_COOKIE=true \
	./bin/vvs-portal serve

run-stb: build-stb
	NATS_URL=nats://$(DEV_NATS) \
	STB_ADDR=$(DEV_STB) \
	VVS_BASE_URL=http://localhost$(DEV_STB) \
	./bin/vvs-stb serve

# ── run-all: both services, one command ───────────────────────────────────────
# Builds both binaries, starts them in background, waits.
# Ctrl-C kills both cleanly via trap.

run-all: build-all
	@mkdir -p data
	@echo "Starting vvs-core on $(DEV_ADDR), vvs-portal on $(DEV_PORTAL), vvs-stb on $(DEV_STB)"
	@trap 'kill %1 %2 %3 2>/dev/null; echo "stopped"' INT TERM; \
	VVS_DB_PATH=$(DEV_DB) \
	VVS_ADDR=$(DEV_ADDR) \
	NATS_LISTEN_ADDR=$(DEV_NATS) \
	./bin/vvs-core serve & \
	sleep 1 && \
	NATS_URL=nats://$(DEV_NATS) \
	PORTAL_ADDR=$(DEV_PORTAL) \
	PORTAL_INSECURE_COOKIE=true \
	./bin/vvs-portal serve & \
	NATS_URL=nats://$(DEV_NATS) \
	STB_ADDR=$(DEV_STB) \
	VVS_BASE_URL=http://localhost$(DEV_STB) \
	./bin/vvs-stb serve & \
	wait

# ── tests ─────────────────────────────────────────────────────────────────────

# Run all tests
test:
	go test ./... -v -race -count=1

# Unit tests only (fast)
test-unit:
	go test ./internal/modules/*/domain/... ./internal/shared/... -v

# Integration tests
test-integration:
	go test ./internal/modules/*/adapters/... -v

# ── codegen + tooling ─────────────────────────────────────────────────────────

# Generate templ files
generate:
	templ generate ./internal/...

# Clean build artifacts
clean:
	rm -rf bin/ data/

# Run with live reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air -c .air.toml
