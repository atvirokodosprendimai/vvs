package http

import (
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"
)

// ReportsData holds all data for the /reports page.
type ReportsData struct {
	MRR          []MonthRevenue    // paid invoices by created_at month, last 12 months
	PaymentTrend []MonthRevenue    // paid invoices by paid_at month, last 12 months
	InvoiceAging []AgingBucket     // outstanding finalized invoices by age bucket
	TopCustomers []CustomerRevenue // top customers by paid revenue
}

// AgingBucket represents one age band in the invoice aging report.
type AgingBucket struct {
	Label string
	Count int64
	Total int64 // cents
}

// CustomerRevenue is one row in the top-customers table.
type CustomerRevenue struct {
	CustomerID    string
	CustomerName  string
	Total         int64 // cents, paid invoices
	InvoiceCount  int64
}

// NewReportsHandler is exported for testing.
func NewReportsHandler(reader *gorm.DB) http.HandlerFunc { return newReportsHandler(reader) }

func newReportsHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var d ReportsData
		now := time.Now().UTC()
		twelveMonthsAgo := now.AddDate(0, -12, 0).Format("2006-01-02")

		// MRR — paid invoices by invoice created_at month (last 12 months)
		type rawMonth struct {
			Month string
			Total int64
		}
		var rawMRR []rawMonth
		reader.Table("invoices").
			Select("strftime('%Y-%m', created_at) as month, COALESCE(SUM(total_amount), 0) as total").
			Where("status = 'paid' AND created_at >= ?", twelveMonthsAgo).
			Group("strftime('%Y-%m', created_at)").
			Order("month ASC").
			Scan(&rawMRR)
		mrrFloats := make([]MonthRevenue, len(rawMRR))
		for i, m := range rawMRR {
			mrrFloats[i] = MonthRevenue{Month: m.Month, Total: float64(m.Total) / 100}
		}
		d.MRR = fillMonths(mrrFloats, 12)

		// Payment trend — paid invoices by paid_at month (last 12 months)
		var rawPay []rawMonth
		reader.Table("invoices").
			Select("strftime('%Y-%m', paid_at) as month, COALESCE(SUM(total_amount), 0) as total").
			Where("status = 'paid' AND paid_at IS NOT NULL AND paid_at >= ?", twelveMonthsAgo).
			Group("strftime('%Y-%m', paid_at)").
			Order("month ASC").
			Scan(&rawPay)
		payFloats := make([]MonthRevenue, len(rawPay))
		for i, m := range rawPay {
			payFloats[i] = MonthRevenue{Month: m.Month, Total: float64(m.Total) / 100}
		}
		d.PaymentTrend = fillMonths(payFloats, 12)

		// Invoice aging — outstanding finalized invoices bucketed by days past due
		type rawAging struct {
			Label string
			Count int64
			Total int64
		}
		var rawAging2 []rawAging
		reader.Raw(`
			SELECT
				CASE
					WHEN due_date >= date('now') THEN 'Current'
					WHEN CAST(julianday('now') - julianday(due_date) AS INTEGER) <= 30 THEN '1-30 days'
					WHEN CAST(julianday('now') - julianday(due_date) AS INTEGER) <= 60 THEN '31-60 days'
					WHEN CAST(julianday('now') - julianday(due_date) AS INTEGER) <= 90 THEN '61-90 days'
					ELSE '90+ days'
				END as label,
				COUNT(*) as count,
				COALESCE(SUM(total_amount), 0) as total
			FROM invoices
			WHERE status = 'finalized'
			GROUP BY label
			ORDER BY CASE label
				WHEN 'Current'    THEN 0
				WHEN '1-30 days'  THEN 1
				WHEN '31-60 days' THEN 2
				WHEN '61-90 days' THEN 3
				ELSE 4
			END
		`).Scan(&rawAging2)
		d.InvoiceAging = make([]AgingBucket, len(rawAging2))
		for i, a := range rawAging2 {
			d.InvoiceAging[i] = AgingBucket{Label: a.Label, Count: a.Count, Total: a.Total}
		}

		// Top customers by paid revenue (last 12 months)
		reader.Table("invoices").
			Select("customer_id, customer_name, SUM(total_amount) as total, COUNT(*) as invoice_count").
			Where("status = 'paid' AND created_at >= ?", twelveMonthsAgo).
			Group("customer_id, customer_name").
			Order("total DESC").
			Limit(10).
			Scan(&d.TopCustomers)

		sse.PatchElementTempl(ReportsContent(d))
	}
}
