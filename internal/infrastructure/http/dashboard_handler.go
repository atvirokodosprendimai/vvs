package http

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"
)

// DashboardData holds all dashboard query results.
type DashboardData struct {
	// Stat cards
	CustomerCount    int64
	ProductCount     int64
	OpenTickets      int64
	ActiveDeals      int64
	UnpaidInvoices   int64
	ActiveTasks      int64

	// Ticket status breakdown
	TicketStatuses []StatusCount

	// Deal pipeline breakdown
	DealStages []StatusCount

	// Customer status breakdown
	CustomerStatuses []StatusCount

	// Recent tickets
	RecentTickets []RecentTicket

	// Recent invoices
	RecentInvoices []RecentInvoice

	// Monthly revenue (last 6 months)
	MonthlyRevenue []MonthRevenue
}

type StatusCount struct {
	Status string
	Count  int64
}

type RecentTicket struct {
	ID        string
	Subject   string
	Status    string
	Priority  string
	CreatedAt time.Time
}

type RecentInvoice struct {
	ID            string
	InvoiceNumber string
	CustomerName  string
	Status        string
	Total         float64
	CreatedAt     time.Time
}

type MonthRevenue struct {
	Month string // "Jan", "Feb", etc.
	Total float64
}

// DonutSegment represents one slice of a donut chart.
type DonutSegment struct {
	Label   string
	Count   int64
	Color   string
	Offset  float64 // dashoffset for SVG
	Length  float64 // dash length for SVG
}

func newDashboardStatsHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var d DashboardData

		// Stat card counts
		reader.Table("customers").Count(&d.CustomerCount)
		reader.Table("products").Count(&d.ProductCount)
		reader.Table("tickets").Where("status IN ('open','in_progress')").Count(&d.OpenTickets)
		reader.Table("deals").Where("stage NOT IN ('won','lost')").Count(&d.ActiveDeals)
		reader.Table("invoices").Where("status NOT IN ('paid','void')").Count(&d.UnpaidInvoices)
		reader.Table("tasks").Where("status IN ('todo','in_progress')").Count(&d.ActiveTasks)

		// Ticket status distribution
		reader.Table("tickets").
			Select("status, count(*) as count").
			Group("status").
			Scan(&d.TicketStatuses)

		// Deal pipeline
		reader.Table("deals").
			Select("stage as status, count(*) as count").
			Group("stage").
			Scan(&d.DealStages)

		// Customer status
		reader.Table("customers").
			Select("status, count(*) as count").
			Group("status").
			Scan(&d.CustomerStatuses)

		// Recent tickets
		reader.Table("tickets").
			Select("id, subject, status, priority, created_at").
			Order("created_at DESC").
			Limit(5).
			Scan(&d.RecentTickets)

		// Recent invoices
		reader.Table("invoices").
			Select("id, invoice_number, customer_name, status, total, created_at").
			Order("created_at DESC").
			Limit(5).
			Scan(&d.RecentInvoices)

		// Monthly revenue — last 6 months
		sixMonthsAgo := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
		reader.Table("invoices").
			Select("strftime('%Y-%m', created_at) as month, COALESCE(SUM(total), 0) as total").
			Where("status = 'paid' AND created_at >= ?", sixMonthsAgo).
			Group("strftime('%Y-%m', created_at)").
			Order("month ASC").
			Scan(&d.MonthlyRevenue)

		// Fill missing months
		d.MonthlyRevenue = fillMonths(d.MonthlyRevenue, 6)

		sse.PatchElementTempl(DashboardContent(d))
	}
}

// fillMonths ensures we have entries for the last N months.
func fillMonths(data []MonthRevenue, n int) []MonthRevenue {
	now := time.Now()
	result := make([]MonthRevenue, n)
	existing := make(map[string]float64)
	for _, m := range data {
		existing[m.Month] = m.Total
	}
	for i := 0; i < n; i++ {
		t := now.AddDate(0, -(n-1-i), 0)
		key := t.Format("2006-01")
		label := t.Format("Jan")
		result[i] = MonthRevenue{Month: label, Total: existing[key]}
	}
	return result
}

// BuildDonut converts StatusCount slices into SVG-ready segments.
func BuildDonut(items []StatusCount, colorMap map[string]string, defaultColor string) []DonutSegment {
	var total int64
	for _, item := range items {
		total += item.Count
	}
	if total == 0 {
		return nil
	}

	circumference := 2 * math.Pi * 40 // radius=40
	segments := make([]DonutSegment, len(items))
	offset := 0.0

	for i, item := range items {
		pct := float64(item.Count) / float64(total)
		length := pct * circumference

		color := defaultColor
		if c, ok := colorMap[item.Status]; ok {
			color = c
		}

		segments[i] = DonutSegment{
			Label:  item.Status,
			Count:  item.Count,
			Color:  color,
			Offset: -offset + circumference/4, // rotate -90deg so first segment starts at top
			Length: length,
		}
		offset += length
	}
	return segments
}

// BarMax returns the max total for scaling bars.
func BarMax(items []MonthRevenue) float64 {
	max := 0.0
	for _, m := range items {
		if m.Total > max {
			max = m.Total
		}
	}
	if max == 0 {
		return 1 // avoid div-by-zero
	}
	return max
}

// FormatCurrency formats a float as a currency string.
func FormatCurrency(v float64) string {
	if v == 0 {
		return "0"
	}
	if v >= 1000 {
		return fmt.Sprintf("%.1fk", v/1000)
	}
	return fmt.Sprintf("%.0f", v)
}
