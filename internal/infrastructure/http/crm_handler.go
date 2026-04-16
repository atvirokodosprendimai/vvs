package http

import (
	"fmt"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"
)

func newCRMStatsHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var dealCount, ticketCount, taskCount, contactCount int64
		reader.Table("deals").Where("stage NOT IN ('won','lost')").Count(&dealCount)
		reader.Table("tickets").Where("status IN ('open','in_progress')").Count(&ticketCount)
		reader.Table("tasks").Where("status IN ('todo','in_progress')").Count(&taskCount)
		reader.Table("contacts").Count(&contactCount)

		sse.PatchElementTempl(CRMStats(
			fmt.Sprintf("%d", dealCount),
			fmt.Sprintf("%d", ticketCount),
			fmt.Sprintf("%d", taskCount),
			fmt.Sprintf("%d", contactCount),
		))
	}
}

func newCRMPipelineHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var rows []struct {
			Stage string
			Count int64
		}
		reader.Raw(
			"SELECT stage, COUNT(*) as count FROM deals WHERE stage NOT IN ('won','lost') GROUP BY stage ORDER BY CASE stage WHEN 'new' THEN 1 WHEN 'qualified' THEN 2 WHEN 'proposal' THEN 3 WHEN 'negotiation' THEN 4 ELSE 5 END",
		).Scan(&rows)

		stages := make([]CRMStageCount, len(rows))
		for i, r := range rows {
			stages[i] = CRMStageCount{Stage: r.Stage, Count: fmt.Sprintf("%d", r.Count)}
		}
		sse.PatchElementTempl(CRMPipeline(stages))
	}
}

func newCRMTicketsHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var rows []struct {
			CustomerID string `gorm:"column:customer_id"`
			Subject    string `gorm:"column:subject"`
			Priority   string `gorm:"column:priority"`
		}
		reader.Raw(
			"SELECT customer_id, subject, priority FROM tickets WHERE status IN ('open','in_progress') ORDER BY CASE priority WHEN 'urgent' THEN 1 WHEN 'high' THEN 2 WHEN 'normal' THEN 3 ELSE 4 END LIMIT 10",
		).Scan(&rows)

		tickets := make([]CRMTicketRow, len(rows))
		for i, r := range rows {
			tickets[i] = CRMTicketRow{CustomerID: r.CustomerID, Subject: r.Subject, Priority: r.Priority}
		}
		sse.PatchElementTempl(CRMOpenTickets(tickets))
	}
}

func newCRMTasksHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var rows []struct {
			Title  string `gorm:"column:title"`
			Status string `gorm:"column:status"`
		}
		reader.Raw(
			"SELECT title, status FROM tasks WHERE status IN ('todo','in_progress') ORDER BY CASE status WHEN 'in_progress' THEN 1 ELSE 2 END, created_at DESC LIMIT 10",
		).Scan(&rows)

		tasks := make([]CRMTaskRow, len(rows))
		for i, r := range rows {
			tasks[i] = CRMTaskRow{Title: r.Title, Status: r.Status}
		}
		sse.PatchElementTempl(CRMPendingTasks(tasks))
	}
}
