---
tldr: Prometheus metrics endpoint + HTTP/NATS/cron/business instrumentation — no more blind production incidents
status: completed
---

# Plan: Structured Observability — Prometheus Metrics

## Context

Consilium 2026-04-19 priority #17. System has `slog` request logging but zero metrics — no request latency histograms, no NATS queue depth, no cron execution visibility. Production incidents diagnosed blind.

**Existing:**
- chi router with `requestLogger` middleware (`internal/infrastructure/http/router.go:17`)
- `slog` structured logging throughout
- No `prometheus/client_golang` in `go.mod`

**Scope decision:** Prometheus metrics only (no OTel traces — single binary on one VPS, traces add infra complexity with minimal gain). Grafana/Prometheus scrape `/metrics` every 15s.

**Security:** `/metrics` served on a separate internal port (`:9091`) — not exposed through the public chi router. Operator configures firewall to allow only Prometheus scraper.

- Consilium backlog: [[project_consilium_backlog_priority]] — #17

## Phases

### Phase 1 — Dependency + metrics registry — status: open

1. [ ] Add `github.com/prometheus/client_golang` to `go.mod`
   - `go get github.com/prometheus/client_golang/prometheus`
   - `go get github.com/prometheus/client_golang/prometheus/promhttp`

2. [ ] Create `internal/infrastructure/metrics/metrics.go`
   - singleton registry (or use `prometheus.DefaultRegisterer`)
   - define all metrics upfront (consistent naming: `vvs_*`):
     ```go
     var (
         HTTPRequestDuration = prometheus.NewHistogramVec(
             prometheus.HistogramOpts{Name: "vvs_http_request_duration_seconds", ...},
             []string{"method", "path", "status"},
         )
         HTTPRequestsTotal = prometheus.NewCounterVec(...)
         ActiveSSEConns     = prometheus.NewGauge(prometheus.GaugeOpts{Name: "vvs_sse_active_connections"})
         NATSPublished      = prometheus.NewCounterVec(..., []string{"subject"})
         NATSReceived       = prometheus.NewCounterVec(..., []string{"subject"})
         CronLastRunTime    = prometheus.NewGaugeVec(..., []string{"action"})
         CronDuration       = prometheus.NewHistogramVec(..., []string{"action"})
         InvoiceDelivered   = prometheus.NewCounter(...)
         InvoiceDeliveryErr = prometheus.NewCounter(...)
         EmailSyncMessages  = prometheus.NewCounter(...)
         EmailSyncErrors    = prometheus.NewCounter(...)
     )
     ```
   - `Register()` func that registers all metrics with `prometheus.DefaultRegisterer`

3. [ ] Expose `/metrics` on internal port in `cmd/vvs-core/main.go`
   - start a second `net/http` listener on `cfg.MetricsAddr` (default `:9091`, env `VVS_METRICS_ADDR`)
   - `http.Handle("/metrics", promhttp.Handler())`
   - if `cfg.MetricsAddr == ""` skip silently (opt-in)

### Phase 2 — HTTP middleware instrumentation — status: open

1. [ ] Add `metricsMiddleware` to chi router in `router.go`
   - wraps `http.ResponseWriter` to capture status code
   - records `HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(elapsed)`
   - records `HTTPRequestsTotal.WithLabelValues(...)` counter
   - path label: use chi route pattern (`chi.RouteContext(r.Context()).RoutePattern()`) — not raw URL (avoid cardinality explosion on `/customers/abc-123/...`)

2. [ ] Track active SSE connections
   - `global_sse.go`, `notifications.go`: `ActiveSSEConns.Inc()` on connect, `.Dec()` on disconnect
   - same for chat SSE

3. [ ] `go build ./... && go test ./internal/infrastructure/...`

### Phase 3 — NATS + cron + business metrics — status: open

1. [ ] Instrument NATS publisher
   - `internal/infrastructure/nats/publisher.go`: `NATSPublished.WithLabelValues(subject).Inc()` after publish

2. [ ] Instrument NATS subscriber
   - `internal/infrastructure/nats/subscriber.go`: `NATSReceived.WithLabelValues(subject).Inc()` on message handler entry

3. [ ] Instrument cron actions
   - `cmd/server/cron_actions.go` (generate-due-invoices) and `cmd/server/dunning_actions.go`
   - wrap action func: record start time → run → record `CronDuration` + `CronLastRunTime`

4. [ ] Instrument invoice delivery worker
   - `internal/modules/invoice/worker/`: `InvoiceDelivered.Inc()` on success, `InvoiceDeliveryErr.Inc()` on error

5. [ ] Instrument email sync worker
   - `internal/modules/email/worker/`: `EmailSyncMessages.Add(float64(fetched))` per sync cycle, `EmailSyncErrors.Inc()` on error

### Phase 4 — Deploy template + documentation — status: open

1. [ ] Add `VVS_METRICS_ADDR` to `deploy/` env templates
   - default `:9091`

2. [ ] Add Prometheus scrape config snippet to `deploy/` (or README)
   ```yaml
   scrape_configs:
     - job_name: vvs
       static_configs:
         - targets: ['localhost:9091']
   ```

## Verification

```bash
go build ./cmd/vvs-core/
VVS_METRICS_ADDR=:9091 ./vvs-core &
curl http://localhost:9091/metrics | grep vvs_
# Make a few HTTP requests → vvs_http_request_duration_seconds should appear
# Check vvs_sse_active_connections after opening dashboard
# Run cron action → vvs_cron_last_run_time_seconds should update
```

## Adjustments

## Progress Log
