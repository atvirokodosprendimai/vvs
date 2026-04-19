# syntax=docker/dockerfile:1.7
# Multi-target Dockerfile — builds vvs-core, vvs-portal, vvs-stb from one source tree.
# Usage:
#   docker build --target core   -t vvs-core   .
#   docker build --target portal -t vvs-portal .
#   docker build --target stb    -t vvs-stb    .
# Or use docker compose which handles target selection automatically.

FROM golang:1.25-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/vvs-core   ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/vvs-portal ./cmd/portal && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/vvs-stb    ./cmd/stb

# ── core (admin UI + NATS + all business logic) ───────────────────────────────
FROM alpine:3.21 AS core
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S vvs \
    && adduser  -S -G vvs vvs \
    && mkdir -p /app/data /app/static \
    && chown -R vvs:vvs /app

COPY --from=builder /out/vvs-core /usr/local/bin/vvs-core
COPY static ./static

USER vvs

VOLUME ["/app/data"]
EXPOSE 8080 4222

ENTRYPOINT ["vvs-core", "serve"]

# ── portal (public customer portal — no DB) ───────────────────────────────────
FROM alpine:3.21 AS portal
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S vvs \
    && adduser  -S -G vvs vvs \
    && chown -R vvs:vvs /app

COPY --from=builder /out/vvs-portal /usr/local/bin/vvs-portal

USER vvs

EXPOSE 8081

ENTRYPOINT ["vvs-portal", "serve"]

# ── stb (IPTV set-top-box API — no DB) ────────────────────────────────────────
FROM alpine:3.21 AS stb
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S vvs \
    && adduser  -S -G vvs vvs \
    && chown -R vvs:vvs /app

COPY --from=builder /out/vvs-stb /usr/local/bin/vvs-stb

USER vvs

EXPOSE 8082

ENTRYPOINT ["vvs-stb", "serve"]
