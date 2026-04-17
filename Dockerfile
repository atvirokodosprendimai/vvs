# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/vvs ./cmd/server

FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S vvs \
    && adduser -S -G vvs vvs \
    && mkdir -p /app/data /app/static \
    && chown -R vvs:vvs /app

COPY --from=builder /out/vvs /usr/local/bin/vvs
COPY static ./static

USER vvs

ENV VVS_DB_PATH=/app/data/vvs.db
ENV VVS_ADDR=:8080

VOLUME ["/app/data"]
EXPOSE 8080

ENTRYPOINT ["vvs", "serve"]
