package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"
)

// CRMData holds all CRM dashboard query results.
type CRMData struct {
	// Stat cards
	ActiveDeals    int64
	PipelineValue  int64 // cents
	OpenTickets    int64
	PendingTasks   int64
	ContactCount   int64
	WonDealsValue  int64 // cents

	// Pipeline stages with values
	PipelineStages []PipelineStage

	// Recent deals
	RecentDeals []CRMDeal

	// Open tickets
	OpenTicketList []CRMTicket

	// Pending tasks
	PendingTaskList []CRMTask

	// Contacts
	Contacts []CRMContact
}

type PipelineStage struct {
	Stage string
	Count int64
	Value int64 // cents
}

type CRMDeal struct {
	ID           string
	Title        string
	Stage        string
	Value        int64
	Currency     string
	CustomerName string
	CreatedAt    time.Time
}

type CRMTicket struct {
	ID           string
	Subject      string
	Status       string
	Priority     string
	CustomerName string
	CreatedAt    time.Time
}

type CRMTask struct {
	ID        string
	Title     string
	Status    string
	CreatedAt time.Time
}

type CRMContact struct {
	ID           string
	FirstName    string
	LastName     string
	Email        string
	Phone        string
	Role         string
	CustomerName string
}

func newCRMStatsHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var d CRMData

		// Stat counts
		reader.Table("deals").Where("stage NOT IN ('won','lost')").Count(&d.ActiveDeals)
		reader.Table("tickets").Where("status IN ('open','in_progress')").Count(&d.OpenTickets)
		reader.Table("tasks").Where("status IN ('todo','in_progress')").Count(&d.PendingTasks)
		reader.Table("contacts").Count(&d.ContactCount)

		// Pipeline total value
		reader.Table("deals").
			Where("stage NOT IN ('won','lost')").
			Select("COALESCE(SUM(value),0)").
			Scan(&d.PipelineValue)

		// Won deals total
		reader.Table("deals").
			Where("stage = 'won'").
			Select("COALESCE(SUM(value),0)").
			Scan(&d.WonDealsValue)

		// Pipeline stages with values
		reader.Table("deals").
			Select("stage, COUNT(*) as count, COALESCE(SUM(value),0) as value").
			Group("stage").
			Order("CASE stage WHEN 'new' THEN 1 WHEN 'qualified' THEN 2 WHEN 'proposal' THEN 3 WHEN 'negotiation' THEN 4 WHEN 'won' THEN 5 WHEN 'lost' THEN 6 ELSE 7 END").
			Scan(&d.PipelineStages)

		// Recent deals
		reader.Table("deals d").
			Select("d.id, d.title, d.stage, d.value, d.currency, COALESCE(c.name,'') as customer_name, d.created_at").
			Joins("LEFT JOIN customers c ON d.customer_id = c.id").
			Order("d.created_at DESC").
			Limit(8).
			Scan(&d.RecentDeals)

		// Open tickets
		reader.Table("tickets t").
			Select("t.id, t.subject, t.status, t.priority, COALESCE(c.name,'') as customer_name, t.created_at").
			Joins("LEFT JOIN customers c ON t.customer_id = c.id").
			Where("t.status IN ('open','in_progress')").
			Order("CASE t.priority WHEN 'urgent' THEN 1 WHEN 'high' THEN 2 WHEN 'normal' THEN 3 ELSE 4 END, t.created_at DESC").
			Limit(8).
			Scan(&d.OpenTicketList)

		// Pending tasks
		reader.Table("tasks").
			Select("id, title, status, created_at").
			Where("status IN ('todo','in_progress')").
			Order("CASE status WHEN 'in_progress' THEN 1 ELSE 2 END, created_at DESC").
			Limit(8).
			Scan(&d.PendingTaskList)

		// Contacts
		reader.Table("contacts co").
			Select("co.id, co.first_name, co.last_name, co.email, co.phone, co.role, COALESCE(c.name,'') as customer_name").
			Joins("LEFT JOIN customers c ON co.customer_id = c.id").
			Order("co.created_at DESC").
			Limit(6).
			Scan(&d.Contacts)

		sse.PatchElementTempl(CRMContent(d))
	}
}

// FormatMoney formats cents as display string.
func FormatMoney(cents int64) string {
	if cents == 0 {
		return "0"
	}
	euros := float64(cents) / 100
	if euros >= 1_000_000 {
		return fmt.Sprintf("%.1fM", euros/1_000_000)
	}
	if euros >= 1_000 {
		return fmt.Sprintf("%.1fk", euros/1_000)
	}
	return fmt.Sprintf("%.0f", euros)
}

// Keep old handlers as stubs redirecting to unified endpoint.
// Router still registers them but they're unused if page uses single SSE init.
func newCRMPipelineHandler(reader *gorm.DB) http.HandlerFunc {
	return newCRMStatsHandler(reader) // unified
}

func newCRMTicketsHandler(reader *gorm.DB) http.HandlerFunc {
	return newCRMStatsHandler(reader) // unified
}

func newCRMTasksHandler(reader *gorm.DB) http.HandlerFunc {
	return newCRMStatsHandler(reader) // unified
}
