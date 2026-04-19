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

	// Today — action items requiring attention now
	TodayOverdueInvoices []TodayInvoice
	TodayDueServices     []TodayService
	TodayUrgentTickets   []TodayTicket
	TodayOverdueTasks    []TodayTask

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

// TodayInvoice is a finalized invoice past its due date.
type TodayInvoice struct {
	ID            string
	InvoiceNumber string
	CustomerName  string
	Total         float64
	DueDate       time.Time
}

// TodayService is an active service whose next billing date has passed.
type TodayService struct {
	ID              string
	ProductName     string
	PriceAmount     int64
	Currency        string
	CustomerID      string
	NextBillingDate time.Time
}

// TodayTicket is an open/in-progress ticket sorted oldest-first.
type TodayTicket struct {
	ID        string
	Subject   string
	Status    string
	Priority  string
	CreatedAt time.Time
}

// TodayTask is a task with a due date that has passed and is not done.
type TodayTask struct {
	ID         string
	Title      string
	Priority   string
	DueDate    time.Time
	CustomerID string
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

		// Today — overdue invoices (finalized, due_date < now)
		now := time.Now().UTC()
		reader.Table("invoices").
			Select("id, invoice_number, customer_name, total, due_date").
			Where("status = 'finalized' AND due_date < ?", now).
			Order("due_date ASC").
			Limit(10).
			Scan(&d.TodayOverdueInvoices)

		// Today — services due for billing (active, next_billing_date <= now)
		reader.Table("customer_services").
			Select("id, product_name, price_amount, currency, customer_id, next_billing_date").
			Where("status = 'active' AND next_billing_date IS NOT NULL AND next_billing_date <= ?", now).
			Order("next_billing_date ASC").
			Limit(10).
			Scan(&d.TodayDueServices)

		// Today — oldest open/in-progress tickets (limit 5)
		reader.Table("tickets").
			Select("id, subject, status, priority, created_at").
			Where("status IN ('open', 'in_progress')").
			Order("created_at ASC").
			Limit(5).
			Scan(&d.TodayUrgentTickets)

		// Today — overdue tasks (due_date <= now, not done/cancelled)
		reader.Table("tasks").
			Select("id, title, priority, due_date, customer_id").
			Where("due_date IS NOT NULL AND due_date <= ? AND status IN ('todo', 'in_progress')", now).
			Order("due_date ASC").
			Limit(10).
			Scan(&d.TodayOverdueTasks)

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
		sixMonthsAgo := now.AddDate(0, -6, 0).Format("2006-01-02")
		type rawMonthCents struct {
			Month string
			Total int64
		}
		var rawRevenue []rawMonthCents
		reader.Table("invoices").
			Select("strftime('%Y-%m', created_at) as month, COALESCE(SUM(total_amount), 0) as total").
			Where("status = 'paid' AND created_at >= ?", sixMonthsAgo).
			Group("strftime('%Y-%m', created_at)").
			Order("month ASC").
			Scan(&rawRevenue)
		for _, m := range rawRevenue {
			d.MonthlyRevenue = append(d.MonthlyRevenue, MonthRevenue{Month: m.Month, Total: float64(m.Total) / 100})
		}

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
