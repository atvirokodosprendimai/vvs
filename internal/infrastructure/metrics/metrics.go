// Package metrics defines all Prometheus metrics for the vvs-core binary.
// All metric names use the vvs_ prefix for easy filtering in Grafana.
// Call Register() once at startup to register all metrics with the default registry.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// HTTP metrics
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vvs_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds, by method, route pattern, and status code.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vvs_http_requests_total",
			Help: "Total number of HTTP requests, by method, route pattern, and status code.",
		},
		[]string{"method", "path", "status"},
	)
	ActiveSSEConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vvs_sse_active_connections",
		Help: "Number of currently active Server-Sent Events connections.",
	})

	// NATS metrics
	NATSPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vvs_nats_published_total",
			Help: "Total NATS messages published, by subject.",
		},
		[]string{"subject"},
	)
	NATSReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vvs_nats_received_total",
			Help: "Total NATS messages received by subscribers, by subject.",
		},
		[]string{"subject"},
	)

	// Cron metrics
	CronLastRunTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vvs_cron_last_run_time_seconds",
			Help: "Unix timestamp of the last cron action run, by action name.",
		},
		[]string{"action"},
	)
	CronDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vvs_cron_duration_seconds",
			Help:    "Duration of cron action runs in seconds, by action name.",
			Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"action"},
	)

	// Business metrics
	InvoiceDelivered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vvs_invoice_delivered_total",
		Help: "Total invoices successfully delivered by email.",
	})
	InvoiceDeliveryErr = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vvs_invoice_delivery_errors_total",
		Help: "Total invoice email delivery failures.",
	})
	EmailSyncMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vvs_email_sync_messages_total",
		Help: "Total email messages synced from IMAP.",
	})
	EmailSyncErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vvs_email_sync_errors_total",
		Help: "Total errors during email IMAP sync.",
	})
)

// Register registers all vvs metrics with the default Prometheus registry.
// Must be called once at startup before any metrics are recorded.
func Register() {
	prometheus.MustRegister(
		HTTPRequestDuration,
		HTTPRequestsTotal,
		ActiveSSEConns,
		NATSPublished,
		NATSReceived,
		CronLastRunTime,
		CronDuration,
		InvoiceDelivered,
		InvoiceDeliveryErr,
		EmailSyncMessages,
		EmailSyncErrors,
	)
}
